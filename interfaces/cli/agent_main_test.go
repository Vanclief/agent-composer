package cli

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/knowledge"
	msgclient "github.com/vanclief/agent-composer/messaging/client"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// The main agent never works, only talks and dispatches (ADR 0015): its
// claude gets Bash and nothing else, Bash only for agc's message, task
// and knowledge commands, nothing approved beyond that list, and every
// file and web tool denied by name.
func TestMainClaudeArgsAllowOnlyAgcCommands(t *testing.T) {
	args := mainClaude{model: "claude-opus-5-5", dir: "/home/agent/empty"}.args("the batch", "s1")

	if slices.Contains(args, "--dangerously-skip-permissions") || slices.Contains(args, "bypassPermissions") {
		t.Fatalf("main must not bypass permissions: %v", args)
	}

	value := func(flag string) []string {
		i := slices.Index(args, flag)
		if i < 0 {
			t.Fatalf("no %s in %v", flag, args)
		}

		values := []string{}
		for _, arg := range args[i+1:] {
			if strings.HasPrefix(arg, "--") {
				break
			}
			values = append(values, arg)
		}

		return values
	}

	got := value("--tools")
	if !slices.Equal(got, []string{"Bash"}) {
		t.Fatalf("--tools = %v, want only Bash", got)
	}
	got = value("--permission-mode")
	if !slices.Equal(got, []string{"dontAsk"}) {
		t.Fatalf("--permission-mode = %v, want dontAsk", got)
	}

	allowed := value("--allowedTools")
	want := []string{"Bash(agc msg *)", "Bash(agc task *)"}
	for _, kind := range []string{"beliefs", "decisions", "rules", "actions", "proposals", "gates"} {
		for _, verb := range []string{"show", "why"} {
			want = append(want, "Bash(agc "+kind+" "+verb+")", "Bash(agc "+kind+" "+verb+" *)")
		}
	}
	if !slices.Equal(allowed, want) {
		t.Fatalf("--allowedTools = %v\nwant %v", allowed, want)
	}
	for _, tool := range allowed {
		if !strings.HasPrefix(tool, "Bash(agc ") {
			t.Fatalf("allowed %q is not an agc command", tool)
		}
	}

	denied := value("--disallowedTools")
	for _, tool := range []string{"Read", "Write", "Edit", "MultiEdit", "NotebookEdit", "Glob", "Grep", "WebFetch", "WebSearch", "Task", "Agent"} {
		if !slices.Contains(denied, tool) {
			t.Fatalf("--disallowedTools %v misses %s", denied, tool)
		}
	}
	if slices.Contains(denied, "Bash") {
		t.Fatalf("Bash is main's one tool and must not be denied: %v", denied)
	}

	for _, flag := range []string{"--strict-mcp-config", "--disable-slash-commands"} {
		if !slices.Contains(args, flag) {
			t.Fatalf("no %s in %v", flag, args)
		}
	}

	got = value("--model")
	if !slices.Equal(got, []string{"claude-opus-5-5"}) {
		t.Fatalf("--model = %v", got)
	}
	got = value("--resume")
	if !slices.Equal(got, []string{"s1"}) {
		t.Fatalf("--resume = %v", got)
	}
	if args[len(args)-2] != "--" || args[len(args)-1] != "the batch" {
		t.Fatalf("the prompt is not last after --: %v", args)
	}

	fresh := mainClaude{dir: "/x"}.args("p", "")
	if slices.Contains(fresh, "--resume") || slices.Contains(fresh, "--model") {
		t.Fatalf("a new session with no model = %v", fresh)
	}
}

// The harness runs in the empty working directory, with the arguments
// above, and refuses one that is not empty.
func TestMainClaudeRunsInTheEmptyDirectory(t *testing.T) {
	bin := t.TempDir()
	record := filepath.Join(t.TempDir(), "record")
	script := "#!/bin/sh\nprintf '%s\\n' \"$PWD\" \"$@\" > " + record + "\nprintf '%s' '{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"quiet\",\"session_id\":\"s9\"}'\n"
	err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	dir := filepath.Join(t.TempDir(), "empty")
	h, err := newMainHarness("claude_code", "", dir)
	if err != nil {
		t.Fatal(err)
	}

	answer, session, err := h.Run(context.Background(), "the batch", "")
	if err != nil || answer != "quiet" || session != "s9" {
		t.Fatalf("Run = %q, %q, %v", answer, session, err)
	}

	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if lines[0] != dir {
		t.Fatalf("claude ran in %s, want %s", lines[0], dir)
	}
	if !slices.Equal(lines[1:], mainClaude{dir: dir}.args("the batch", "")) {
		t.Fatalf("claude got %v", lines[1:])
	}

	err = os.WriteFile(filepath.Join(dir, "notes.md"), []byte("x"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = h.Run(context.Background(), "the batch", "")
	if err == nil || !strings.Contains(ez.ErrorMessage(err), "must be empty") {
		t.Fatalf("Run in a directory that is not empty: %v", err)
	}
}

func TestMainHarnessIsClaudeCodeOnly(t *testing.T) {
	_, err := newMainHarness("codex_cli", "", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "claude_code only") {
		t.Fatalf("codex_cli: %v", err)
	}
}

func TestBatchPromptInlinesAndSpills(t *testing.T) {
	long := strings.Repeat("é", 300) + "the rest"
	reply := int64(7)
	got := BatchPrompt([]AgentRecord{
		{ID: 7, Channel: "#franco", From: "main-1", Kind: "question", Body: "Create this?\ntitle: x"},
		{ID: 9, Channel: "#franco", From: "franco", Human: true, Body: "yes", ReplyTo: &reply},
		{ID: 12, Channel: "#ads", From: "agc", Body: long},
	})

	want := "## New messages since your last turn (3)\n\n" +
		"- id 7 · #franco · @main-1 · question\n  Create this?\n  title: x\n" +
		"- id 9 · #franco · @franco (human) · message · reply to 7\n  yes\n" +
		"- id 12 · #ads · @agc · message\n  " + strings.Repeat("é", 300) + "\n  … 8 more characters: agc msg read '#ads' --after 11 --limit 1\n" +
		"\nRead around a message with: agc msg read '<channel>' --after <id>\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestMainPromptHoldsOnlyWhoToolsRulesGatesAndBatch(t *testing.T) {
	records := []AgentRecord{{ID: 1, Channel: "#franco", From: "franco", Human: true, Body: "hi"}}
	first := MainPrompt(MainTurn{Name: "main-1", Owner: "franco", Brief: "# Main\n", Rules: "## main\n\n- ☐ Stay quiet.", Gates: "- default: decide", Tier: knowledge.Decide, Records: records})

	for _, part := range []string{
		"You are @main-1, agc's main agent. Your owner is franco; their channel is #franco.",
		"# Main",
		"## Your tools",
		"agc task create -f -",
		"## The rules you are trusted with (agc rules show --trusted main --ids)\n\n## main\n\n- ☐ Stay quiet.",
		"## Gates (agc gates show --ids)\n\n- default: decide",
		"Your task acts (proposes: task by main: create, accept, reject) are at tier decide now: no gate covers them, so the default applies.",
		BatchPrompt(records),
	} {
		if !strings.Contains(first, part) {
			t.Fatalf("first prompt misses %q:\n%s", part, first)
		}
	}

	later := MainPrompt(MainTurn{Name: "main-1", Owner: "franco", Tier: knowledge.Observe, Gate: "main-tasks-observed", TierError: "", Records: records})
	if strings.Contains(later, "## Your tools") || strings.Contains(later, "You are @main-1") {
		t.Fatalf("a resumed session gets who and tools again:\n%s", later)
	}
	if !strings.Contains(later, "at tier observe now: gate main-tasks-observed.") || !strings.Contains(later, "(no rule trusts main)") {
		t.Fatalf("later prompt:\n%s", later)
	}

	failed := MainPrompt(MainTurn{Tier: knowledge.Decide, TierError: "no gates/"})
	if !strings.Contains(failed, "The gates could not be read (no gates/), so every act is decide.") {
		t.Fatalf("failed prompt:\n%s", failed)
	}
}

// main takes one turn per batch on one session, never for its own
// messages (by participant, by name or posted from its home by the
// harness) nor agc's sleep notices, but for agc's other posts (triggers);
// it posts nothing itself, and a failing harness is said in #<owner>.
func TestHomeLoopMainBatches(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fromHome := homeMsg(4, "#franco", "main-1-2", 0, "posted by the harness")
	fromHome.Agent = "main-1"
	trigger := fromAgcIn(5, "#ads", "Trigger: the ads job failed")
	asleep := fromAgcIn(6, "@franco/reviewer-1", "Asleep.")

	msg := &fakeHomeMessenger{
		cancel: cancel,
		nextID: 100,
		agent:  msgclient.AgentResponse{Agent: msgclient.AgentView{Name: "main-1", Role: "main", Owner: "franco", Home: "main-1"}, Channels: []string{"#franco", "#ads"}, Idle: "15m"},
		script: [][]msgmodels.Message{
			{homeMsg(1, "#franco", "franco", 0, "make a task for the ads"), homeMsg(2, "#ads", "main-1", 0, "my own")},
			{homeMsg(3, "#ads", "ads-agent", 0, "done?"), fromHome, trigger, asleep},
			{asleep},
			{homeMsg(7, "#franco", "franco", 0, "and?")},
		},
	}
	harness := &homeHarness{failAt: map[int]bool{3: true}}
	loop, _ := newHomeLoop(t, msg, harness)
	loop.name = "main-1"
	loop.brief = "# Main\n"

	reads := []string{}
	pulls := 0
	loop.main = &mainMode{
		knowledge: func(ctx context.Context, args ...string) (string, error) {
			reads = append(reads, strings.Join(args, " "))
			if args[0] == "gates" {
				return "", ez.New(ez.EINTERNAL, "boom", nil)
			}
			return "- ☐ Stay quiet.\n", nil
		},
		tier: func() (knowledge.Tier, string, error) {
			return knowledge.Decide, "", nil
		},
		refresh: func(ctx context.Context) error {
			pulls++
			return nil
		},
	}

	err := loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(harness.turns) != 3 {
		t.Fatalf("turns = %d, want 3 (the batch of agc's sleep notice alone is no turn): %#v", len(harness.turns), harness.turns)
	}

	first := harness.turns[0]
	if first.session != "" || !strings.Contains(first.prompt, "You are @main-1") || !strings.Contains(first.prompt, "# Main") || !strings.Contains(first.prompt, "make a task for the ads") || strings.Contains(first.prompt, "my own") {
		t.Fatalf("first turn = %#v", first)
	}
	if !strings.Contains(first.prompt, "- ☐ Stay quiet.") || !strings.Contains(first.prompt, "(agc gates show --ids failed: boom)") {
		t.Fatalf("first turn misses the rules or the gates failure:\n%s", first.prompt)
	}

	second := harness.turns[1]
	if second.session != "s1" || strings.Contains(second.prompt, "You are @main-1") {
		t.Fatalf("second turn = %#v", second)
	}
	if !strings.Contains(second.prompt, "done?") || !strings.Contains(second.prompt, "Trigger: the ads job failed") {
		t.Fatalf("second turn misses a message or the trigger:\n%s", second.prompt)
	}
	if strings.Contains(second.prompt, "posted by the harness") || strings.Contains(second.prompt, "Asleep.") {
		t.Fatalf("second turn has main's own post or a sleep notice:\n%s", second.prompt)
	}

	if !slices.Equal(reads[:2], []string{"rules show --trusted main --ids", "gates show --ids"}) || len(reads) != 6 || pulls != 3 {
		t.Fatalf("knowledge reads = %v, pulls = %d", reads, pulls)
	}

	// Two quiet turns posted nothing; the failed one says so in #franco.
	if len(msg.posts) != 1 || msg.posts[0].channel != "#franco" || msg.posts[0].replyTo != 0 || !strings.Contains(msg.posts[0].body, "The main agent's harness failed on a batch of 1 message(s): claude failed: out of credits") {
		t.Fatalf("posts = %+v", msg.posts)
	}
}

// A main agent whose harness cannot be restricted does not run.
func TestHomeLoopMainRefusesAnUnrestrictedHarness(t *testing.T) {
	msg := &fakeHomeMessenger{
		agent: msgclient.AgentResponse{Agent: msgclient.AgentView{Name: "main-1", Role: "main"}, Channels: []string{"#a"}, Idle: "15m"},
	}
	loop, _ := newHomeLoop(t, msg, &homeHarness{})
	loop.main = newMainMode("codex_cli", "", t.TempDir(), "")

	err := loop.run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "claude_code only") {
		t.Fatalf("run: %v", err)
	}
}

func TestTaskTierReadsTheGates(t *testing.T) {
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, "gates"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	tier, gate, err := taskTier(dir)
	if err != nil || tier != knowledge.Decide || gate != "" {
		t.Fatalf("no gates: %s, %q, %v", tier, gate, err)
	}

	err = os.WriteFile(filepath.Join(dir, "gates", "main.yaml"), []byte("gates:\n  - id: main-tasks-observed\n    proposes: task\n    by: [main]\n    tier: observe\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	tier, gate, err = taskTier(dir)
	if err != nil || tier != knowledge.Observe || gate != "main-tasks-observed" {
		t.Fatalf("observe gate: %s, %q, %v", tier, gate, err)
	}

	tier, _, err = taskTier("")
	if err == nil || tier != knowledge.Decide {
		t.Fatalf("no dir: %s, %v", tier, err)
	}
}

func TestKnowledgeRootFallsBackToTheKnowledgeDir(t *testing.T) {
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, "gates"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	t.Chdir(t.TempDir())
	t.Setenv("AGC_KNOWLEDGE_DIR", dir)

	got := existing("gates", "rules")
	if !slices.Equal(got, []string{filepath.Join(dir, "gates")}) {
		t.Fatalf("existing in an empty directory = %v", got)
	}

	err = os.MkdirAll("rules", 0o755)
	if err != nil {
		t.Fatal(err)
	}
	got = existing("gates", "rules")
	if !slices.Equal(got, []string{"rules"}) {
		t.Fatalf("existing where rules/ is = %v", got)
	}
}

// main creates a task from the exact yaml it asked about, on stdin, and
// the file's accept criteria reach the task.
func TestTaskFileFromStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.WriteString("title: Retry the ads job\nbrief: Run it again.\naccept: [The job is green.]\n")
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	stdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = stdin
	})

	parsed, err := loadTaskFile(context.Background(), nil, "-", "")
	if err != nil || parsed.Title != "Retry the ads job" || !slices.Equal(parsed.Accept, []string{"The job is green."}) {
		t.Fatalf("parsed = %+v, %v", parsed, err)
	}
}
