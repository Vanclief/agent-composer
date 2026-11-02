package task

import (
	"strings"
	"testing"

	"github.com/vanclief/ez"
)

func validReport() Report {
	files := 41
	return Report{
		Criteria: []Criterion{{Met: "yes", Note: "test TestListAgents"}, {Met: "partly", Note: ""}},
		Changes:  &Changes{Branch: "agc/agents-core", SHA: "efbec13", Files: &files},
		Checks:   []Check{{Name: "go test ./...", Passed: true}},
		LeftOut:  []LeftOut{{What: "DM channel", Why: "needs ADR 0011"}},
		Needs:    []string{"Decide the sweep"},
	}
}

func TestReportValidate(t *testing.T) {
	negative := -1
	long := strings.Repeat("é", MaxReportNote+1)
	many := make([]string, MaxReportEntries+1)
	for i := range many {
		many[i] = "x"
	}

	cases := []struct {
		name   string
		change func(r *Report)
		want   string
	}{
		{"valid", func(r *Report) {}, ""},
		{"empty", func(r *Report) { *r = Report{} }, ""},
		{"note at the limit", func(r *Report) { r.Criteria[0].Note = long[2:] }, ""},
		{"40 hex sha", func(r *Report) { r.Changes.SHA = strings.Repeat("aB", 20) }, ""},
		{"no files", func(r *Report) { r.Changes.Files = nil }, ""},
		{"bad met", func(r *Report) { r.Criteria[1].Met = "done" }, `criteria[1].met is "done"`},
		{"met is case-sensitive", func(r *Report) { r.Criteria[0].Met = "Yes" }, `criteria[0].met is "Yes"`},
		{"long criterion note", func(r *Report) { r.Criteria[0].Note = long }, "criteria[0].note is 501 characters, at most 500"},
		{"no branch", func(r *Report) { r.Changes.Branch = "  " }, "changes.branch is required"},
		{"short sha", func(r *Report) { r.Changes.SHA = "efbec1" }, `changes.sha is "efbec1"`},
		{"long sha", func(r *Report) { r.Changes.SHA = strings.Repeat("a", 41) }, "changes.sha"},
		{"non-hex sha", func(r *Report) { r.Changes.SHA = "efbec1z" }, "changes.sha"},
		{"negative files", func(r *Report) { r.Changes.Files = &negative }, "changes.files is -1"},
		{"check without name", func(r *Report) { r.Checks[0].Name = "" }, "checks[0].name is required"},
		{"long check note", func(r *Report) { r.Checks[0].Note = long }, "checks[0].note is 501 characters"},
		{"left out without what", func(r *Report) { r.LeftOut[0].What = " " }, "left_out[0].what is required"},
		{"left out without why", func(r *Report) { r.LeftOut[0].Why = "" }, "left_out[0].why is required"},
		{"empty need", func(r *Report) { r.Needs = []string{"a", "\t"} }, "needs[1] is empty"},
		{"too many needs", func(r *Report) { r.Needs = many }, "needs has 51 entries, at most 50"},
		{"too many checks", func(r *Report) {
			r.Checks = make([]Check, MaxReportEntries+1)
		}, "checks has 51 entries"},
	}

	for _, tc := range cases {
		report := validReport()
		tc.change(&report)

		err := report.Validate()
		if tc.want == "" {
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			continue
		}

		if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), tc.want) {
			t.Fatalf("%s: got %v, want EINVALID containing %q", tc.name, err, tc.want)
		}
	}
}

func TestReportValidateForCountsCriteria(t *testing.T) {
	report := validReport()

	err := report.ValidateFor([]string{"one", "two"})
	if err != nil {
		t.Fatal(err)
	}

	err = report.ValidateFor([]string{"one", "two", "three"})
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "report has 2 criteria but the task has 3 accept criteria") {
		t.Fatalf("got %v", err)
	}

	err = report.ValidateFor(nil)
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "report has 2 criteria but the task has 0") {
		t.Fatalf("got %v", err)
	}

	empty := Report{}
	err = empty.ValidateFor(nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestReportNormalizedTrimsAndFillsLists(t *testing.T) {
	report := Report{
		Criteria: []Criterion{{Met: " yes ", Note: "  ok \n"}},
		Changes:  &Changes{Branch: " agc/x ", SHA: " abc1234 "},
		Needs:    []string{" decide "},
	}

	n := report.Normalized()
	if n.Criteria[0].Met != "yes" || n.Criteria[0].Note != "ok" || n.Changes.Branch != "agc/x" || n.Changes.SHA != "abc1234" || n.Needs[0] != "decide" {
		t.Fatalf("normalized = %+v", n)
	}
	if n.Checks == nil || n.LeftOut == nil || len(n.Checks) != 0 {
		t.Fatalf("lists must be empty arrays: %+v", n)
	}

	err := report.Validate()
	if err != nil {
		t.Fatalf("trimmed values must validate: %v", err)
	}
}
