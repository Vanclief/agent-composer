// Package knowledge loads evidence, rules, beliefs, decisions and actions:
// plain YAML files anything can read
// (docs/decisions/0006-rules-and-beliefs.md).
//
// Evidence is something seen: who saw it, when, and where. A rule says what
// must or must not be done; when reality disagrees, the actor is wrong. A
// belief is something held true; when reality disagrees, the belief is
// wrong. A decision is a choice made once on beliefs; it is reviewed when
// what it rests on is in doubt. An action is something to do because of a
// decision, open until it is done. Beliefs rest on other beliefs and cite
// evidence; decisions, rules and actions rest on beliefs and decisions.
// Any of them may list alternative reasons under or:, and beliefs and
// evidence may name what they contradict. Whether a belief is grounded, an
// opinion, contested or in doubt is computed from its evidence when asked,
// never stored.
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
// command whose output was seen. Who is the witness, a person or a
// mechanical check such as ci, never a model: a model only translates a
// real source into a record, and TranslatedBy names it then. Retracted
// evidence stays in its file but no longer grounds anything. Contradicts
// names beliefs or evidence that cannot be true along with it.
type Evidence struct {
	ID           string      `yaml:"id" json:"id"`
	Evidence     string      `yaml:"evidence" json:"evidence"`
	Who          string      `yaml:"who" json:"who"`
	When         string      `yaml:"when" json:"when"`
	Source       string      `yaml:"source" json:"source,omitempty"`
	TranslatedBy string      `yaml:"translated_by" json:"translated_by,omitempty"`
	Retracted    *Retraction `yaml:"retracted" json:"retracted,omitempty"`
	Contradicts  []string    `yaml:"contradicts" json:"contradicts,omitempty"`
	File         string      `yaml:"-" json:"file"`
}

// Support is one set of reasons, all of which must hold together. Only
// beliefs cite evidence: a set is grounded when every belief in Because is
// and, if Evidence is listed, one piece of it is live.
type Support struct {
	Because  []string `yaml:"because" json:"because,omitempty"`
	Evidence []string `yaml:"evidence" json:"evidence,omitempty"`
}

// Belief is something held true. It rests on other beliefs, cites
// evidence, both or neither; one with no evidence under it is an opinion.
// Or lists alternative reasons: the belief is grounded if its own because
// and evidence are, or if any one alternative is. Contradicts names
// beliefs or evidence that cannot be true along with it; declaring it on
// one side is enough. By, when set, names who holds or stated the
// belief, a person or an agent, and When the day they did; neither grounds
// anything, so a belief by an agent with no evidence is still an opinion.
type Belief struct {
	ID          string    `yaml:"id" json:"id"`
	Belief      string    `yaml:"belief" json:"belief"`
	Because     []string  `yaml:"because" json:"because,omitempty"`
	Evidence    []string  `yaml:"evidence" json:"evidence,omitempty"`
	Or          []Support `yaml:"or" json:"or,omitempty"`
	Contradicts []string  `yaml:"contradicts" json:"contradicts,omitempty"`
	By          string    `yaml:"by" json:"by,omitempty"`
	When        string    `yaml:"when" json:"when,omitempty"`
	File        string    `yaml:"-" json:"file"`
}

// Supports lists the belief's own reasons first, then its alternatives.
func (belief Belief) Supports() []Support {
	return append([]Support{{Because: belief.Because, Evidence: belief.Evidence}}, belief.Or...)
}

// Decision is a choice made once on beliefs and other decisions, whose
// outcome is an action or a standing state of the world.
type Decision struct {
	ID       string       `yaml:"id" json:"id"`
	Decision string       `yaml:"decision" json:"decision"`
	Because  []string     `yaml:"because" json:"because"`
	Or       []Support    `yaml:"or" json:"or,omitempty"`
	ADR      string       `yaml:"adr" json:"adr,omitempty"`
	Decided  *Observation `yaml:"decided" json:"decided"`
	File     string       `yaml:"-" json:"file"`
}

// Supports lists the decision's own reasons first, then its alternatives.
func (decision Decision) Supports() []Support {
	return append([]Support{{Because: decision.Because}}, decision.Or...)
}

// Rule says what must or must not be done. Check, when set, is a shell
// command whose exit status says whether the rule holds. Judge, when set,
// names who is trusted to judge it by reading: a rule may have either,
// both or neither.
type Rule struct {
	ID      string    `yaml:"id" json:"id"`
	Rule    string    `yaml:"rule" json:"rule"`
	Check   string    `yaml:"check" json:"check,omitempty"`
	Judge   *Judge    `yaml:"judge" json:"judge,omitempty"`
	Because []string  `yaml:"because" json:"because,omitempty"`
	Or      []Support `yaml:"or" json:"or,omitempty"`
	File    string    `yaml:"-" json:"file"`
}

// Supports lists the rule's own reasons first, then its alternatives.
func (rule Rule) Supports() []Support {
	return append([]Support{{Because: rule.Because}}, rule.Or...)
}

// Action is something to do because of a decision. It is open until Done
// is set.
type Action struct {
	ID      string       `yaml:"id" json:"id"`
	Action  string       `yaml:"action" json:"action"`
	Because []string     `yaml:"because" json:"because"`
	Or      []Support    `yaml:"or" json:"or,omitempty"`
	Opened  *Observation `yaml:"opened" json:"opened"`
	Done    *Observation `yaml:"done" json:"done,omitempty"`
	File    string       `yaml:"-" json:"file"`
}

// Supports lists the action's own reasons first, then its alternatives.
func (action Action) Supports() []Support {
	return append([]Support{{Because: action.Because}}, action.Or...)
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
	Actions   []Action

	// nodes holds every belief and decision by id, evidenceByID every
	// piece of evidence, contradicts both sides of every contradiction and
	// proper the words written capitalized mid-sentence, all built by
	// Validate. without holds the ids set aside by Without.
	nodes        map[string]node
	evidenceByID map[string]Evidence
	contradicts  map[string][]string
	proper       map[string]bool
	without      map[string]bool
}

// file is one YAML file; exactly one of its keys says its kind.
type file struct {
	Subject   *string     `yaml:"subject"`
	Rules     []Rule      `yaml:"rules"`
	Evidence  *[]Evidence `yaml:"evidence"`
	Beliefs   *[]Belief   `yaml:"beliefs"`
	Decisions *[]Decision `yaml:"decisions"`
	Actions   *[]Action   `yaml:"actions"`
}

// Load reads and validates files and directories. A directory loads every
// *.yaml in it, not recursing. A file with subject: is a ruleset, one with
// evidence: an evidence file, one with beliefs: a beliefs file, one with
// decisions: a decisions file, one with actions: an actions file.
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
	for _, present := range []bool{parsed.Subject != nil, parsed.Evidence != nil, parsed.Beliefs != nil, parsed.Decisions != nil, parsed.Actions != nil} {
		if present {
			kinds++
		}
	}
	if kinds != 1 || (parsed.Rules != nil && parsed.Subject == nil) {
		msg := filePath + ": a file has exactly one of subject: with rules: (a ruleset), evidence:, beliefs:, decisions: or actions:"
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
	case parsed.Decisions != nil:
		for _, decision := range *parsed.Decisions {
			decision.File = filePath
			s.Decisions = append(s.Decisions, decision)
		}
	default:
		for _, action := range *parsed.Actions {
			action.File = filePath
			s.Actions = append(s.Actions, action)
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
