// Package knowledge loads rules, beliefs and decisions: plain YAML files
// anything can read (docs/decisions/0006-rules-and-beliefs.md).
//
// A rule says what must or must not be done; when reality disagrees, the
// actor is wrong. A belief is something held true; when reality disagrees,
// the belief is wrong. A decision is a choice made once on beliefs; it is
// reviewed when what it rests on is in doubt. Beliefs rest on other
// beliefs or on an observation; decisions and rules rest on beliefs and
// decisions.
package knowledge

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/vanclief/ez"
	"gopkg.in/yaml.v3"
)

// idRe is the task name slug (models/task).
var idRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// Observation says who saw or decided something, and when.
type Observation struct {
	Who  string `yaml:"who" json:"who"`
	When string `yaml:"when" json:"when"`
}

// Belief is something held true. It rests on an observation, on other
// beliefs, or on both.
type Belief struct {
	ID       string       `yaml:"id" json:"id"`
	Belief   string       `yaml:"belief" json:"belief"`
	Observed *Observation `yaml:"observed" json:"observed,omitempty"`
	Because  []string     `yaml:"because" json:"because,omitempty"`
	File     string       `yaml:"-" json:"file"`
}

// Decision is a choice made once on beliefs and other decisions, whose
// outcome is an action or a standing state of the world.
type Decision struct {
	ID       string       `yaml:"id" json:"id"`
	Decision string       `yaml:"decision" json:"decision"`
	Because  []string     `yaml:"because" json:"because"`
	ADR      string       `yaml:"adr" json:"adr,omitempty"`
	Decided  *Observation `yaml:"decided" json:"decided"`
	File     string       `yaml:"-" json:"file"`
}

// Rule says what must or must not be done. Check, when set, is a shell
// command whose exit status says whether the rule holds.
type Rule struct {
	ID      string   `yaml:"id" json:"id"`
	Rule    string   `yaml:"rule" json:"rule"`
	Check   string   `yaml:"check" json:"check,omitempty"`
	Because []string `yaml:"because" json:"because,omitempty"`
	File    string   `yaml:"-" json:"file"`
}

// Ruleset is the rules of one subject, from one file.
type Ruleset struct {
	Subject string `json:"subject"`
	File    string `json:"file"`
	Rules   []Rule `json:"rules"`
}

// Set is everything loaded from a group of files.
type Set struct {
	Rulesets  []Ruleset
	Beliefs   []Belief
	Decisions []Decision

	// nodes holds every belief and decision by id, built by Validate.
	nodes map[string]node
}

// file is one YAML file; exactly one of its keys says its kind.
type file struct {
	Subject   *string     `yaml:"subject"`
	Rules     []Rule      `yaml:"rules"`
	Beliefs   *[]Belief   `yaml:"beliefs"`
	Decisions *[]Decision `yaml:"decisions"`
}

// Load reads and validates files and directories. A directory loads every
// *.yaml in it, not recursing. A file with subject: is a ruleset, one with
// beliefs: a beliefs file, one with decisions: a decisions file.
func Load(paths ...string) (*Set, error) {
	files, err := yamlFiles(paths)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	set := &Set{}
	for _, filePath := range files {
		err = set.loadFile(filePath)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	err = set.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return set, nil
}

func yamlFiles(paths []string) ([]string, error) {
	files := []string{}
	seen := map[string]bool{}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, ez.New(ez.EINVALID, "cannot read "+path, err)
		}

		found := []string{path}
		if info.IsDir() {
			found, err = filepath.Glob(filepath.Join(path, "*.yaml"))
			if err != nil {
				return nil, ez.New(ez.EINVALID, "cannot list "+path, err)
			}
		}

		for _, filePath := range found {
			clean := filepath.Clean(filePath)
			if seen[clean] {
				continue
			}
			seen[clean] = true
			files = append(files, clean)
		}
	}

	return files, nil
}

// loadFile parses one file. Unknown keys are rejected so a typo does not
// silently drop a field.
func (s *Set) loadFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ez.New(ez.EINVALID, "cannot read "+filePath, err)
	}

	var parsed file
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	err = decoder.Decode(&parsed)
	if err != nil && !errors.Is(err, io.EOF) {
		return ez.New(ez.EINVALID, filePath+": "+err.Error(), err)
	}

	isRuleset := parsed.Subject != nil
	isBeliefs := parsed.Beliefs != nil && parsed.Rules == nil
	isDecisions := parsed.Decisions != nil && parsed.Rules == nil

	switch {
	case isRuleset && parsed.Beliefs == nil && parsed.Decisions == nil:
		ruleset := Ruleset{Subject: *parsed.Subject, File: filePath}
		for _, rule := range parsed.Rules {
			rule.File = filePath
			ruleset.Rules = append(ruleset.Rules, rule)
		}
		s.Rulesets = append(s.Rulesets, ruleset)
	case isBeliefs && !isRuleset && parsed.Decisions == nil:
		for _, belief := range *parsed.Beliefs {
			belief.File = filePath
			s.Beliefs = append(s.Beliefs, belief)
		}
	case isDecisions && !isRuleset && parsed.Beliefs == nil:
		for _, decision := range *parsed.Decisions {
			decision.File = filePath
			s.Decisions = append(s.Decisions, decision)
		}
	default:
		msg := filePath + ": a file has exactly one of subject: with rules: (a ruleset), beliefs: or decisions:"
		return ez.New(ez.EINVALID, msg, nil)
	}

	return nil
}

func invalid(filePath string, kind string, id string, msg string) error {
	return ez.New(ez.EINVALID, fmt.Sprintf("%s: %s %s: %s", filePath, kind, id, msg), nil)
}
