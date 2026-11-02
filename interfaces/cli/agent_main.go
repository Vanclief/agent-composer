package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/agents"
	"github.com/vanclief/agent-composer/knowledge"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	"github.com/vanclief/agent-composer/runtime/harnesses"
	"github.com/vanclief/agent-composer/task"
)

// The main agent (ADR 0015): the home loop of the role main, which reads
// every channel, takes one turn per batch of new messages on one long
// session, and only talks and dispatches. Its harness runs in an empty
// directory with Bash limited to agc's message, task and knowledge
// commands; what it may do on its own is a gate with proposes: task.

const (
	// mainWorkDir is the main agent's working directory: empty, so there
	// is nothing to read or change, and fixed, since Claude Code keys a
	// session to its directory and main resumes one session for good.
	mainWorkDir = task.HomeDir + "/empty"
	// inlineChars is how much of a message body the batch carries; the
	// rest is read with agc msg read.
	inlineChars = 300
	// mainProposer is who main's acts are by in gates/: its role.
	mainProposer = "main"
	// taskActs is the proposal kind of creating, accepting and rejecting a
	// task (knowledge.Proposable).
	taskActs = "task"
	// knowledgeTimeout bounds one knowledge command or pull.
	knowledgeTimeout = time.Minute
)

// mainAllowedTools is the main agent's whole tool allow list: Bash limited
// to agc msg and agc task, and the show and why commands of the knowledge
// kinds. A bare form and a form with arguments each, so `agc gates show`
// matches as well as `agc gates show --ids`.
func mainAllowedTools() []string {
	tools := []string{"Bash(agc msg *)", "Bash(agc task *)"}
	for _, kind := range []string{"beliefs", "decisions", "rules", "actions", "proposals", "gates"} {
		for _, verb := range []string{"show", "why"} {
			tools = append(tools, "Bash(agc "+kind+" "+verb+")", "Bash(agc "+kind+" "+verb+" *)")
		}
	}

	return tools
}

// mainDisallowedTools are every file, shell-adjacent, agent and web tool
// Claude Code has, denied by name although --tools already leaves them
// out: if a version adds one back to the set, it is still denied.
var mainDisallowedTools = []string{
	"Read", "Write", "Edit", "MultiEdit", "NotebookEdit", "NotebookRead",
	"Glob", "Grep", "LS",
	"WebFetch", "WebSearch",
	"Task", "Agent", "BashOutput", "KillShell", "Monitor",
	"Skill", "SlashCommand", "TodoWrite", "ExitPlanMode",
}

// mainClaude is Claude Code as the main agent: only Bash, only the agc
// commands of mainAllowedTools, nothing else approved (dontAsk denies what
// the list does not name, where the task agent runs with
// --dangerously-skip-permissions), no MCP servers, no slash commands, in
// dir.
type mainClaude struct {
	model string
	dir   string
}

// args are the claude arguments of one turn.
func (a mainClaude) args(prompt string, session string) []string {
	args := []string{"-p", "--output-format", "json", "--permission-mode", "dontAsk", "--tools", "Bash"}
	args = append(args, "--allowedTools")
	args = append(args, mainAllowedTools()...)
	args = append(args, "--disallowedTools")
	args = append(args, mainDisallowedTools...)
	args = append(args, "--strict-mcp-config", "--disable-slash-commands")
	if a.model != "" {
		args = append(args, "--model", a.model)
	}
	if session != "" {
		args = append(args, "--resume", session)
	}

	return append(args, "--", prompt)
}

func (a mainClaude) Run(ctx context.Context, prompt string, session string) (string, string, error) {
	err := ensureEmptyDir(a.dir)
	if err != nil {
		return "", "", err
	}

	stdout, err := runHarnessIn(ctx, a.dir, "claude", a.args(prompt, session))
	if err != nil {
		return "", "", err
	}

	return harnesses.ClaudeCodeAnswer(stdout)
}

// newMainHarness is the main agent's harness. Only Claude Code can be held
// to a list of commands (codex exec has no per-command allow list), so
// main refuses any other.
func newMainHarness(harness string, model string, dir string) (agentHarness, error) {
	if harness != "claude_code" {
		return nil, ez.New(ez.EINVALID, "the main agent runs on claude_code only, the harness whose tools can be limited to agc commands (ADR 0015); its template names "+harness, nil)
	}

	return mainClaude{model: model, dir: dir}, nil
}

// errNotEmpty says the working directory has something in it.
var errNotEmpty = errors.New("not empty")

// ensureEmptyDir makes dir, and refuses to run in it when it holds
// anything: main has no file tools, so something there means something
// other than main wrote it.
func ensureEmptyDir(dir string) error {
	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		return ez.Wrap(err)
	}

	f, err := os.Open(dir)
	if err != nil {
		return ez.Wrap(err)
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return ez.Wrap(err)
	}

	return ez.New(ez.EINVALID, "the main agent's working directory "+dir+" must be empty and is not: empty it by hand", errNotEmpty)
}

// runHarnessIn is runHarness in dir.
func runHarnessIn(ctx context.Context, dir string, name string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

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

// mainMode is what the role main adds to the home loop. Every field may
// be nil in tests.
type mainMode struct {
	// harness is the restricted harness; err why there is none.
	harness agentHarness
	err     error
	// knowledge runs an agc knowledge command and returns what it printed.
	knowledge func(ctx context.Context, args ...string) (string, error)
	// tier is the tier main's task acts get now and the gate giving it.
	tier func() (knowledge.Tier, string, error)
	// refresh brings the knowledge clone up to date before a turn.
	refresh func(ctx context.Context) error
}

// newMainMode runs the knowledge commands with this agc binary in the
// empty working directory, where they read $AGC_KNOWLEDGE_DIR (dir), and
// pulls dir before every turn so a gate Franco merges counts at once.
func newMainMode(harness string, model string, workDir string, dir string) *mainMode {
	h, err := newMainHarness(harness, model, workDir)

	mode := &mainMode{harness: h, err: err}

	mode.knowledge = func(ctx context.Context, args ...string) (string, error) {
		self, err := os.Executable()
		if err != nil {
			return "", ez.Wrap(err)
		}

		ctx, cancel := context.WithTimeout(ctx, knowledgeTimeout)
		defer cancel()

		err = os.MkdirAll(workDir, 0o700)
		if err != nil {
			return "", ez.Wrap(err)
		}

		return runHarnessIn(ctx, workDir, self, args)
	}

	mode.tier = func() (knowledge.Tier, string, error) {
		return taskTier(dir)
	}

	mode.refresh = func(ctx context.Context) error {
		if dir == "" {
			return nil
		}

		_, err := os.Stat(filepath.Join(dir, ".git"))
		if err != nil {
			return nil
		}

		ctx, cancel := context.WithTimeout(ctx, knowledgeTimeout)
		defer cancel()

		_, err = runHarnessIn(ctx, dir, "git", []string{"pull", "--ff-only", "--quiet"})
		return err
	}

	return mode
}

// taskTier loads the gates under dir and says the tier of main's task
// acts. No dir, or no gates there, is the default: decide.
func taskTier(dir string) (knowledge.Tier, string, error) {
	if dir == "" {
		return knowledge.Decide, "", ez.New(ez.EINVALID, "AGC_KNOWLEDGE_DIR is not set, so no gate is read", nil)
	}

	paths := []string{}
	for _, kind := range knowledgeKinds {
		path := filepath.Join(dir, kind)

		_, err := os.Stat(path)
		if err == nil {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return knowledge.Decide, "", ez.New(ez.EINVALID, "no knowledge directories under "+dir, nil)
	}

	set, err := knowledge.Load(paths...)
	if err != nil {
		return knowledge.Decide, "", ez.Wrap(err)
	}

	tier, gate := set.TierFor(taskActs, mainProposer)

	return tier, gate, nil
}

// isMainTurn is isTurn for main: every message is for it, agc's included
// (triggers land as agc's messages, ADR 0014), except its own, those older
// than since and agc's sleep notices.
func (l *homeLoop) isMainTurn(m msgmodels.Message, since time.Time) bool {
	if m.ParticipantID == l.me.ID || m.From == l.me.Name {
		return false
	}

	if l.home != "" && m.Agent == l.home {
		return false
	}

	if !since.IsZero() && m.CreatedAt.Before(since) {
		return false
	}

	return !(isSystem(m) && strings.TrimSpace(m.Body) == agents.SleepNotice)
}

// takeMain runs main's turn on a batch. Its answer is not posted: main
// says what it has to say with agc msg, and a batch that needs nothing
// ends with no post. A failing harness is said in #<owner>.
func (l *homeLoop) takeMain(ctx context.Context, t homeTurn) {
	session := l.sessions.Get(t.key)
	turn := l.mainTurn(ctx, t.records, session == "")

	harness := l.harness
	if l.main != nil && l.main.harness != nil {
		harness = l.main.harness
	}

	l.logf("main turn with %d new message(s)", len(t.records))

	answer, next, err := harness.Run(ctx, MainPrompt(turn), session)
	if next != "" && next != session {
		saveErr := l.sessions.Set(t.key, next)
		if saveErr != nil {
			l.logf("cannot save the sessions: %s", ez.ErrorMessage(saveErr))
		}
	}

	if err == nil {
		first, _, _ := strings.Cut(strings.TrimSpace(answer), "\n")
		l.logf("main turn done: %s", first)
		return
	}

	l.logf("main's harness failed: %s", ez.ErrorMessage(err))
	if l.owner == "" {
		return
	}

	body := "The main agent's harness failed on a batch of " + fmt.Sprint(len(t.records)) + " message(s): " + ez.ErrorMessage(err)
	if len(body) > msgmodels.MaxBodyBytes {
		body = body[:msgmodels.MaxBodyBytes-100] + "…"
	}

	_, postErr := l.msg.Post(ctx, l.me.ID, "#"+l.owner, body, nil)
	if postErr != nil {
		l.logf("cannot say so in #%s: %s", l.owner, ez.ErrorMessage(postErr))
	}
}

// mainTurn gathers what main's prompt holds this turn, reading the rules
// and the gates afresh.
func (l *homeLoop) mainTurn(ctx context.Context, records []AgentRecord, first bool) MainTurn {
	turn := MainTurn{Name: l.me.Name, Owner: l.owner, Records: records, Tier: knowledge.Decide}
	if first {
		turn.Brief = l.brief
	}

	mode := l.main
	if mode == nil {
		return turn
	}

	if mode.refresh != nil {
		err := mode.refresh(ctx)
		if err != nil {
			l.logf("cannot update the knowledge clone, reading it as it is: %s", ez.ErrorMessage(err))
		}
	}

	if mode.knowledge != nil {
		turn.Rules = knowledgeSection(ctx, mode.knowledge, "rules", "show", "--trusted", mainProposer, "--ids")
		turn.Gates = knowledgeSection(ctx, mode.knowledge, "gates", "show", "--ids")
	}

	if mode.tier != nil {
		tier, gate, err := mode.tier()
		turn.Tier = tier
		turn.Gate = gate
		if err != nil {
			turn.TierError = ez.ErrorMessage(err)
		}
	}

	return turn
}

func knowledgeSection(ctx context.Context, run func(ctx context.Context, args ...string) (string, error), args ...string) string {
	out, err := run(ctx, args...)
	if err != nil {
		return "(agc " + strings.Join(args, " ") + " failed: " + ez.ErrorMessage(err) + ")"
	}

	return strings.TrimSpace(out)
}

// MainTurn is what one prompt of the main agent is made of: who it is
// (Name, Owner, and Brief, AGENT.md, on a session's first turn), the
// rules it is trusted with, the gates and the tier of its task acts, and
// the batch.
type MainTurn struct {
	Name  string
	Owner string
	Brief string
	// Rules is agc rules show --trusted main --ids; Gates agc gates show
	// --ids.
	Rules string
	Gates string
	// Tier and Gate are what a task act gets now; TierError why the gates
	// could not be read, in which case Tier is decide.
	Tier      knowledge.Tier
	Gate      string
	TierError string
	Records   []AgentRecord
}

// MainPrompt is the main agent's prompt for one batch, and all of it:
// who it is and its tools on a session's first turn, then every turn the
// rules it is trusted with, the gates and the batch. What main should do
// is in the rules (rules/main.yaml), not here.
func MainPrompt(t MainTurn) string {
	var b strings.Builder

	if t.Brief != "" {
		owner := "You have no owner, so nobody's channel is yours to escalate in; say so in your answer."
		if t.Owner != "" {
			owner = "Your owner is " + t.Owner + "; their channel is #" + t.Owner + "."
		}
		fmt.Fprintf(&b, "You are @%s, agc's main agent. %s\n", t.Name, owner)
		b.WriteString("You read every channel and act only by talking and by creating, accepting and rejecting tasks.\n\n")
		b.WriteString(strings.TrimSpace(t.Brief))
		b.WriteString("\n\n## Your tools\n\n")
		b.WriteString(mainTools)
		b.WriteString("\n")
	}

	b.WriteString("## The rules you are trusted with (agc rules show --trusted main --ids)\n\n")
	b.WriteString(orNone(t.Rules, "(no rule trusts main)"))
	b.WriteString("\n\n## Gates (agc gates show --ids)\n\n")
	b.WriteString(orNone(t.Gates, "(no gates)"))
	b.WriteString("\n\n")

	gate := "no gate covers them, so the default applies"
	if t.Gate != "" {
		gate = "gate " + t.Gate
	}
	fmt.Fprintf(&b, "Your task acts (proposes: %s by %s: create, accept, reject) are at tier %s now: %s.\n", taskActs, mainProposer, t.Tier, gate)
	if t.TierError != "" {
		b.WriteString("The gates could not be read (" + t.TierError + "), so every act is decide.\n")
	}

	b.WriteString("\n")
	b.WriteString(BatchPrompt(t.Records))

	return b.String()
}

// mainTools says the main agent's tools as mainClaude allows them.
const mainTools = `Your one tool is Bash, and it runs only these agc commands; anything else is refused. You have no file, web or other shell tools, and your working directory is empty.

- agc msg read '<channel>' --after <id> [--limit <n>]: read messages (JSON, with human, kind and reply_to).
- agc msg post '<channel>' <text> [--reply-to <id>]: say something. agc msg ask '<channel>' <text>: ask, so it shows as waiting.
- agc task create -f - <<'EOF' … EOF: create the task whose yaml is on stdin (title, brief, accept, assignee, model; you are its manager).
- agc task get <name>, agc task accept <name>, agc task reject <name> --why <reason>: the tasks you manage.
- agc beliefs|decisions|rules|actions|proposals|gates show [--ids], and agc beliefs|proposals why <id>.

Your final answer is never posted; it stays in your transcript as the trace. To say anything, post it. A batch that needs nothing from you ends with no post.
`

func orNone(text string, none string) string {
	if strings.TrimSpace(text) == "" {
		return none
	}

	return text
}

// inline cuts body to inlineChars characters, and says how many are left.
func inline(body string) (string, int) {
	body = strings.TrimSpace(body)
	if utf8.RuneCountInString(body) <= inlineChars {
		return body, 0
	}

	runes := []rune(body)

	return string(runes[:inlineChars]), len(runes) - inlineChars
}
