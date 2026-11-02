package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	cli "github.com/urfave/cli/v2"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging/httpapi"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	"github.com/vanclief/agent-composer/runtime/harnesses"
)

const (
	// agentWaitTimeout is how long one wait for the next message blocks.
	agentWaitTimeout = time.Minute
	// agentRetryDelay is the pause after a failed wait before trying again.
	agentRetryDelay = 5 * time.Second
	// doneCommand closes a task when the manager or the owner posts it.
	doneCommand = "/done"
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
	Wait(ctx context.Context, channel string, after int64, timeout time.Duration) (*httpapi.MessagesResponse, error)
	Channels(ctx context.Context) (*httpapi.ChannelsResponse, error)
}

// agentLoop is `agc agent run` (ADR 0009 item 2): it joins #<task>, runs
// the harness on the brief, posts the answer, then answers every message
// from someone else on the same session until the manager or the owner
// posts /done, which gets one closing summary.
type agentLoop struct {
	harness agentHarness
	msg     agentMessenger
	task    string
	name    string
	brief   string
	// closers may close the task with /done: its manager and owner.
	closers map[string]bool
	log     io.Writer
	// joined is called with the participant once joined.
	joined func(p *msgmodels.Participant) error
}

func agentCommand() *cli.Command {
	return &cli.Command{
		Name:  "agent",
		Usage: "Run a task's agent inside its sandbox (ADR 0009)",
		Subcommands: []*cli.Command{
			{
				Name:  "run",
				Usage: "Join #<task>, run the harness on the brief, then answer every message in the channel until /done",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "harness", Usage: "claude_code or codex_cli", Required: true},
					&cli.StringFlag{Name: "model", Usage: "Model the harness runs"},
					&cli.StringFlag{Name: "task", Usage: "Task name; its channel is #<task> (default $AGC_TASK)"},
					&cli.StringFlag{Name: "brief", Usage: "Path to the instructions", Value: "/home/agent/TASK.md"},
					&cli.StringFlag{Name: "name", Usage: "Messaging name to join as (default $AGC_SANDBOX, else the task name)"},
					&cli.StringFlag{Name: "manager", Usage: "Messaging name of the task's manager, who may post /done"},
					&cli.StringFlag{Name: "owner", Usage: "Messaging name of the task's owner, who may post /done"},
				},
				Action: agentRun,
			},
		},
	}
}

func agentRun(c *cli.Context) error {
	taskName := firstNonEmpty(c.String("task"), os.Getenv("AGC_TASK"))
	if taskName == "" {
		return ez.New(ez.EINVALID, "pass --task or set AGC_TASK", nil)
	}

	brief, err := os.ReadFile(c.String("brief"))
	if err != nil {
		return ez.New(ez.EINVALID, "cannot read the brief "+c.String("brief"), err)
	}

	harness, err := newAgentHarness(c.String("harness"), c.String("model"))
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

			if strings.TrimSpace(m.Body) == doneCommand && l.closers[m.From] {
				l.logf("@%s posted /done, closing summary", m.From)
				prompt := "@" + m.From + " closed the task with /done. Write a short closing summary for " + l.channel() +
					": what you changed, what you verified, where the work is (branch and commits), and what is left undone."
				l.turn(ctx, p, prompt, session)
				return nil
			}

			prompt := "@" + m.From + " in " + l.channel() + ":\n\n" + m.Body
			if session == "" {
				// The first turn failed, so the brief was never read.
				prompt = l.brief + "\n\n---\n\n" + prompt
			}

			l.logf("message %d from @%s, next turn", m.ID, m.From)
			session = l.turn(ctx, p, prompt, session)
		}
	}
}

// join takes the sandbox's name, or name-2 … name-5 when an earlier run
// still holds it. Names are cut to the 32 characters messaging allows.
func (l *agentLoop) join(ctx context.Context) (*msgmodels.Participant, error) {
	base := l.name
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
		p, err = l.msg.Join(ctx, candidate)
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
// answers only what is posted from now on.
func (l *agentLoop) lastID(ctx context.Context) (int64, error) {
	res, err := l.msg.Channels(ctx)
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
// channel. It returns the session to continue, which stays the old one
// when the harness gave none.
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

	_, err = l.msg.Post(ctx, p.ID, l.channel(), body, nil)
	if err != nil {
		l.logf("cannot post the answer: %s", ez.ErrorMessage(err))
	}

	return next
}

func (l *agentLoop) pause(ctx context.Context) {
	timer := time.NewTimer(agentRetryDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// newAgentHarness drives the harness CLI non-interactively, as
// runtime/harnesses does, without permission prompts: the VM is the
// sandbox.
func newAgentHarness(harness string, model string) (agentHarness, error) {
	switch harness {
	case "claude_code":
		return claudeAgent{model: model}, nil
	case "codex_cli":
		return codexAgent{model: model}, nil
	}

	return nil, ez.New(ez.EINVALID, "agc agent run supports --harness claude_code or codex_cli, not "+harness, nil)
}

// runHarness runs name with args in the current directory. stderr goes to
// this process's stderr, the terminal log, as well as into the error; a
// harness that says nothing on stderr is quoted from stdout.
func runHarness(ctx context.Context, name string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)

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
	model string
}

func (a claudeAgent) Run(ctx context.Context, prompt string, session string) (string, string, error) {
	args := []string{"-p", "--output-format", "json", "--dangerously-skip-permissions"}
	if a.model != "" {
		args = append(args, "--model", a.model)
	}
	if session != "" {
		args = append(args, "--resume", session)
	}
	args = append(args, "--", prompt)

	stdout, err := runHarness(ctx, "claude", args)
	if err != nil {
		return "", "", err
	}

	return harnesses.ClaudeCodeAnswer(stdout)
}

type codexAgent struct {
	model string
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
