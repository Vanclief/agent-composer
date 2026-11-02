package sandboxes

import (
	"context"
	"slices"
	"testing"

	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

func TestInstructions(t *testing.T) {
	cases := []struct {
		name  string
		owner taskmodels.Task
		spec  task.Spec
		want  string
	}{
		{
			name: "harness with model and repo",
			owner: taskmodels.Task{
				Title: "Fix login", Name: "fix-login", Brief: "Make 2FA work.\n",
				Accept:   []string{"2FA codes from the app log in", "`go test ./...` passes"},
				Assignee: "claude_code", Model: "claude-opus-5-5", Manager: "orchestrator", Owner: "franco",
			},
			spec: task.Spec{Repo: "github.com/vanclief/agent-composer"},
			want: "# Fix login\n" +
				"\n" +
				"Make 2FA work.\n" +
				"\n" +
				"## Accepted when\n" +
				"\n" +
				"- 2FA codes from the app log in\n" +
				"- `go test ./...` passes\n" +
				"\n" +
				"## How this task works\n" +
				"\n" +
				"You are claude_code (claude-opus-5-5) working for the task `fix-login` inside a sandbox.\n" +
				"Your manager is orchestrator; the owner is franco.\n" +
				"The repository is cloned at /home/agent/agent-composer.\n" +
				"Work counts only once it leaves this sandbox:\n" +
				"\n" +
				"- Commit on a branch named `agc/fix-login` and push it.\n" +
				"- Post what you changed and what you verified in `#fix-login` with\n" +
				"  `agc msg post '#fix-login' <text>`; your manager reads that channel and\n" +
				"  answers there. Ask your manager there too, with\n" +
				"  `agc msg ask '#fix-login' <question>` (or start your answer with\n" +
				"  `QUESTION:`) so the question shows up as waiting on them, then keep\n" +
				"  working on what does not depend on the answer.\n" +
				"- If something needs a decision only franco can make, ask it in\n" +
				"  `#franco` (their inbox) with `agc msg ask '#franco' <question>` so it\n" +
				"  shows up as waiting on them, and say so in `#fix-login`; do not wait idle.\n" +
				"- Do not modify files outside the repository, and never print secret values.\n" +
				"- When the work is finished, deliver it as \"Delivering\" below says. The\n" +
				"  task is done only when your manager or franco accepts it; a rejection comes\n" +
				"  back in `#fix-login` with the reason.\n" +
				"\n" +
				"## Delivering\n" +
				"\n" +
				"Write `report.json` (do not commit it) with exactly these fields:\n" +
				"\n" +
				"```json\n" +
				"{\n" +
				"  \"criteria\": [ { \"met\": \"yes\", \"note\": \"how you know\" } ],\n" +
				"  \"changes\":  { \"branch\": \"agc/fix-login\", \"sha\": \"<pushed commit>\", \"files\": 12 },\n" +
				"  \"checks\":   [ { \"name\": \"go test ./...\", \"passed\": true, \"note\": \"\" } ],\n" +
				"  \"left_out\": [ { \"what\": \"what you did not do\", \"why\": \"why\" } ],\n" +
				"  \"needs\":    [ \"what only the reviewer can decide\" ]\n" +
				"}\n" +
				"```\n" +
				"\n" +
				"- `criteria`: exactly 2 entries, one per \"Accepted when\" line, in order; `met` is `yes`,\n" +
				"  `no` or `partly`, `note` at most 500 characters.\n" +
				"- `changes`: the branch you pushed and its commit (7 to 40 hex characters).\n" +
				"- `checks`: what you ran and whether it passed.\n" +
				"- `left_out`: what you did not do and why; `needs`: what you need the\n" +
				"  reviewer to decide. `[]` when there is nothing.\n" +
				"\n" +
				"Then run `agc task deliver \"<one or two sentences>\" --report report.json`.\n" +
				"The summary is what the reviewer reads first; the report is what they\n" +
				"judge. A report that breaks these rules is refused with the reason: fix it\n" +
				"and deliver again. Only when the report cannot be written, start your\n" +
				"answer with `DONE:` followed by the summary instead; that delivers the\n" +
				"summary alone.\n",
		},
		{
			name: "person without repo",
			owner: taskmodels.Task{
				Title: "Call the bank", Name: "call-the-bank", Brief: "Ask about the fee.",
				Assignee: "ana", Manager: "franco", Owner: "franco",
			},
			want: "# Call the bank\n" +
				"\n" +
				"Ask about the fee.\n" +
				"\n" +
				"## How this task works\n" +
				"\n" +
				"You are ana working for the task `call-the-bank` inside a sandbox.\n" +
				"Your manager is franco; the owner is franco.\n" +
				"There is no repository.\n" +
				"Work counts only once it leaves this sandbox:\n" +
				"\n" +
				"- Commit on a branch named `agc/call-the-bank` and push it.\n" +
				"- Post what you changed and what you verified in `#call-the-bank` with\n" +
				"  `agc msg post '#call-the-bank' <text>`; your manager reads that channel and\n" +
				"  answers there. Ask your manager there too, with\n" +
				"  `agc msg ask '#call-the-bank' <question>` (or start your answer with\n" +
				"  `QUESTION:`) so the question shows up as waiting on them, then keep\n" +
				"  working on what does not depend on the answer.\n" +
				"- If something needs a decision only franco can make, ask it in\n" +
				"  `#franco` (their inbox) with `agc msg ask '#franco' <question>` so it\n" +
				"  shows up as waiting on them, and say so in `#call-the-bank`; do not wait idle.\n" +
				"- Do not modify files outside the repository, and never print secret values.\n" +
				"- When the work is finished, deliver it as \"Delivering\" below says. The\n" +
				"  task is done only when your manager accepts it; a rejection comes\n" +
				"  back in `#call-the-bank` with the reason.\n" +
				"\n" +
				"## Delivering\n" +
				"\n" +
				"Write `report.json` (do not commit it) with exactly these fields:\n" +
				"\n" +
				"```json\n" +
				"{\n" +
				"  \"criteria\": [ { \"met\": \"yes\", \"note\": \"how you know\" } ],\n" +
				"  \"changes\":  { \"branch\": \"agc/call-the-bank\", \"sha\": \"<pushed commit>\", \"files\": 12 },\n" +
				"  \"checks\":   [ { \"name\": \"go test ./...\", \"passed\": true, \"note\": \"\" } ],\n" +
				"  \"left_out\": [ { \"what\": \"what you did not do\", \"why\": \"why\" } ],\n" +
				"  \"needs\":    [ \"what only the reviewer can decide\" ]\n" +
				"}\n" +
				"```\n" +
				"\n" +
				"- `criteria`: `[]`, as this task has no \"Accepted when\" lines; `met` is `yes`,\n" +
				"  `no` or `partly`, `note` at most 500 characters.\n" +
				"- `changes`: the branch you pushed and its commit (7 to 40 hex characters).\n" +
				"- `checks`: what you ran and whether it passed.\n" +
				"- `left_out`: what you did not do and why; `needs`: what you need the\n" +
				"  reviewer to decide. `[]` when there is nothing.\n" +
				"\n" +
				"Then run `agc task deliver \"<one or two sentences>\" --report report.json`.\n" +
				"The summary is what the reviewer reads first; the report is what they\n" +
				"judge. A report that breaks these rules is refused with the reason: fix it\n" +
				"and deliver again. Only when the report cannot be written, start your\n" +
				"answer with `DONE:` followed by the summary instead; that delivers the\n" +
				"summary alone.\n",
		},
		{
			name: "no owner",
			owner: taskmodels.Task{
				Title: "Bump deps", Name: "bump-deps", Brief: "Update go.mod.",
				Assignee: "codex_cli", Model: "gpt-5.5", Manager: "orchestrator",
			},
			spec: task.Spec{Repo: "https://github.com/o/n.git"},
			want: "# Bump deps\n" +
				"\n" +
				"Update go.mod.\n" +
				"\n" +
				"## How this task works\n" +
				"\n" +
				"You are codex_cli (gpt-5.5) working for the task `bump-deps` inside a sandbox.\n" +
				"Your manager is orchestrator; no owner is set.\n" +
				"The repository is cloned at /home/agent/n.\n" +
				"Work counts only once it leaves this sandbox:\n" +
				"\n" +
				"- Commit on a branch named `agc/bump-deps` and push it.\n" +
				"- Post what you changed and what you verified in `#bump-deps` with\n" +
				"  `agc msg post '#bump-deps' <text>`; your manager reads that channel and\n" +
				"  answers there. Ask your manager there too, with\n" +
				"  `agc msg ask '#bump-deps' <question>` (or start your answer with\n" +
				"  `QUESTION:`) so the question shows up as waiting on them, then keep\n" +
				"  working on what does not depend on the answer.\n" +
				"- Do not modify files outside the repository, and never print secret values.\n" +
				"- When the work is finished, deliver it as \"Delivering\" below says. The\n" +
				"  task is done only when your manager accepts it; a rejection comes\n" +
				"  back in `#bump-deps` with the reason.\n" +
				"\n" +
				"## Delivering\n" +
				"\n" +
				"Write `report.json` (do not commit it) with exactly these fields:\n" +
				"\n" +
				"```json\n" +
				"{\n" +
				"  \"criteria\": [ { \"met\": \"yes\", \"note\": \"how you know\" } ],\n" +
				"  \"changes\":  { \"branch\": \"agc/bump-deps\", \"sha\": \"<pushed commit>\", \"files\": 12 },\n" +
				"  \"checks\":   [ { \"name\": \"go test ./...\", \"passed\": true, \"note\": \"\" } ],\n" +
				"  \"left_out\": [ { \"what\": \"what you did not do\", \"why\": \"why\" } ],\n" +
				"  \"needs\":    [ \"what only the reviewer can decide\" ]\n" +
				"}\n" +
				"```\n" +
				"\n" +
				"- `criteria`: `[]`, as this task has no \"Accepted when\" lines; `met` is `yes`,\n" +
				"  `no` or `partly`, `note` at most 500 characters.\n" +
				"- `changes`: the branch you pushed and its commit (7 to 40 hex characters).\n" +
				"- `checks`: what you ran and whether it passed.\n" +
				"- `left_out`: what you did not do and why; `needs`: what you need the\n" +
				"  reviewer to decide. `[]` when there is nothing.\n" +
				"\n" +
				"Then run `agc task deliver \"<one or two sentences>\" --report report.json`.\n" +
				"The summary is what the reviewer reads first; the report is what they\n" +
				"judge. A report that breaks these rules is refused with the reason: fix it\n" +
				"and deliver again. Only when the report cannot be written, start your\n" +
				"answer with `DONE:` followed by the summary instead; that delivers the\n" +
				"summary alone.\n",
		},
	}

	for _, tc := range cases {
		got := Instructions(&tc.owner, tc.spec)
		if got != tc.want {
			t.Fatalf("%s:\n--- got\n%s\n--- want\n%s", tc.name, got, tc.want)
		}
	}
}

func TestAgentCommand(t *testing.T) {
	cases := []struct {
		owner taskmodels.Task
		want  string
	}{
		{taskmodels.Task{Name: "fix-login", Assignee: "claude_code", Model: "claude-opus-5-5", Manager: "orchestrator", Owner: "franco"}, "/usr/local/bin/agc agent run --harness claude_code --model claude-opus-5-5 --task fix-login --manager orchestrator --owner franco --brief /home/agent/TASK.md"},
		{taskmodels.Task{Name: "bump", Assignee: "codex_cli"}, "/usr/local/bin/agc agent run --harness codex_cli --task bump --brief /home/agent/TASK.md"},
		{taskmodels.Task{Name: "call", Assignee: "franco"}, ""},
	}

	for _, tc := range cases {
		got := AgentCommand(&tc.owner)
		if got != tc.want {
			t.Fatalf("AgentCommand(%+v) = %q, want %q", tc.owner, got, tc.want)
		}
	}
}

func TestAllocateStoresInstructionsAndRunsTheAgent(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	owner := &taskmodels.Task{Title: "Fix login", Name: "fix-login", Brief: "Make 2FA work.", Assignee: "claude_code", Model: "claude-opus-5-5", Manager: "orchestrator", Owner: "franco"}
	err := owner.Insert(ctx, env.db)
	if err != nil {
		t.Fatal(err)
	}

	request := task.Sandbox{Spec: task.Spec{Repo: "github.com/o/n"}}

	view, err := env.api.AllocateFor(ctx, owner, request)
	if err != nil {
		t.Fatal(err)
	}

	want := Instructions(owner, request.Spec)
	if view.Instructions != want {
		t.Fatalf("view instructions:\n%s", view.Instructions)
	}
	if view.Command != AgentCommand(owner) {
		t.Fatalf("command = %q", view.Command)
	}

	env.api.Wait()

	if env.rt.files[BriefPath] != want {
		t.Fatalf("TASK.md written:\n%s", env.rt.files[BriefPath])
	}

	calls := env.rt.Calls()
	wantCall := "terminal /usr/local/bin/agc agent run --harness claude_code --model claude-opus-5-5 --task fix-login --manager orchestrator --owner franco --brief /home/agent/TASK.md in /home/agent/n"
	if !slices.Contains(calls, wantCall) {
		t.Fatalf("calls = %q", calls)
	}

	created := env.rt.created[0]
	if created.Env["AGC_SANDBOX"] != "fix-login" {
		t.Fatalf("env = %v", created.Env)
	}
}

func TestGrantsAddTheOwnerInbox(t *testing.T) {
	spec := task.Spec{Channels: []string{"#releases", "#franco:read"}}

	grants, err := Grants(&taskmodels.Task{Name: "fix-login", Owner: "franco"}, spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 2 || grants[1].Channel != "#franco" || grants[1].ReadOnly {
		t.Fatalf("grants = %+v", grants)
	}

	grants, err = Grants(&taskmodels.Task{Name: "fix-login", Owner: "ana"}, task.Spec{})
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 1 || grants[0].Channel != "#ana" || grants[0].ReadOnly {
		t.Fatalf("grants = %+v", grants)
	}

	grants, err = Grants(&taskmodels.Task{Name: "fix-login"}, task.Spec{})
	if err != nil || len(grants) != 0 {
		t.Fatalf("grants without an owner = %+v, err %v", grants, err)
	}
}
