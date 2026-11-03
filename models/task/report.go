package task

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/vanclief/ez"
)

// Delivery report limits (ADR 0010, "Delivery report").
const (
	MaxReportEntries = 50
	MaxReportNote    = 500
	// MaxShots is how many screenshots a delivery may carry, and
	// MaxShotBytes how large each PNG may be.
	MaxShots     = 8
	MaxShotBytes = 2 << 20
)

// Values of Criterion.Met.
const (
	MetYes    = "yes"
	MetNo     = "no"
	MetPartly = "partly"
)

var (
	shaRegex   = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)
	labelRegex = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
)

// pngSignature starts every PNG file.
var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

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
	// Shots are screenshots of what the change looks like, before and
	// after; a UI change carries them (ADR 0010, "Delivery report").
	Shots []Shot `json:"shots,omitempty"`
}

// Shot is one screenshot of a delivery. When sent, File is the PNG's path
// where the agent took it; the PNG itself travels next to the report, in
// the delivery's files under Label. On the stored report File is the name
// agc keeps it under and URL where it is served.
type Shot struct {
	// Label is a slug, unique per report: before and after for a UI
	// change.
	Label string `json:"label"`
	File  string `json:"file"`
	// URL is set by agc on the stored report, never sent.
	URL string `json:"url,omitempty"`
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

	for _, shot := range r.Shots {
		if out.Shots == nil {
			out.Shots = make([]Shot, 0, len(r.Shots))
		}
		out.Shots = append(out.Shots, Shot{Label: strings.TrimSpace(shot.Label), File: strings.TrimSpace(shot.File), URL: strings.TrimSpace(shot.URL)})
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

	return validateShots(n.Shots)
}

func validateShots(shots []Shot) error {
	if len(shots) > MaxShots {
		return invalid("report shots has %d entries, at most %d", len(shots), MaxShots)
	}

	seen := map[string]bool{}
	for i, shot := range shots {
		if !labelRegex.MatchString(shot.Label) {
			return invalid("report shots[%d].label is %q: use a slug such as before or after (lowercase letters, digits and dashes)", i, shot.Label)
		}
		if seen[shot.Label] {
			return invalid("report shots[%d].label %q is used twice: give each shot its own label", i, shot.Label)
		}
		seen[shot.Label] = true

		if !strings.EqualFold(path.Ext(shot.File), ".png") {
			return invalid("report shots[%d].file is %q: a shot is a PNG file (.png)", i, shot.File)
		}
	}

	return nil
}

// ValidateShotFiles checks the files a delivery carries against the
// report's shots: one PNG of at most MaxShotBytes per shot, under its
// label, and nothing else.
func (r Report) ValidateShotFiles(files map[string][]byte) error {
	shots := r.Normalized().Shots
	labels := map[string]bool{}

	for _, shot := range shots {
		labels[shot.Label] = true

		data, ok := files[shot.Label]
		if !ok {
			return invalid("shot %q has no file: send the PNG under files.%s", shot.Label, shot.Label)
		}
		if len(data) > MaxShotBytes {
			return invalid("shot %q is %d bytes, at most %d (2 MB): take a smaller screenshot", shot.Label, len(data), MaxShotBytes)
		}
		if !IsPNG(data) {
			return invalid("shot %q is not a PNG: send a .png screenshot", shot.Label)
		}
	}

	for label := range files {
		if !labels[label] {
			return invalid("files has %q, which no shot in the report names", label)
		}
	}

	return nil
}

// IsShotLabel reports whether label is a valid shot label, a slug.
func IsShotLabel(label string) bool {
	return labelRegex.MatchString(label)
}

// IsPNG reports whether data starts with the PNG signature.
func IsPNG(data []byte) bool {
	return bytes.HasPrefix(data, pngSignature)
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
