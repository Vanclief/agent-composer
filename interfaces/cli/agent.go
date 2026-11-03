package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cli "github.com/urfave/cli/v2"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/messaging/httpapi"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	"github.com/vanclief/agent-composer/runtime/harnesses"
	runtimetypes "github.com/vanclief/agent-composer/runtime/types"
)

const (
	// agentWaitTimeout is how long one wait for the next message blocks.
	agentWaitTimeout = time.Minute
	// agentRetryDelay is the pause after a failed wait before trying again.
	agentRetryDelay = 5 * time.Second
	// homeRetryMin and homeRetryMax bound the home loop's back-off after a
	// failed wait: it doubles from min to max and resets on a good wait.
	homeRetryMin = 2 * time.Second
	homeRetryMax = 30 * time.Second
	// deliverPrefix starts an answer that delivers the task (ADR 0010
	// item 2); the rest of the answer is the summary.
	deliverPrefix = "DONE:"
	// questionPrefix starts an answer that is a question for the manager
	// or the owner: the rest is posted as a question, which shows as
	// waiting on them until someone answers it.
	questionPrefix = "QUESTION:"
)

// agentHarness runs one turn: prompt on session, or a new session when
// session is empty. It returns the final answer and the session id.
type agentHarness interface {
	Run(ctx context.Context, prompt string, session string) (string, string, error)
}

// agentMessenger is the part of the messaging client the loop uses.
type agentMessenger interface {
	Join(ctx context.Context, name string) (*msgmodels.Participant, error)
	Leave(ctx context.Context, participantID string) (*httpapi.LeaveResponse, error)
	Post(ctx context.Context, participantID, channel, body string, replyTo *int64) (*msgmodels.Message, error)
	Ask(ctx context.Context, participantID, channel, body string) (*msgmodels.Message, error)
	Wait(ctx context.Context, channel string, after int64, timeout time.Duration) (*httpapi.MessagesResponse, error)
	Channels(ctx context.Context, archived bool) (*httpapi.ChannelsResponse, error)
	// Deliver is agc task deliver on the sandbox's socket.
	Deliver(ctx context.Context, summary string, report json.RawMessage, files map[string][]byte, out any) error
	// Image downloads a message's image (ADR 0005 "Images").
	Image(ctx context.Context, messageID int64, name string) ([]byte, error)
}

// agentLoop is `agc agent run` (ADR 0009 item 2, ADR 0010): it joins
// #<task>, runs the harness on the brief, posts the answer, then answers
// every message from someone else on the same session. An answer starting
// with QUESTION: is posted as a question; one starting with DONE: delivers
// the task instead of being posted, and the loop keeps waiting for review: a rejection is its next turn, an acceptance ends it
// (the sandbox is being stopped).
type agentLoop struct {
	harness agentHarness
	msg     agentMessenger
	task    string
	name    string
	brief   string
	// closers may accept or reject the task: its manager and owner. Their
	// /accept and /reject are not turns; agc's answer to them is.
	closers map[string]bool
	log     io.Writer
	// joined is called with the participant once joined.
	joined func(p *msgmodels.Participant) error
	// images is where a turn's images are put, empty for defaultImagesDir.
	images string
}

func agentCommand() *cli.Command {
	return &cli.Command{
		Name:  "agent",
		Usage: "Run a task's agent inside its sandbox (ADR 0009), or a persistent agent inside its home (ADR 0016)",
		Subcommands: []*cli.Command{
			{
				Name:  "run",
				Usage: "Join #<task>, run the harness on the brief, then answer every message in the channel until the task is accepted; with --agent, answer the agent's channels one thread at a time until it is idle",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "harness", Usage: "claude_code or codex_cli", Required: true},
					&cli.StringFlag{Name: "model", Usage: "Model the harness runs"},
					&cli.StringFlag{Name: "effort", Usage: "Reasoning effort the harness runs at: low, medium, high or xhigh (default the harness's)"},
					&cli.StringFlag{Name: "task", Usage: "Task name; its channel is #<task> (default $AGC_TASK)"},
					&cli.StringFlag{Name: "brief", Usage: "Path to the instructions", Value: "/home/agent/TASK.md"},
					&cli.StringFlag{Name: "name", Usage: "Messaging name to join as (default $AGC_SANDBOX, else the task name)"},
					&cli.StringFlag{Name: "manager", Usage: "Messaging name of the task's manager, who may post /accept and /reject"},
					&cli.StringFlag{Name: "owner", Usage: "Messaging name of the task's owner, who may post /accept and /reject"},
					&cli.StringFlag{Name: "agent", Usage: "Persistent agent to run as, in its home sandbox (default $AGC_AGENT when no task is given)"},
					&cli.StringFlag{Name: "sessions", Usage: "With --agent, directory of the sessions and cursor files (the kept volume)", Value: defaultHarnessDir},
					&cli.StringFlag{Name: "repo", Usage: "With --agent, the home's clone, where each thread gets its own git worktree (default the current directory when it is the top of a git repository)"},
				},
				Action: agentRun,
			},
		},
	}
}

func agentRun(c *cli.Context) error {
	taskName := firstNonEmpty(c.String("task"), os.Getenv("AGC_TASK"))

	agentName := c.String("agent")
	if agentName == "" && taskName == "" {
		agentName = os.Getenv("AGC_AGENT")
	}
	if agentName != "" {
		return agentHomeRun(c, agentName)
	}

	if taskName == "" {
		return ez.New(ez.EINVALID, "pass --task or --agent, or set AGC_TASK or AGC_AGENT", nil)
	}

	brief, err := os.ReadFile(c.String("brief"))
	if err != nil {
		return ez.New(ez.EINVALID, "cannot read the brief "+c.String("brief"), err)
	}

	harness, err := agentHarnessFor(c)
	if err != nil {
		return err
	}

	closers := map[string]bool{}
	for _, name := range []string{c.String("manager"), c.String("owner")} {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			closers[trimmed] = true
		}
	}

	loop := &agentLoop{
		harness: harness,
		msg:     newMsgClient(),
		task:    taskName,
		name:    firstNonEmpty(c.String("name"), os.Getenv("AGC_SANDBOX"), taskName),
		brief:   string(brief),
		closers: closers,
		log:     os.Stdout,
		joined: func(p *msgmodels.Participant) error {
			// agc msg post from the harness then speaks as the same
			// participant, so the loop does not take it for a reply.
			return saveMsgState(msgState{ParticipantID: p.ID, Name: p.Name})
		},
	}

	return loop.run(c.Context)
}

// agentHomeRun is agc agent run --agent: the loop of a persistent agent
// (ADR 0016) inside its home sandbox.
func agentHomeRun(c *cli.Context, name string) error {
	brief, err := os.ReadFile(c.String("brief"))
	if err != nil {
		return ez.New(ez.EINVALID, "cannot read the brief "+c.String("brief"), err)
	}

	harness, err := agentHarnessFor(c)
	if err != nil {
		return err
	}

	repo := c.String("repo")
	if repo == "" {
		// The home's job runs in its clone (sandboxes.workDir).
		cwd, err := os.Getwd()
		if err == nil {
			repo = homeRepo(c.Context, cwd)
		}
	}

	loop := &homeLoop{
		harness: harness,
		msg:     newMsgClient(),
		name:    name,
		brief:   string(brief),
		dir:     c.String("sessions"),
		repo:    repo,
		log:     os.Stdout,
		joined: func(p *msgmodels.Participant) error {
			return saveMsgState(msgState{ParticipantID: p.ID, Name: p.Name})
		},
	}

	prior, err := loadMsgState()
	if err == nil {
		loop.prior = &prior
	}

	return loop.run(c.Context)
}

func (l *agentLoop) channel() string {
	return "#" + l.task
}

func (l *agentLoop) logf(format string, args ...any) {
	if l.log != nil {
		fmt.Fprintf(l.log, "agc agent: "+format+"\n", args...)
	}
}

func (l *agentLoop) run(ctx context.Context) error {
	p, err := l.join(ctx)
	if err != nil {
		return err
	}
	defer l.leave(p)

	if l.joined != nil {
		err = l.joined(p)
		if err != nil {
			return err
		}
	}

	cursor, err := l.lastID(ctx)
	if err != nil {
		return err
	}

	l.logf("joined %s as @%s, first turn on the brief", l.channel(), p.Name)

	session := l.turn(ctx, p, l.brief, "")

	for {
		res, err := l.msg.Wait(ctx, l.channel(), cursor, agentWaitTimeout)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			l.logf("wait failed, retrying: %s", ez.ErrorMessage(err))
			l.pause(ctx)
			continue
		}

		for _, m := range res.Messages {
			cursor = m.ID

			if m.ParticipantID == p.ID {
				continue
			}

			if isSystem(m) && strings.HasPrefix(m.Body, msgmodels.AcceptedPrefix) {
				l.logf("the task is accepted (%s), exiting", strings.TrimSpace(m.Body))
				return nil
			}

			if !l.isTurn(m, p) {
				continue
			}

			prompt := messagePrompt(m, l.channel(), imagePaths(ctx, l.msg, l.images, m))
			if session == "" {
				// The first turn failed, so the brief was never read.
				prompt = l.brief + "\n\n---\n\n" + prompt
			}

			l.logf("message %d from @%s, next turn", m.ID, m.From)
			session = l.turn(ctx, p, prompt, session)
		}
	}
}

// messagePrompt is the prompt for a message in the task's channel: who
// wrote it where, its body, and the local path of each of its images.
func messagePrompt(m msgmodels.Message, channel string, images []string) string {
	prompt := "@" + m.From + " in " + channel + ":\n\n" + m.Body
	if len(images) > 0 {
		prompt += "\n\nImages (read each file to see it):\n" + imageLines(images)
	}

	return prompt
}

// isTurn reports whether m, not the loop's own, is the agent's next turn.
// agc's posts are not, except a rejection; another agent's post is only
// when it is a question or names the agent (msgmodels.TurnFor, ADR 0016
// item 6); the manager's and the owner's /accept and /reject are not
// either, since agc answers them in the channel.
func (l *agentLoop) isTurn(m msgmodels.Message, me *msgmodels.Participant) bool {
	if isSystem(m) {
		return strings.HasPrefix(m.Body, msgmodels.RejectedPrefix)
	}

	if !msgmodels.TurnFor(&m, me) {
		return false
	}

	if !l.closers[m.From] {
		return true
	}

	command, ok := tasks.ParseCommand(m.Body)
	if !ok {
		return true
	}

	return command.Verb == tasks.AcceptCommand && command.Arg != ""
}

// isSystem reports whether agc itself posted m.
func isSystem(m msgmodels.Message) bool {
	return msgmodels.FromSystem(&m)
}

// join takes the sandbox's name, or name-2 … name-5 when an earlier run
// still holds it.
func (l *agentLoop) join(ctx context.Context) (*msgmodels.Participant, error) {
	return joinAs(ctx, l.msg, l.name)
}

// joiner is the part of the messaging client joinAs uses.
type joiner interface {
	Join(ctx context.Context, name string) (*msgmodels.Participant, error)
}

// joinAs takes name, or name-2 … name-5 when it is held. Names are cut to
// the 32 characters messaging allows.
func joinAs(ctx context.Context, msg joiner, name string) (*msgmodels.Participant, error) {
	base := name
	if len(base) > 32 {
		base = strings.TrimRight(base[:32], "-")
	}

	var err error
	for n := 1; n <= 5; n++ {
		candidate := base
		if n > 1 {
			candidate = fmt.Sprintf("%s-%d", strings.TrimRight(base[:min(len(base), 30)], "-"), n)
		}

		var p *msgmodels.Participant
		p, err = msg.Join(ctx, candidate)
		if err == nil {
			return p, nil
		}

		if ez.ErrorCode(err) != ez.ECONFLICT {
			return nil, err
		}
	}

	return nil, err
}

func (l *agentLoop) leave(p *msgmodels.Participant) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := l.msg.Leave(ctx, p.ID)
	if err != nil {
		l.logf("cannot leave: %s", ez.ErrorMessage(err))
	}
}

// lastID is the newest message id in the task's channel, so the loop
// answers only what is posted from now on. Archived channels count too, so
// an archived #<task> never reads as empty.
func (l *agentLoop) lastID(ctx context.Context) (int64, error) {
	res, err := l.msg.Channels(ctx, true)
	if err != nil {
		return 0, err
	}

	for _, info := range res.Channels {
		if info.Channel == l.channel() {
			return info.LastID, nil
		}
	}

	return 0, nil
}

// turn runs the harness once and posts the answer, or the failure, in the
// channel; an answer starting with QUESTION: is posted as a question
// without the prefix; one starting with DONE: delivers instead, and is
// posted only when the delivery fails. It returns the session to continue, which
// stays the old one when the harness gave none.
func (l *agentLoop) turn(ctx context.Context, p *msgmodels.Participant, prompt string, session string) string {
	answer, next, err := l.harness.Run(ctx, prompt, session)
	if next == "" {
		next = session
	}

	body := strings.TrimSpace(answer)
	if err != nil {
		body = "The agent's harness failed: " + ez.ErrorMessage(err)
	}
	if body == "" {
		body = "(the agent answered with an empty message)"
	}
	if len(body) > msgmodels.MaxBodyBytes {
		body = body[:msgmodels.MaxBodyBytes-100] + "\n\n(cut: the answer was longer than a message allows)"
	}

	if err == nil && strings.HasPrefix(body, deliverPrefix) {
		summary := strings.TrimSpace(strings.TrimPrefix(body, deliverPrefix))

		var view any
		// DONE: delivers the summary only, with no report.
		err = l.msg.Deliver(ctx, summary, nil, nil, &view)
		if err == nil {
			l.logf("delivered, waiting for review")
			return next
		}

		l.logf("cannot deliver, posting the answer: %s", ez.ErrorMessage(err))
	}

	question := strings.TrimSpace(strings.TrimPrefix(body, questionPrefix))
	if err == nil && strings.HasPrefix(body, questionPrefix) && question != "" {
		_, err = l.msg.Ask(ctx, p.ID, l.channel(), question)
		if err == nil {
			l.logf("asked, waiting for the answer")
			return next
		}

		l.logf("cannot ask, posting the answer: %s", ez.ErrorMessage(err))
	}

	_, err = l.msg.Post(ctx, p.ID, l.channel(), body, nil)
	if err != nil {
		l.logf("cannot post the answer: %s", ez.ErrorMessage(err))
	}

	return next
}

func (l *agentLoop) pause(ctx context.Context) {
	pause(ctx)
}

// agentHarnessFor is the harness agc agent run's flags name.
func agentHarnessFor(c *cli.Context) (agentHarness, error) {
	return newAgentHarness(c.String("harness"), c.String("model"), c.String("effort"))
}

// newAgentHarness drives the harness CLI non-interactively, as
// runtime/harnesses does, without permission prompts: the VM is the
// sandbox.
func newAgentHarness(harness string, model string, effort string) (agentHarness, error) {
	effort = strings.TrimSpace(effort)
	if effort != "" {
		err := runtimetypes.ReasoningEffort(effort).Validate()
		if err != nil {
			return nil, ez.New(ez.EINVALID, "effort "+effort+" is not one of low, medium, high, xhigh", err)
		}
	}

	switch harness {
	case "claude_code":
		return claudeAgent{model: model, effort: effort}, nil
	case "codex_cli":
		return codexAgent{model: model, effort: effort}, nil
	}

	return nil, ez.New(ez.EINVALID, "agc agent run supports --harness claude_code or codex_cli, not "+harness, nil)
}

// runHarness runs name with args in the directory withHarnessDir put in
// ctx, else the current one (Claude Code's and Codex's tools run in the
// process's directory). stderr goes to
// this process's stderr, the terminal log, as well as into the error; a
// harness that says nothing on stderr is quoted from stdout.
func runHarness(ctx context.Context, name string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = harnessDir(ctx)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)

	err := cmd.Run()
	if err != nil {
		msg := firstNonEmpty(stderr.String(), stdout.String(), err.Error())
		if len(msg) > 2000 {
			msg = "..." + msg[len(msg)-2000:]
		}
		return "", ez.New(ez.EINTERNAL, name+" failed: "+msg, err)
	}

	return stdout.String(), nil
}

type claudeAgent struct {
	model  string
	effort string
}

// args is the claude command line, effort passed as runtime/harnesses
// does.
func (a claudeAgent) args(prompt string, session string) []string {
	args := []string{"-p", "--output-format", "json", "--dangerously-skip-permissions"}
	if a.model != "" {
		args = append(args, "--model", a.model)
	}
	if a.effort != "" {
		args = append(args, "--effort", a.effort)
	}
	if session != "" {
		args = append(args, "--resume", session)
	}

	return append(args, "--", prompt)
}

func (a claudeAgent) Run(ctx context.Context, prompt string, session string) (string, string, error) {
	stdout, err := runHarness(ctx, "claude", a.args(prompt, session))
	if err != nil {
		return "", "", err
	}

	return harnesses.ClaudeCodeAnswer(stdout)
}

type codexAgent struct {
	model  string
	effort string
}

func (a codexAgent) Run(ctx context.Context, prompt string, session string) (string, string, error) {
	dir, err := os.MkdirTemp("", "agc-agent")
	if err != nil {
		return "", "", ez.Wrap(err)
	}
	defer os.RemoveAll(dir)

	lastMessage := filepath.Join(dir, "last-message.txt")

	args := []string{"exec"}
	if session != "" {
		args = append(args, "resume", session)
	} else {
		args = append(args, "--dangerously-bypass-approvals-and-sandbox")
	}
	args = append(args, "--json", "--skip-git-repo-check", "-o", lastMessage)
	if a.model != "" {
		args = append(args, "-m", a.model)
	}
	if a.effort != "" {
		args = append(args, "-c", "model_reasoning_effort="+strconv.Quote(a.effort))
	}
	args = append(args, prompt)

	stdout, err := runHarness(ctx, "codex", args)
	if err != nil {
		return "", "", err
	}

	next, failure := harnesses.CodexSessionRef(stdout)
	if failure != "" {
		return "", next, ez.New(ez.EINTERNAL, "codex failed: "+failure, nil)
	}

	answer, err := os.ReadFile(lastMessage)
	if err != nil {
		return "", next, ez.New(ez.EINTERNAL, "codex wrote no final message", err)
	}

	return string(answer), next, nil
}
