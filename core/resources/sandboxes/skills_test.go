package sandboxes

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/task"
)

// writeSkill puts a skill with SKILL.md and a script beside it in dir.
func writeSkill(t *testing.T, dir string, name string, text string) {
	t.Helper()

	err := os.MkdirAll(filepath.Join(dir, name, "scripts"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(dir, name, "SKILL.md"), []byte(text), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(dir, name, "scripts", "run.sh"), []byte("echo hi\n"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
}

func skillsEnv(t *testing.T) (*testEnv, string) {
	t.Helper()

	env := newTestEnv(t)
	dir := t.TempDir()
	writeSkill(t, dir, "rtsdesign", "---\nname: rtsdesign\n---\nDesign.\n")
	env.api.SetSkillsDir(dir)

	return env, dir
}

func indexOf(calls []string, prefix string) int {
	for i, call := range calls {
		if strings.HasPrefix(call, prefix) {
			return i
		}
	}

	return -1
}

func checkSkillWritten(t *testing.T, env *testEnv, text string) {
	t.Helper()

	for _, dir := range []string{"/home/agent/.claude/skills/rtsdesign", "/home/agent/.codex/skills/rtsdesign"} {
		got := env.rt.files[dir+"/SKILL.md"]
		if got != text {
			t.Fatalf("%s/SKILL.md = %q, want %q", dir, got, text)
		}
		if env.rt.files[dir+"/scripts/run.sh"] != "echo hi\n" {
			t.Fatalf("%s/scripts/run.sh not written: %v", dir, env.rt.files)
		}
	}
}

func TestAllocateWritesSkills(t *testing.T) {
	env, _ := skillsEnv(t)
	ctx := context.Background()
	insertTask(t, env.db, "review-42")

	request := testSandbox()
	request.Skills = []string{"rtsdesign"}

	_, err := env.api.Allocate(ctx, nil, &AllocateRequest{TaskName: "review-42", Sandbox: request})
	if err != nil {
		t.Fatal(err)
	}
	env.api.Wait()

	checkSkillWritten(t, env, "---\nname: rtsdesign\n---\nDesign.\n")

	calls := env.rt.Calls()
	config := indexOf(calls, "run bash -c set -eu\ndir=")
	remove := indexOf(calls, "run rm -rf /home/agent/.claude/skills/rtsdesign /home/agent/.codex/skills/rtsdesign")
	skill := indexOf(calls, "write /home/agent/.claude/skills/rtsdesign/SKILL.md")
	chmod := indexOf(calls, "run chmod +x /home/agent/.claude/skills/rtsdesign/scripts/run.sh /home/agent/.codex/skills/rtsdesign/scripts/run.sh")
	brief := indexOf(calls, "write "+BriefPath)
	if config < 0 || remove < 0 || skill < 0 || chmod < 0 || brief < 0 || !(config < remove && remove < skill && skill < chmod && chmod < brief) {
		t.Fatalf("skills not written between the agent config and TASK.md: %q", calls)
	}
}

func TestAllocateHomeWritesSkills(t *testing.T) {
	env, _ := skillsEnv(t)
	ctx := context.Background()

	home := task.Sandbox{Spec: task.Spec{Skills: []string{"rtsdesign"}}}

	_, err := env.api.AllocateHome(ctx, HomeRequest{Agent: "reviewer-1", Sandbox: home, Command: "agc agent run --agent reviewer-1"})
	if err != nil {
		t.Fatal(err)
	}
	env.api.Wait()

	checkSkillWritten(t, env, "---\nname: rtsdesign\n---\nDesign.\n")
	if indexOf(env.rt.Calls(), "write "+AgentBriefPath) < 0 {
		t.Fatalf("AGENT.md not written: %q", env.rt.Calls())
	}
}

func TestAllocateRefusesAnUnknownSkillBeforeBoot(t *testing.T) {
	env, _ := skillsEnv(t)
	ctx := context.Background()
	insertTask(t, env.db, "review-42")

	request := testSandbox()
	request.Skills = []string{"rtsdesign", "nope"}

	_, err := env.api.Allocate(ctx, nil, &AllocateRequest{TaskName: "review-42", Sandbox: request})
	if err == nil {
		t.Fatal("an unknown skill was accepted")
	}
	msg := ez.ErrorMessage(err)
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(msg, `"nope"`) || !strings.Contains(msg, "rtsdesign") {
		t.Fatalf("error = %s %q", ez.ErrorCode(err), msg)
	}

	_, err = env.api.AllocateHome(ctx, HomeRequest{Agent: "reviewer-1", Sandbox: task.Sandbox{Spec: task.Spec{Skills: []string{"nope"}}}})
	if err == nil || !strings.Contains(ez.ErrorMessage(err), `"nope"`) {
		t.Fatalf("home with an unknown skill: %v", err)
	}

	env.api.Wait()
	if len(env.rt.created) != 0 {
		t.Fatalf("a sandbox was created: %+v", env.rt.created)
	}
}

func TestAllocateRefusesAnInvalidSkillName(t *testing.T) {
	env, _ := skillsEnv(t)
	insertTask(t, env.db, "review-42")

	request := testSandbox()
	request.Skills = []string{"../etc"}

	_, err := env.api.Allocate(context.Background(), nil, &AllocateRequest{TaskName: "review-42", Sandbox: request})
	if err == nil || ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("err = %v", err)
	}
}

// A start writes the skills again, so an edit to the repository reaches
// a woken sandbox.
func TestStartRewritesSkills(t *testing.T) {
	env, dir := skillsEnv(t)
	ctx := context.Background()
	owner := insertTask(t, env.db, "review-42")

	record := insertBooted(t, env.db, owner, "review-42", time.Now().UTC())
	record.Spec = []byte(`{"skills":["rtsdesign"]}`)
	err := record.Update(ctx, env.db, "spec")
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.api.Stop(ctx, nil, &StopRequest{Name: "review-42"})
	if err != nil {
		t.Fatal(err)
	}

	writeSkill(t, dir, "rtsdesign", "edited\n")

	_, err = env.api.Start(ctx, nil, &StartRequest{Name: "review-42"})
	if err != nil {
		t.Fatal(err)
	}

	checkSkillWritten(t, env, "edited\n")
}
