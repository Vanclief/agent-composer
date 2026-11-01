// Package knowledge loads evidence, rules, beliefs and decisions: plain
// YAML files anything can read (docs/decisions/0006-rules-and-beliefs.md).
//
// Evidence is something seen: who saw it, when, and where. A rule says what
// must or must not be done; when reality disagrees, the actor is wrong. A
// belief is something held true; when reality disagrees, the belief is
// wrong. A decision is a choice made once on beliefs; it is reviewed when
// what it rests on is in doubt. Beliefs rest on other beliefs and cite
// evidence; decisions and rules rest on beliefs and decisions. Whether a
// belief is grounded, an opinion or in doubt is computed from its evidence
// when asked, never stored.
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

// Retraction says who withdrew a piece of evidence, when and why.
type Retraction struct {
	Who  string `yaml:"who" json:"who"`
	When string `yaml:"when" json:"when"`
	Why  string `yaml:"why" json:"why"`
}

// Evidence is something seen. Source, when set, is a URL, a file path or a
// command whose output was seen. Retracted evidence stays in its file but
// no longer grounds anything.
type Evidence struct {
	ID        string      `yaml:"id" json:"id"`
	Evidence  string      `yaml:"evidence" json:"evidence"`
	Who       string      `yaml:"who" json:"who"`
	When      string      `yaml:"when" json:"when"`
	Source    string      `yaml:"source" json:"source,omitempty"`
	Retracted *Retraction `yaml:"retracted" json:"retracted,omitempty"`
	File      string      `yaml:"-" json:"file"`
}

// Belief is something held true. It rests on other beliefs, cites
// evidence, both or neither; one with no evidence under it is an opinion.
type Belief struct {
	ID       string   `yaml:"id" json:"id"`
	Belief   string   `yaml:"belief" json:"belief"`
	Because  []string `yaml:"because" json:"because,omitempty"`
	Evidence []string `yaml:"evidence" json:"evidence,omitempty"`
	File     string   `yaml:"-" json:"file"`
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
	Evidence  []Evidence
	Rulesets  []Ruleset
	Beliefs   []Belief
	Decisions []Decision

	// nodes holds every belief and decision by id, and evidenceByID every
	// piece of evidence, both built by Validate.
	nodes        map[string]node
	evidenceByID map[string]Evidence
}

// file is one YAML file; exactly one of its keys says its kind.
type file struct {
	Subject   *string     `yaml:"subject"`
	Rules     []Rule      `yaml:"rules"`
	Evidence  *[]Evidence `yaml:"evidence"`
	Beliefs   *[]Belief   `yaml:"beliefs"`
	Decisions *[]Decision `yaml:"decisions"`
}

// Load reads and validates files and directories. A directory loads every
// *.yaml in it, not recursing. A file with subject: is a ruleset, one with
// evidence: an evidence file, one with beliefs: a beliefs file, one with
// decisions: a decisions file.
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

	err = rejectObserved(filePath, data)
	if err != nil {
		return err
	}

	var parsed file
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	err = decoder.Decode(&parsed)
	if err != nil && !errors.Is(err, io.EOF) {
		return ez.New(ez.EINVALID, filePath+": "+err.Error(), err)
	}

	kinds := 0
	for _, present := range []bool{parsed.Subject != nil, parsed.Evidence != nil, parsed.Beliefs != nil, parsed.Decisions != nil} {
		if present {
			kinds++
		}
	}
	if kinds != 1 || (parsed.Rules != nil && parsed.Subject == nil) {
		msg := filePath + ": a file has exactly one of subject: with rules: (a ruleset), evidence:, beliefs: or decisions:"
		return ez.New(ez.EINVALID, msg, nil)
	}

	switch {
	case parsed.Subject != nil:
		ruleset := Ruleset{Subject: *parsed.Subject, File: filePath}
		for _, rule := range parsed.Rules {
			rule.File = filePath
			ruleset.Rules = append(ruleset.Rules, rule)
		}
		s.Rulesets = append(s.Rulesets, ruleset)
	case parsed.Evidence != nil:
		for _, evidence := range *parsed.Evidence {
			evidence.File = filePath
			s.Evidence = append(s.Evidence, evidence)
		}
	case parsed.Beliefs != nil:
		for _, belief := range *parsed.Beliefs {
			belief.File = filePath
			s.Beliefs = append(s.Beliefs, belief)
		}
	default:
		for _, decision := range *parsed.Decisions {
			decision.File = filePath
			s.Decisions = append(s.Decisions, decision)
		}
	}

	return nil
}

// rejectObserved names a belief still written with observed:, which became
// evidence files cited by id, so the error says what to do instead of
// naming a Go type. Anything else wrong with the file is left to the strict
// decode.
func rejectObserved(filePath string, data []byte) error {
	var legacy struct {
		Beliefs []struct {
			ID       string `yaml:"id"`
			Observed any    `yaml:"observed"`
		} `yaml:"beliefs"`
	}
	err := yaml.Unmarshal(data, &legacy)
	if err != nil {
		return nil
	}

	for _, belief := range legacy.Beliefs {
		if belief.Observed != nil {
			msg := "observed: is no longer a belief field: move it to an evidence file and cite it with evidence: [ids]"
			return invalid(filePath, "belief", belief.ID, msg)
		}
	}

	return nil
}

func invalid(filePath string, kind string, id string, msg string) error {
	return ez.New(ez.EINVALID, fmt.Sprintf("%s: %s %s: %s", filePath, kind, id, msg), nil)
}
