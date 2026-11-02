package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func review(t *testing.T, set *Set, id string) Review {
	t.Helper()

	found, err := set.Review(id)
	if err != nil {
		t.Fatalf("Review(%s): %v", id, err)
	}

	return found
}

func TestLoadProposalsAndGates(t *testing.T) {
	set := loadGood(t)

	if len(set.Proposals) != 10 || len(set.Gates) != 3 {
		t.Fatalf("loaded %d proposals, %d gates", len(set.Proposals), len(set.Gates))
	}

	first := set.Proposals[0]
	if first.ID != "proposed-rule" || first.Proposes != "rule" || first.By != "codex_cli" || !first.Open() || first.File != filepath.Join("testdata", "good", "proposals.yaml") {
		t.Fatalf("proposal = %+v", first)
	}

	accepted := set.Proposals[7]
	if accepted.Open() || accepted.Accepted == nil || accepted.Accepted.Who != "franco" || !slices.Equal(accepted.Evidence, []string{"seen-b"}) {
		t.Fatalf("accepted = %+v", accepted)
	}

	rejected := set.Proposals[8]
	if rejected.Rejected == nil || rejected.Rejected.Why != "X still pays for itself." {
		t.Fatalf("rejected = %+v", rejected)
	}

	gate := set.Gates[0]
	if gate.ID != "agent-beliefs-observed" || gate.Tier != Observe || gate.Sample != 2 || !slices.Equal(gate.By, []string{"codex_cli"}) || gate.File != filepath.Join("testdata", "good", "gates.yaml") {
		t.Fatalf("gate = %+v", gate)
	}

	kind, found := set.Kind("proposed-rule")
	if !found || kind != "proposal" {
		t.Fatalf("Kind(proposed-rule) = %q, %v", kind, found)
	}

	node, err := set.WhyAny("proposed-rule")
	if err != nil || node.Kind != "proposal" || len(node.Because) != 1 || node.Because[0].ID != "top" {
		t.Fatalf("WhyAny(proposed-rule) = %+v, %v", node, err)
	}
}

func TestProposalRejects(t *testing.T) {
	good := `evidence:
  - id: seen
    evidence: It was seen.
    who: franco
    when: 2026-11-01
`
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{"goal", "proposals:\n  - id: p\n    proposes: goal\n    proposal: Win.\n    by: codex_cli\n    when: 2026-11-01\n", "a goal is never proposed"},
		{"kind", "proposals:\n  - id: p\n    proposes: wish\n    proposal: W.\n    by: codex_cli\n    when: 2026-11-01\n", "proposes: must be one of"},
		{"text", "proposals:\n  - id: p\n    proposes: rule\n    by: codex_cli\n    when: 2026-11-01\n", "proposal: text is required"},
		{"by", "proposals:\n  - id: p\n    proposes: rule\n    proposal: W.\n    when: 2026-11-01\n", "needs by and when"},
		{"both", "proposals:\n  - id: p\n    proposes: rule\n    proposal: W.\n    by: codex_cli\n    when: 2026-11-01\n    accepted: { who: franco, when: 2026-11-02 }\n    rejected: { who: franco, when: 2026-11-02, why: No. }\n", "both accepted and rejected"},
		{"why", "proposals:\n  - id: p\n    proposes: rule\n    proposal: W.\n    by: codex_cli\n    when: 2026-11-01\n    rejected: { who: franco, when: 2026-11-02 }\n", "rejected: needs who, when and why"},
		{"became-open", "proposals:\n  - id: p\n    proposes: rule\n    proposal: W.\n    by: codex_cli\n    when: 2026-11-01\n    became: seen\n", "it is not accepted"},
		{"became-missing", "proposals:\n  - id: p\n    proposes: rule\n    proposal: W.\n    by: codex_cli\n    when: 2026-11-01\n    accepted: { who: franco, when: 2026-11-02 }\n    became: nowhere\n", "became names nowhere"},
		{"because", "proposals:\n  - id: p\n    proposes: rule\n    proposal: W.\n    because: [nothing]\n    by: codex_cli\n    when: 2026-11-01\n", "because names nothing"},
		{"evidence", "proposals:\n  - id: p\n    proposes: rule\n    proposal: W.\n    evidence: [unseen]\n    by: codex_cli\n    when: 2026-11-01\n", "evidence names unseen"},
		{"gate-on-gates", "gates:\n  - id: g\n    proposes: gate\n    tier: observe\n", "a gate change always needs a human"},
		{"gate-kind", "gates:\n  - id: g\n    proposes: wish\n    tier: observe\n", "proposes: must be one of"},
		{"gate-tier", "gates:\n  - id: g\n    proposes: rule\n    tier: maybe\n", "tier: must be decide, observe or delegated"},
		{"gate-sample", "gates:\n  - id: g\n    proposes: rule\n    tier: decide\n    sample: 3\n", "only an observe gate audits"},
		{"gate-by", "gates:\n  - id: g\n    proposes: rule\n    tier: decide\n    by: [\"\"]\n", "by: has an empty name"},
		{"gate-because", "gates:\n  - id: g\n    proposes: rule\n    tier: decide\n    because: [nothing]\n", "because names nothing"},
		{"two-kinds", "proposals: []\ngates: []\n", "exactly one of"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			err := os.WriteFile(filepath.Join(dir, "evidence.yaml"), []byte(good), 0o644)
			if err != nil {
				t.Fatal(err)
			}
			err = os.WriteFile(filepath.Join(dir, "under-test.yaml"), []byte(tc.yaml), 0o644)
			if err != nil {
				t.Fatal(err)
			}

			_, err = Load(dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load: %v, want %q", err, tc.want)
			}
		})
	}
}

func TestReviewTiers(t *testing.T) {
	set := loadGood(t)

	// A rule covered by a decide gate on anyone.
	rule := review(t, set, "proposed-rule")
	if rule.Gate != "any-rule-decided" || rule.Tier != Decide || rule.Written != Decide || rule.Illegible || rule.Status.Label != Grounded {
		t.Fatalf("proposed-rule = %+v", rule)
	}

	// Beliefs by codex_cli are observed, about one in two audited. The
	// sample is by hash of the id, so which one is audited is fixed.
	belief := review(t, set, "proposed-belief")
	audited := review(t, set, "proposed-belief-audited")
	if belief.Gate != "agent-beliefs-observed" || audited.Gate != "agent-beliefs-observed" {
		t.Fatalf("gates = %s, %s", belief.Gate, audited.Gate)
	}
	if belief.Audited == audited.Audited {
		t.Fatalf("both audited = %v; the sample of two should split them", belief.Audited)
	}
	for _, r := range []Review{belief, audited} {
		want := Observe
		if r.Audited {
			want = Decide
		}
		if r.Tier != want || r.Written != Observe {
			t.Fatalf("%s tier = %s, written %s, audited %v", r.Proposal.ID, r.Tier, r.Written, r.Audited)
		}
	}
	if audited.Status.Label != Opinion || audited.Illegible {
		t.Fatalf("a belief resting on nothing is an opinion, not illegible: %+v", audited)
	}

	// A belief by a person is not covered by the agents' gate.
	ana := review(t, set, "proposed-by-ana")
	if ana.Gate != "" || ana.Tier != Decide {
		t.Fatalf("proposed-by-ana = %+v", ana)
	}

	// Code is delegated to codex_cli since 2026-10-01, and the rejection
	// before that does not tighten it; but this one rests on nothing.
	code := review(t, set, "proposed-code")
	if code.Gate != "code-delegated" || code.Written != Delegated || !code.Illegible || code.Tier != Decide || len(code.TightenedBy) != 0 {
		t.Fatalf("proposed-code = %+v", code)
	}
	old := review(t, set, "rejected-old-code")
	if old.Tier != Delegated || old.Illegible {
		t.Fatalf("rejected-old-code = %+v", old)
	}

	// A gate change is covered by nothing.
	gate := review(t, set, "proposed-gate")
	if gate.Gate != "" || gate.Tier != Decide || gate.Illegible {
		t.Fatalf("proposed-gate = %+v", gate)
	}

	// An action resting on a doubted decision says so.
	action := review(t, set, "proposed-action")
	if action.Status.Label != InDoubt || !slices.Equal(action.Status.Doubted, []string{"retracted-only"}) {
		t.Fatalf("proposed-action = %+v", action)
	}

	_, err := set.Review("nothing")
	if err == nil || !strings.Contains(err.Error(), "proposal nothing not found") {
		t.Fatalf("Review(nothing): %v", err)
	}
}

func TestTightenedByRejection(t *testing.T) {
	files := map[string]string{
		"beliefs.yaml": "beliefs:\n  - id: b\n    belief: B.\n",
		"gates.yaml":   "gates:\n  - id: g\n    proposes: belief\n    by: [codex_cli]\n    tier: observe\n    since: 2026-11-01\n  - id: g-strict\n    proposes: belief\n    tier: decide\n    by: [pi]\n",
		"proposals.yaml": `proposals:
  - id: open-one
    proposes: belief
    proposal: One.
    by: codex_cli
    when: 2026-11-02
  - id: old-rejection
    proposes: belief
    proposal: Old.
    by: codex_cli
    when: 2026-10-01
    rejected: { who: franco, when: 2026-10-02, why: Not so. }
  - id: new-rejection
    proposes: belief
    proposal: New.
    by: codex_cli
    when: 2026-11-02
    rejected: { who: franco, when: 2026-11-03, why: Nor this. }
  - id: by-pi
    proposes: belief
    proposal: Pi.
    by: pi
    when: 2026-11-02
`,
	}
	dir := t.TempDir()
	for name, content := range files {
		err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}
	set := load(t, dir)

	open := review(t, set, "open-one")
	if open.Tier != Decide || open.Written != Observe || !slices.Equal(open.TightenedBy, []string{"new-rejection"}) {
		t.Fatalf("open-one = %+v", open)
	}

	// pi is covered by a decide gate only; the observe gate names codex_cli.
	pi := review(t, set, "by-pi")
	if pi.Gate != "g-strict" || pi.Tier != Decide || len(pi.TightenedBy) != 0 {
		t.Fatalf("by-pi = %+v", pi)
	}

	reports := set.GateReports()
	if len(reports) != 3 {
		t.Fatalf("reports = %d", len(reports))
	}
	g := reports[0]
	if g.Gate.ID != "g" || g.Tier != Decide || !slices.Equal(g.TightenedBy, []string{"new-rejection"}) || g.Open != 1 || g.Rejected != 2 || g.Accepted != 0 {
		t.Fatalf("report g = %+v", g)
	}
	if reports[1].Gate.ID != "g-strict" || reports[1].Open != 1 || reports[1].Tier != Decide {
		t.Fatalf("report g-strict = %+v", reports[1])
	}
	if reports[2].Gate.ID != "" || reports[2].Gate.Tier != Decide || reports[2].Tier != Decide || reports[2].Open != 0 {
		t.Fatalf("default report = %+v", reports[2])
	}
}

func TestGateReportsAndRubberStamp(t *testing.T) {
	set := loadGood(t)
	reports := set.GateReports()
	if len(reports) != 4 {
		t.Fatalf("reports = %d", len(reports))
	}

	byID := map[string]GateReport{}
	for _, report := range reports {
		byID[report.Gate.ID] = report
	}

	beliefs := byID["agent-beliefs-observed"]
	if beliefs.Open != 2 || beliefs.Audited != 1 || beliefs.Tier != Observe || beliefs.Status.Label != Grounded {
		t.Fatalf("agent-beliefs-observed = %+v", beliefs)
	}
	code := byID["code-delegated"]
	if code.Open != 1 || code.Rejected != 1 || code.Tier != Delegated || len(code.TightenedBy) != 0 {
		t.Fatalf("code-delegated = %+v", code)
	}
	rule := byID["any-rule-decided"]
	if rule.Open != 1 || rule.Status.Label != Opinion || rule.AllAccepted {
		t.Fatalf("any-rule-decided = %+v", rule)
	}
	// The default covers ana's belief, the gate change, the action, the
	// accepted evidence and the rejected decision.
	def := byID[""]
	if def.Open != 3 || def.Accepted != 1 || def.Rejected != 1 || def.AllAccepted {
		t.Fatalf("default = %+v", def)
	}

	// Five accepted and none rejected on a decide gate is a rubber stamp.
	stamped := "gates:\n  - id: g\n    proposes: rule\n    tier: decide\n"
	proposals := "proposals:\n"
	for i := 0; i < rubberStamp; i++ {
		proposals += "  - id: p" + string(rune('a'+i)) + "\n    proposes: rule\n    proposal: R.\n    by: codex_cli\n    when: 2026-11-01\n    accepted: { who: franco, when: 2026-11-02 }\n"
	}
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "gates.yaml"), []byte(stamped), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(dir, "proposals.yaml"), []byte(proposals), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	report := load(t, dir).GateReports()[0]
	if !report.AllAccepted || report.Accepted != rubberStamp {
		t.Fatalf("stamped = %+v", report)
	}
}

func TestProposalAndGateLines(t *testing.T) {
	set := loadGood(t)

	cases := map[string]Line{
		"proposed-rule": {
			Text:    "codex_cli proposes the rule that never do W because D follows from C, and it needs my decision.",
			Trailer: "(proposed-rule ← top · decide via any-rule-decided · proposed by codex_cli on 2026-11-02)",
		},
		"proposed-by-ana": {
			Text:    "ana proposes the belief that L is so, with no evidence under it, and it needs my decision.",
			Trailer: "(proposed-by-ana · decide by default · proposed by ana on 2026-11-02)",
		},
		"proposed-code": {
			Text:    "codex_cli proposes the code change to add W to the build, but it rests on nothing I hold, so there is no why to read, and it needs my decision.",
			Trailer: "(proposed-code · decide via code-delegated, written delegated · proposed by codex_cli on 2026-11-02 · change: codex/add-w)",
		},
		"proposed-gate": {
			Text:    "codex_cli proposes the gate change to let codex_cli land rules because we chose X because of C, and it needs my decision.",
			Trailer: "(proposed-gate ← choice · decide by default · proposed by codex_cli on 2026-11-02)",
		},
		"proposed-action": {
			Text:    "codex_cli proposes the action to build W because we chose Z because G is there — though I now doubt that G is there, and it needs my decision.",
			Trailer: "(proposed-action ← doubted-choice · decide by default · proposed by codex_cli on 2026-11-02)",
		},
		"accepted-evidence": {
			Text:    "I accepted the evidence that W was seen in the logs, proposed by codex_cli, on 2026-11-02.",
			Trailer: "(accepted-evidence ← seen-b · decide via agent-evidence-needs-a-human · proposed by codex_cli on 2026-11-01 · accepted by franco)",
		},
		"rejected-decision": {
			Text:    "I rejected the decision that drop X, proposed by codex_cli, on 2026-10-02: X still pays for itself.",
			Trailer: "(rejected-decision ← derived · decide by default · proposed by codex_cli on 2026-10-01 · rejected by franco)",
		},
	}
	// The good fixture has no evidence gate; the accepted evidence falls to the default.
	cases["accepted-evidence"] = Line{
		Text:    "I accepted the evidence that W was seen in the logs, proposed by codex_cli, on 2026-11-02.",
		Trailer: "(accepted-evidence ← seen-b · decide by default · proposed by codex_cli on 2026-11-01 · accepted by franco)",
	}

	for id, want := range cases {
		got := set.ProposalLine(review(t, set, id))
		if got != want {
			t.Fatalf("%s =\n%+v\nwant\n%+v", id, got, want)
		}
	}

	belief := review(t, set, "proposed-belief")
	audited := review(t, set, "proposed-belief-audited")
	observed := belief
	sampled := audited
	if belief.Audited {
		observed = audited
		sampled = belief
	}
	if !strings.HasSuffix(set.ProposalLine(observed).Text, ", and it lands for me to see.") || !strings.Contains(set.ProposalLine(audited).Text+set.ProposalLine(observed).Text, "K might be so, with no evidence under it, and") {
		t.Fatalf("observed = %q", set.ProposalLine(observed).Text)
	}
	if !strings.HasSuffix(set.ProposalLine(sampled).Text, ", and it is audited, so it needs my decision.") {
		t.Fatalf("audited = %q", set.ProposalLine(sampled).Text)
	}

	reports := set.GateReports()
	wantGates := []Line{
		{
			Text:    "Beliefs proposed by codex_cli land for me to see, about one in 2 audited, because D follows from C. So far 0 accepted, 0 rejected and 2 open, 1 audited.",
			Trailer: "(agent-beliefs-observed · belief by codex_cli · observe · sample 1/2 ← top)",
		},
		{
			Text:    "Code changes proposed by codex_cli land without me since 2026-10-01. So far 0 accepted, 1 rejected and 1 open.",
			Trailer: "(code-delegated · code by codex_cli · delegated · since 2026-10-01)",
		},
		{
			Text:    "Rules proposed by anyone need my decision before they land because F is true, though nobody saw it — though I only think that F is true, though nobody saw it. So far 0 accepted, 0 rejected and 1 open.",
			Trailer: "(any-rule-decided · rule by anyone · decide ← hunch)",
		},
		{
			Text:    "Anything no gate covers needs my decision. So far 1 accepted, 1 rejected and 3 open.",
			Trailer: "(default · decide)",
		},
	}
	for i, want := range wantGates {
		got := set.GateLine(reports[i])
		if got != want {
			t.Fatalf("gate %d =\n%+v\nwant\n%+v", i, got, want)
		}
	}
}

func TestTightenedLines(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"gates.yaml": "gates:\n  - id: g\n    proposes: belief\n    tier: observe\n",
		"proposals.yaml": `proposals:
  - id: open-one
    proposes: belief
    proposal: One.
    by: codex_cli
    when: 2026-11-02
  - id: no
    proposes: belief
    proposal: Nope.
    by: codex_cli
    when: 2026-11-02
    rejected: { who: franco, when: 2026-11-03, why: Not so. }
`,
	}
	for name, content := range files {
		err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}
	set := load(t, dir)

	got := set.ProposalLine(review(t, set, "open-one"))
	want := Line{
		Text:    "codex_cli proposes the belief that one, with no evidence under it, and it needs my decision, since I rejected one like it.",
		Trailer: "(open-one · decide via g, written observe · tightened by no · proposed by codex_cli on 2026-11-02)",
	}
	if got != want {
		t.Fatalf("tightened =\n%+v\nwant\n%+v", got, want)
	}

	got = set.GateLine(set.GateReports()[0])
	want = Line{
		Text:    "Beliefs proposed by anyone land for me to see, but need my decision now, since I rejected one of them. So far 0 accepted, 1 rejected and 1 open.",
		Trailer: "(g · belief by anyone · observe · tightened by no)",
	}
	if got != want {
		t.Fatalf("tightened gate =\n%+v\nwant\n%+v", got, want)
	}
}

func TestAcceptAndReject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "proposals.yaml")
	original := `# Kept as is.
proposals:
  # One to accept.
  - id: yes
    proposes: belief
    proposal: Yes.
    by: codex_cli
    when: 2026-11-02
  - id: no
    proposes: belief
    proposal: No.
    by: codex_cli
    when: 2026-11-02
  - id: later
    proposes: belief
    proposal: Later.
    by: codex_cli
    when: 2026-11-02
`
	err := os.WriteFile(path, []byte(original), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	err = Accept(path, "yes", "franco", "2026-11-03")
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	err = Reject(path, "no", "franco", "2026-11-03", "Not so.")
	if err != nil {
		t.Fatalf("Reject: %v", err)
	}

	set := load(t, path)
	yes, _ := set.proposal("yes")
	no, _ := set.proposal("no")
	later, _ := set.proposal("later")
	if yes.Accepted == nil || yes.Accepted.Who != "franco" || yes.Accepted.When != "2026-11-03" || yes.Rejected != nil {
		t.Fatalf("yes = %+v", yes)
	}
	if no.Rejected == nil || no.Rejected.Why != "Not so." || no.Accepted != nil {
		t.Fatalf("no = %+v", no)
	}
	if !later.Open() {
		t.Fatalf("later = %+v", later)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "# Kept as is.") || !strings.Contains(text, "# One to accept.") {
		t.Fatalf("comments lost:\n%s", text)
	}
	if !strings.Contains(text, `accepted: {who: franco, when: "2026-11-03"}`) {
		t.Fatalf("accepted not written in flow style:\n%s", text)
	}

	err = Accept(path, "yes", "franco", "2026-11-04")
	if err == nil || !strings.Contains(err.Error(), "already accepted") {
		t.Fatalf("second Accept: %v", err)
	}
	err = Accept(path, "no", "franco", "2026-11-04")
	if err == nil || !strings.Contains(err.Error(), "already rejected") {
		t.Fatalf("Accept after Reject: %v", err)
	}
	err = Reject(path, "later", "franco", "2026-11-04", "  ")
	if err == nil || !strings.Contains(err.Error(), "needs why") {
		t.Fatalf("Reject without why: %v", err)
	}
	err = Accept(path, "nothing", "franco", "2026-11-04")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Accept nothing: %v", err)
	}

	notProposals := filepath.Join(dir, "beliefs.yaml")
	err = os.WriteFile(notProposals, []byte("beliefs: []\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	err = Accept(notProposals, "yes", "franco", "2026-11-04")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Accept in a beliefs file: %v", err)
	}
}

// A gate may say proposes: task (ADR 0015): the main agent's creating,
// accepting and rejecting tasks. TierFor gives the tier such an act gets
// without a proposal written down; a rejected task proposal under an
// observe gate pulls it to decide like any other.
func TestTaskActsAreGated(t *testing.T) {
	files := map[string]string{
		"beliefs.yaml": "beliefs:\n  - id: b\n    belief: B.\n",
		"gates.yaml":   "gates:\n  - id: main-tasks-observed\n    proposes: task\n    by: [main]\n    tier: observe\n    since: 2026-11-02\n    because: [b]\n",
	}
	set := load(t, writeFiles(t, files))

	tier, gate := set.TierFor("task", "main")
	if tier != Observe || gate != "main-tasks-observed" {
		t.Fatalf("TierFor(task, main) = %s, %q", tier, gate)
	}

	tier, gate = set.TierFor("task", "codex_cli")
	if tier != Decide || gate != "" {
		t.Fatalf("TierFor(task, codex_cli) = %s, %q", tier, gate)
	}

	tier, gate = set.TierFor("gate", "main")
	if tier != Decide || gate != "" {
		t.Fatalf("TierFor(gate, main) = %s, %q", tier, gate)
	}

	files["proposals.yaml"] = `proposals:
  - id: task-ads-retry
    proposes: task
    proposal: Retry the ads job.
    because: [b]
    by: main
    when: 2026-11-02
    rejected: { who: franco, when: 2026-11-03, why: Not before I look. }
`
	set = load(t, writeFiles(t, files))

	tier, gate = set.TierFor("task", "main")
	if tier != Decide || gate != "main-tasks-observed" {
		t.Fatalf("TierFor after a rejection = %s, %q", tier, gate)
	}

	reviewed := review(t, set, "task-ads-retry")
	if reviewed.Gate != "main-tasks-observed" || reviewed.Tier != Decide || len(reviewed.TightenedBy) != 1 {
		t.Fatalf("review = %+v", reviewed)
	}

	line := set.ProposalLine(reviewed)
	if !strings.Contains(line.Text, "the task act to retry the ads job") {
		t.Fatalf("ProposalLine = %+v", line)
	}

	// A task act resting on nothing cannot be read, so it is decide.
	illegible := writeFiles(t, map[string]string{
		"gates.yaml":     "gates:\n  - id: g\n    proposes: task\n    tier: delegated\n",
		"proposals.yaml": "proposals:\n  - id: p\n    proposes: task\n    proposal: Do it.\n    by: main\n    when: 2026-11-02\n",
	})
	unread := review(t, load(t, illegible), "p")
	if !unread.Illegible || unread.Tier != Decide {
		t.Fatalf("illegible task act = %+v", unread)
	}
}

// main is an agent role a rule may trust, and never a witness.
func TestMainIsATrustedJudgeAndNotAWitness(t *testing.T) {
	rules := "subject: the main agent\nrules:\n  - id: r\n    rule: Stay quiet.\n    judge:\n      trusted: [main]\n"
	set := load(t, writeFiles(t, map[string]string{"rules.yaml": rules}))
	if JudgeNote(set.Rulesets[0].Rules[0]) != "needs main" || JudgeHarness("main") != "claude_code" || JudgeHarness("franco") != "" {
		t.Fatalf("note = %q", JudgeNote(set.Rulesets[0].Rules[0]))
	}

	_, err := Load(writeFiles(t, map[string]string{"e.yaml": "evidence:\n  - id: e\n    evidence: Saw it.\n    who: main\n    when: 2026-11-02\n"}))
	if err == nil || !strings.Contains(err.Error(), "evidence e") {
		t.Fatalf("evidence by main: %v", err)
	}

	asked := []string{}
	judgments, err := set.Judge(context.Background(), "main", Subject{Name: "a batch", Text: "x"}, func(ctx context.Context, rule Rule, prompt string) (string, error) {
		asked = append(asked, rule.ID)
		return "VERDICT: pass — it stayed quiet.", nil
	})
	if err != nil || len(judgments) != 1 || judgments[0].Verdict != Pass || !slices.Equal(asked, []string{"r"}) {
		t.Fatalf("Judge as main = %+v, %v", judgments, err)
	}
}
