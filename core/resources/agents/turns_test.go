package agents

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/vanclief/ez"
)

const (
	claudeFixture = "testdata/claude-two-turns.jsonl"
	codexFixture  = "testdata/codex-one-turn.jsonl"
	claudeSession = "11111111-2222-4333-8444-555555555555"
	codexSession  = "019a0000-0000-7000-8000-000000000001"
)

// putFile copies a fixture, or writes body, to rel under the volume dir.
func putFile(t *testing.T, dir string, rel string, body string) {
	t.Helper()

	full := filepath.Join(dir, filepath.FromSlash(rel))
	err := os.MkdirAll(filepath.Dir(full), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(full, []byte(body), 0o644)
	if err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T, name string) string {
	t.Helper()

	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}

	return string(body)
}

// newVolumes is a volume root with agc-reviewer-1 holding the Claude Code
// session (turns at 10:00 and 10:05) and the Codex one (11:00).
func newVolumes(t *testing.T) (string, string) {
	t.Helper()

	root := t.TempDir()
	dir := filepath.Join(root, "agc-reviewer-1")
	putFile(t, dir, ".claude/projects/-home-agent-project/"+claudeSession+".jsonl", fixture(t, claudeFixture))
	putFile(t, dir, ".codex/sessions/2026/11/02/rollout-2026-11-02T11-00-00-"+codexSession+".jsonl", fixture(t, codexFixture))

	return root, dir
}

func TestClaudeTurns(t *testing.T) {
	parsed := parseClaude(strings.NewReader(fixture(t, claudeFixture)), true)
	if parsed.session != claudeSession || parsed.skipped != 0 {
		t.Fatalf("session %q skipped %d", parsed.session, parsed.skipped)
	}
	if len(parsed.turns) != 2 {
		t.Fatalf("turns = %d, want 2 (the tool result does not start one)", len(parsed.turns))
	}

	first := parsed.turns[0]
	if first.Injected != "List the ADRs in docs/decisions." {
		t.Fatalf("injected %q", first.Injected)
	}
	if !first.StartedAt.Equal(time.Date(2026, 11, 2, 10, 0, 0, 100e6, time.UTC)) || !first.EndedAt.Equal(time.Date(2026, 11, 2, 10, 0, 5, 0, time.UTC)) {
		t.Fatalf("first turn %s – %s", first.StartedAt, first.EndedAt)
	}
	if len(first.ToolCalls) != 1 {
		t.Fatalf("tool calls %+v", first.ToolCalls)
	}

	call := first.ToolCalls[0]
	if call.Name != "Bash" || !strings.Contains(string(call.Args), `"command":"ls docs/decisions"`) || call.Result != "0001-git.md\n0002-storage.md" || call.Truncated || call.Error {
		t.Fatalf("tool call %+v args %s", call, call.Args)
	}
	if first.Answer != "There are two ADRs: 0001 and 0002.\nBoth are accepted." {
		t.Fatalf("answer %q", first.Answer)
	}

	// msg_A1 is written twice (thinking, tool_use) with the same usage:
	// counted once, plus msg_A2.
	want := Tokens{Input: 3 + 2, Output: 120 + 40, CacheRead: 12000 + 12900, CacheWrite: 900 + 150}
	if first.Tokens == nil || *first.Tokens != want {
		t.Fatalf("tokens %+v, want %+v", first.Tokens, want)
	}

	second := parsed.turns[1]
	if second.Injected != "Which one is newer?" || second.Answer != "0002-storage.md is newer." || len(second.ToolCalls) != 0 {
		t.Fatalf("second turn %+v", second)
	}
	if second.Tokens == nil || *second.Tokens != (Tokens{Input: 1, Output: 12, CacheRead: 13050, CacheWrite: 30}) {
		t.Fatalf("second tokens %+v", second.Tokens)
	}
}

func TestCodexTurns(t *testing.T) {
	parsed := parseCodex(strings.NewReader(fixture(t, codexFixture)), true)
	if parsed.session != codexSession {
		t.Fatalf("session %q", parsed.session)
	}
	if len(parsed.turns) != 1 {
		t.Fatalf("turns = %d, want 1 (environment_context is not a turn)", len(parsed.turns))
	}

	turn := parsed.turns[0]
	if turn.Injected != "run `echo hello` in the shell then say hi" || turn.Answer != "hi" {
		t.Fatalf("turn %+v", turn)
	}
	if len(turn.ToolCalls) != 1 || turn.ToolCalls[0].Name != "exec_command" || string(turn.ToolCalls[0].Args) != `{"cmd":"echo hello"}` || turn.ToolCalls[0].Result != "hello\n" {
		t.Fatalf("tool calls %+v", turn.ToolCalls)
	}

	// The running total at the end, less nothing before the turn; input
	// leaves the cached tokens out.
	if turn.Tokens == nil || *turn.Tokens != (Tokens{Input: 2100, Output: 45, CacheRead: 8000}) {
		t.Fatalf("tokens %+v", turn.Tokens)
	}
}

func TestToolResultCut(t *testing.T) {
	long := strings.Repeat("é", MaxToolResult)
	body := `{"type":"user","sessionId":"s","timestamp":"2026-11-02T10:00:00Z","message":{"role":"user","content":"go"}}
{"type":"assistant","sessionId":"s","timestamp":"2026-11-02T10:00:01Z","message":{"id":"m1","role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"x"}}]}}
{"type":"user","sessionId":"s","timestamp":"2026-11-02T10:00:02Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":[{"type":"text","text":"` + long + `"}]}]}}
`
	parsed := parseClaude(strings.NewReader(body), true)
	if len(parsed.turns) != 1 || len(parsed.turns[0].ToolCalls) != 1 {
		t.Fatalf("turns %+v", parsed.turns)
	}

	call := parsed.turns[0].ToolCalls[0]
	if !call.Truncated || !call.Error || len(call.Result) > MaxToolResult || len(call.Result) < MaxToolResult-1 || !strings.HasPrefix(long, call.Result) {
		t.Fatalf("truncated %v error %v len %d", call.Truncated, call.Error, len(call.Result))
	}
	if parsed.turns[0].Tokens != nil {
		t.Fatalf("no usage in the file, tokens %+v", parsed.turns[0].Tokens)
	}
}

func TestListTurnsOrdersAcrossSessions(t *testing.T) {
	root, _ := newVolumes(t)
	api := NewTurnsAPI(root, nil)

	list, err := api.ListTurns(context.Background(), nil, &ListTurnsRequest{Name: "reviewer-1"})
	if err != nil {
		t.Fatal(err)
	}
	if list.Skipped != 0 || len(list.Turns) != 3 {
		t.Fatalf("list %+v", list)
	}

	// Newest first, numbered by start time across both sessions.
	wants := []struct {
		n       int
		session string
		harness string
		answer  string
		calls   int
	}{
		{3, codexSession, "codex_cli", "hi", 1},
		{2, claudeSession, "claude_code", "0002-storage.md is newer.", 0},
		{1, claudeSession, "claude_code", "There are two ADRs: 0001 and 0002.", 1},
	}
	for i, want := range wants {
		got := list.Turns[i]
		if got.N != want.n || got.Session != want.session || got.Harness != want.harness || got.Answer != want.answer || got.ToolCalls != want.calls {
			t.Fatalf("turn %d = %+v, want %+v", i, got, want)
		}
	}

	page, err := api.ListTurns(context.Background(), nil, &ListTurnsRequest{Name: "reviewer-1", TurnsPage: TurnsPage{Limit: 1, Before: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Turns) != 1 || page.Turns[0].N != 2 {
		t.Fatalf("page %+v", page.Turns)
	}

	turn, err := api.GetTurn(context.Background(), nil, &GetTurnRequest{Name: "reviewer-1", N: 1})
	if err != nil {
		t.Fatal(err)
	}
	if turn.N != 1 || turn.Injected != "List the ADRs in docs/decisions." || len(turn.ToolCalls) != 1 || turn.ToolCalls[0].Result == "" {
		t.Fatalf("turn 1 %+v", turn)
	}

	_, err = api.GetTurn(context.Background(), nil, &GetTurnRequest{Name: "reviewer-1", N: 4})
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("turn 4: %v", err)
	}
}

func TestListTurnsNoVolume(t *testing.T) {
	api := NewTurnsAPI(t.TempDir(), nil)

	_, err := api.ListTurns(context.Background(), nil, &ListTurnsRequest{Name: "nobody-1"})
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("list: %v", err)
	}

	_, err = api.GetTurn(context.Background(), nil, &GetTurnRequest{Name: "nobody-1", N: 1})
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("get: %v", err)
	}

	_, err = api.ListTurns(context.Background(), nil, &ListTurnsRequest{Name: ".."})
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("..: %v", err)
	}
}

func TestListTurnsSkipsCorruptLines(t *testing.T) {
	root, dir := newVolumes(t)
	body := fixture(t, claudeFixture)
	lines := strings.SplitAfter(body, "\n")
	// A torn write in the middle of the first turn, and one at the end.
	corrupt := strings.Join(lines[:4], "") + `{"type":"assistant","message":{"id":` + "\n" + strings.Join(lines[4:], "") + `{"type":"us`
	putFile(t, dir, ".claude/projects/-home-agent-project/"+claudeSession+".jsonl", corrupt)

	list, err := NewTurnsAPI(root, nil).ListTurns(context.Background(), nil, &ListTurnsRequest{Name: "reviewer-1"})
	if err != nil {
		t.Fatal(err)
	}
	if list.Skipped != 2 || len(list.Turns) != 3 || list.Turns[2].ToolCalls != 1 {
		t.Fatalf("skipped %d turns %+v", list.Skipped, list.Turns)
	}
}

func TestTurnsThreadLabels(t *testing.T) {
	root, dir := newVolumes(t)
	putFile(t, dir, SessionsFile, `{"#reviewer-1:0190a7c2-root":"`+claudeSession+`","main":"`+codexSession+`","odd":42}`)

	list, err := NewTurnsAPI(root, nil).ListTurns(context.Background(), nil, &ListTurnsRequest{Name: "reviewer-1"})
	if err != nil {
		t.Fatal(err)
	}

	codex := list.Turns[0]
	claude := list.Turns[1]
	if codex.Channel != "" || codex.Thread != "main" {
		t.Fatalf("codex label %q %q", codex.Channel, codex.Thread)
	}
	if claude.Channel != "#reviewer-1" || claude.Thread != "0190a7c2-root" {
		t.Fatalf("claude label %q %q", claude.Channel, claude.Thread)
	}
}

type fakeVolumes map[string]string

func (f fakeVolumes) VolumeDir(ctx context.Context, id uuid.UUID, name string) (string, error) {
	dir, ok := f[name]
	if !ok {
		return "", ez.New(ez.ENOTFOUND, "sandbox "+name+" not found", nil)
	}

	return dir, nil
}

func TestSandboxTurns(t *testing.T) {
	root, dir := newVolumes(t)
	api := NewTurnsAPI(root, fakeVolumes{"review-42": dir})

	list, err := api.ListSandboxTurns(context.Background(), nil, &ListSandboxTurnsRequest{Name: "review-42"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Turns) != 3 {
		t.Fatalf("turns %+v", list.Turns)
	}

	turn, err := api.GetSandboxTurn(context.Background(), nil, &GetSandboxTurnRequest{Name: "review-42", N: 3})
	if err != nil {
		t.Fatal(err)
	}
	if turn.Harness != "codex_cli" || turn.Answer != "hi" {
		t.Fatalf("turn %+v", turn)
	}

	_, err = api.ListSandboxTurns(context.Background(), nil, &ListSandboxTurnsRequest{Name: "gone"})
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("gone: %v", err)
	}
}
