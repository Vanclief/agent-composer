package knowledge

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vanclief/ez"
)

// Kind says how a rule is checked.
type Kind string

const (
	// Mechanical rules have a check command, and may also be judged.
	Mechanical Kind = "mechanical"
	// Judged rules have no check but name who is trusted to judge them.
	Judged Kind = "judged"
	// Manual rules have neither: a human reads them.
	Manual Kind = "manual"
)

// Kind says how the rule is checked: mechanical when it has a check,
// judged when it has only a judge block, manual otherwise.
func (rule Rule) Kind() Kind {
	if strings.TrimSpace(rule.Check) != "" {
		return Mechanical
	}
	if rule.Judge != nil {
		return Judged
	}

	return Manual
}

// Result is one rule's outcome. Passed is nil for judged and manual rules.
type Result struct {
	Rule   Rule   `json:"rule"`
	Kind   Kind   `json:"kind"`
	Passed *bool  `json:"passed"`
	Output string `json:"output,omitempty"`
}

// Runner runs a check command and returns its output. An error means the
// check failed.
type Runner func(ctx context.Context, command string) (string, error)

// Shell runs command with sh -c in the current directory.
func Shell(ctx context.Context, command string) (string, error) {
	output, err := exec.CommandContext(ctx, "sh", "-c", command).CombinedOutput()
	return string(output), err
}

// Check runs the rules of the ruleset named by its subject or its file, in
// order. Judged and manual rules are listed, not run.
func (s *Set) Check(ctx context.Context, ruleset string, run Runner) ([]Result, error) {
	found, exists := s.ruleset(ruleset)
	if !exists {
		return nil, ez.New(ez.ENOTFOUND, "ruleset "+ruleset+" not found", nil)
	}

	results := []Result{}
	for _, rule := range found.Rules {
		results = append(results, checkRule(ctx, rule, run))
	}

	return results, nil
}

func checkRule(ctx context.Context, rule Rule, run Runner) Result {
	kind := rule.Kind()
	if kind != Mechanical {
		return Result{Rule: rule, Kind: kind}
	}

	output, err := run(ctx, rule.Check)
	output = strings.TrimSpace(output)
	passed := err == nil
	if err != nil {
		output = strings.TrimSpace(output + "\n" + err.Error())
	}

	return Result{Rule: rule, Kind: Mechanical, Passed: &passed, Output: output}
}

func (s *Set) ruleset(name string) (Ruleset, bool) {
	for _, ruleset := range s.Rulesets {
		if ruleset.Subject == name || ruleset.File == filepath.Clean(name) {
			return ruleset, true
		}
	}

	return Ruleset{}, false
}
