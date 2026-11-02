package task

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/vanclief/ez"
)

// Delivery report limits (ADR 0010, "Delivery report").
const (
	MaxReportEntries = 50
	MaxReportNote    = 500
)

// Values of Criterion.Met.
const (
	MetYes    = "yes"
	MetNo     = "no"
	MetPartly = "partly"
)

var shaRegex = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)

// Report is what a delivery says against the task's criteria, so the
// reviewer judges without reading the channel (ADR 0010, "Delivery
// report"). Stored on the task next to Summary.
type Report struct {
	// Criteria has one entry per line of the task's Accept, in order.
	Criteria []Criterion `json:"criteria"`
	// Changes is where the work is; nil when the delivery names none.
	Changes *Changes  `json:"changes"`
	Checks  []Check   `json:"checks"`
	LeftOut []LeftOut `json:"left_out"`
	// Needs is what the agent needs the reviewer to decide.
	Needs []string `json:"needs"`
}

// Criterion is the agent's verdict on one accept criterion.
type Criterion struct {
	// Met is yes, no or partly.
	Met  string `json:"met"`
	Note string `json:"note"`
}

// Changes names the pushed work.
type Changes struct {
	Branch string `json:"branch"`
	// SHA is 7 to 40 hex characters.
	SHA string `json:"sha"`
	// Files is how many files changed; nil when not given.
	Files *int `json:"files"`
}

// Check is a command or check the agent ran and its outcome.
type Check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Note   string `json:"note"`
}

// LeftOut is something not done, and why.
type LeftOut struct {
	What string `json:"what"`
	Why  string `json:"why"`
}

// Normalized returns a copy with every string trimmed and every list an
// empty array rather than nil.
func (r Report) Normalized() Report {
	out := Report{
		Criteria: make([]Criterion, 0, len(r.Criteria)),
		Checks:   make([]Check, 0, len(r.Checks)),
		LeftOut:  make([]LeftOut, 0, len(r.LeftOut)),
		Needs:    make([]string, 0, len(r.Needs)),
	}

	for _, c := range r.Criteria {
		out.Criteria = append(out.Criteria, Criterion{Met: strings.TrimSpace(c.Met), Note: strings.TrimSpace(c.Note)})
	}

	if r.Changes != nil {
		changes := Changes{
			Branch: strings.TrimSpace(r.Changes.Branch),
			SHA:    strings.TrimSpace(r.Changes.SHA),
		}
		if r.Changes.Files != nil {
			files := *r.Changes.Files
			changes.Files = &files
		}
		out.Changes = &changes
	}

	for _, c := range r.Checks {
		out.Checks = append(out.Checks, Check{Name: strings.TrimSpace(c.Name), Passed: c.Passed, Note: strings.TrimSpace(c.Note)})
	}

	for _, l := range r.LeftOut {
		out.LeftOut = append(out.LeftOut, LeftOut{What: strings.TrimSpace(l.What), Why: strings.TrimSpace(l.Why)})
	}

	for _, n := range r.Needs {
		out.Needs = append(out.Needs, strings.TrimSpace(n))
	}

	return out
}

// Validate checks the report's shape, after trimming. The count of
// criteria against the task is ValidateFor.
func (r Report) Validate() error {
	n := r.Normalized()

	lists := []struct {
		name  string
		count int
	}{
		{"criteria", len(n.Criteria)},
		{"checks", len(n.Checks)},
		{"left_out", len(n.LeftOut)},
		{"needs", len(n.Needs)},
	}
	for _, list := range lists {
		if list.count > MaxReportEntries {
			return invalid("report %s has %d entries, at most %d", list.name, list.count, MaxReportEntries)
		}
	}

	for i, c := range n.Criteria {
		if c.Met != MetYes && c.Met != MetNo && c.Met != MetPartly {
			return invalid("report criteria[%d].met is %q: use yes, no or partly", i, c.Met)
		}
		err := validateNote("criteria", i, c.Note)
		if err != nil {
			return err
		}
	}

	if n.Changes != nil {
		if n.Changes.Branch == "" {
			return invalid("report changes.branch is required when changes is given")
		}
		if !shaRegex.MatchString(n.Changes.SHA) {
			return invalid("report changes.sha is %q: use 7 to 40 hex characters", n.Changes.SHA)
		}
		if n.Changes.Files != nil && *n.Changes.Files < 0 {
			return invalid("report changes.files is %d: it cannot be negative", *n.Changes.Files)
		}
	}

	for i, c := range n.Checks {
		if c.Name == "" {
			return invalid("report checks[%d].name is required", i)
		}
		err := validateNote("checks", i, c.Note)
		if err != nil {
			return err
		}
	}

	for i, l := range n.LeftOut {
		if l.What == "" {
			return invalid("report left_out[%d].what is required", i)
		}
		if l.Why == "" {
			return invalid("report left_out[%d].why is required", i)
		}
	}

	for i, need := range n.Needs {
		if need == "" {
			return invalid("report needs[%d] is empty", i)
		}
	}

	return nil
}

// ValidateFor checks the report against the task's accept criteria: one
// entry per criterion, in order, or none when the task has none.
func (r Report) ValidateFor(accept []string) error {
	err := r.Validate()
	if err != nil {
		return err
	}

	if len(r.Criteria) != len(accept) {
		return invalid("report has %d criteria but the task has %d accept criteria: give one per \"Accepted when\" line, in order", len(r.Criteria), len(accept))
	}

	return nil
}

func validateNote(list string, i int, note string) error {
	count := utf8.RuneCountInString(note)
	if count > MaxReportNote {
		return invalid("report %s[%d].note is %d characters, at most %d", list, i, count, MaxReportNote)
	}

	return nil
}

func invalid(format string, args ...any) error {
	return ez.New(ez.EINVALID, fmt.Sprintf(format, args...), nil)
}
