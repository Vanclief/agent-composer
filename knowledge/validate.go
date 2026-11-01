package knowledge

import (
	"slices"
	"strings"
)

// node is a belief or a decision in the because graph. Only beliefs cite
// evidence.
type node struct {
	kind     string
	file     string
	because  []string
	evidence []string
}

// Validate checks ids, required fields and that every because and every
// cited piece of evidence resolves, and rejects cycles among beliefs and
// decisions. Errors name the file and the id.
func (s *Set) Validate() error {
	s.nodes = map[string]node{}
	s.evidenceByID = map[string]Evidence{}
	owners := map[string]string{}

	for _, evidence := range s.Evidence {
		err := claim(owners, evidence.File, "evidence", evidence.ID)
		if err != nil {
			return err
		}

		err = validateEvidence(evidence)
		if err != nil {
			return err
		}
		s.evidenceByID[evidence.ID] = evidence
	}

	for _, belief := range s.Beliefs {
		err := claim(owners, belief.File, "belief", belief.ID)
		if err != nil {
			return err
		}

		err = validateBelief(belief)
		if err != nil {
			return err
		}
		s.nodes[belief.ID] = node{kind: "belief", file: belief.File, because: belief.Because, evidence: belief.Evidence}
	}

	for _, decision := range s.Decisions {
		err := claim(owners, decision.File, "decision", decision.ID)
		if err != nil {
			return err
		}

		err = validateDecision(decision)
		if err != nil {
			return err
		}
		s.nodes[decision.ID] = node{kind: "decision", file: decision.File, because: decision.Because}
	}

	for _, ruleset := range s.Rulesets {
		err := validateRuleset(owners, ruleset)
		if err != nil {
			return err
		}
	}

	err := s.resolveAll()
	if err != nil {
		return err
	}

	return s.findCycle()
}

// claim records id as used by filePath, rejecting bad and repeated ids.
func claim(owners map[string]string, filePath string, kind string, id string) error {
	if !idRe.MatchString(id) {
		return invalid(filePath, kind, id, "id must be 1-63 lowercase letters, digits or -, starting with a letter")
	}

	owner, taken := owners[id]
	if taken {
		return invalid(filePath, kind, id, "id is already used in "+owner)
	}
	owners[id] = filePath

	return nil
}

func validateEvidence(evidence Evidence) error {
	if strings.TrimSpace(evidence.Evidence) == "" {
		return invalid(evidence.File, "evidence", evidence.ID, "evidence: text is required")
	}

	if strings.TrimSpace(evidence.Who) == "" || strings.TrimSpace(evidence.When) == "" {
		return invalid(evidence.File, "evidence", evidence.ID, "needs who and when")
	}

	retracted := evidence.Retracted
	if retracted != nil && (strings.TrimSpace(retracted.Who) == "" || strings.TrimSpace(retracted.When) == "" || strings.TrimSpace(retracted.Why) == "") {
		return invalid(evidence.File, "evidence", evidence.ID, "retracted: needs who, when and why")
	}

	return nil
}

// validateBelief checks the text only: a belief with no because and no
// evidence is allowed, and shows up as an opinion.
func validateBelief(belief Belief) error {
	if strings.TrimSpace(belief.Belief) == "" {
		return invalid(belief.File, "belief", belief.ID, "belief: text is required")
	}

	return nil
}

func validateDecision(decision Decision) error {
	if strings.TrimSpace(decision.Decision) == "" {
		return invalid(decision.File, "decision", decision.ID, "decision: text is required")
	}

	if len(decision.Because) == 0 {
		return invalid(decision.File, "decision", decision.ID, "because: is required")
	}

	if decision.Decided == nil || !complete(decision.Decided) {
		return invalid(decision.File, "decision", decision.ID, "decided: needs who and when")
	}

	return nil
}

func complete(observation *Observation) bool {
	return strings.TrimSpace(observation.Who) != "" && strings.TrimSpace(observation.When) != ""
}

func validateRuleset(owners map[string]string, ruleset Ruleset) error {
	if strings.TrimSpace(ruleset.Subject) == "" {
		return invalid(ruleset.File, "ruleset", "", "subject: is required")
	}

	for _, rule := range ruleset.Rules {
		err := claim(owners, rule.File, "rule", rule.ID)
		if err != nil {
			return err
		}

		if strings.TrimSpace(rule.Rule) == "" {
			return invalid(rule.File, "rule", rule.ID, "rule: text is required")
		}
	}

	return nil
}

// resolveAll checks every because and every cited piece of evidence:
// beliefs rest on beliefs and cite evidence, decisions and rules rest on
// beliefs or decisions.
func (s *Set) resolveAll() error {
	for _, belief := range s.Beliefs {
		err := s.resolve(belief.File, "belief", belief.ID, belief.Because, true)
		if err != nil {
			return err
		}

		for _, evidenceID := range belief.Evidence {
			_, exists := s.evidenceByID[evidenceID]
			if !exists {
				return invalid(belief.File, "belief", belief.ID, "evidence names "+evidenceID+", which is not loaded evidence")
			}
		}
	}

	for _, decision := range s.Decisions {
		err := s.resolve(decision.File, "decision", decision.ID, decision.Because, false)
		if err != nil {
			return err
		}
	}

	for _, ruleset := range s.Rulesets {
		for _, rule := range ruleset.Rules {
			err := s.resolve(rule.File, "rule", rule.ID, rule.Because, false)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Set) resolve(filePath string, kind string, id string, because []string, beliefsOnly bool) error {
	want := "belief or decision"
	if beliefsOnly {
		want = "belief"
	}

	for _, target := range because {
		found, exists := s.nodes[target]
		if !exists || (beliefsOnly && found.kind != "belief") {
			return invalid(filePath, kind, id, "because names "+target+", which is not a loaded "+want)
		}
	}

	return nil
}

const (
	unvisited = iota
	visiting
	visited
)

// findCycle walks the because graph depth first from every node.
func (s *Set) findCycle() error {
	state := map[string]int{}
	for _, id := range s.nodeIDs() {
		cycle := s.visit(id, state, nil)
		if cycle != nil {
			first := s.nodes[cycle[0]]
			return invalid(first.file, first.kind, cycle[0], "because forms a cycle: "+strings.Join(cycle, " -> "))
		}
	}

	return nil
}

// visit returns the cycle reachable from id, if there is one.
func (s *Set) visit(id string, state map[string]int, stack []string) []string {
	stack = append(stack, id)
	if state[id] == visiting {
		return stack[slices.Index(stack, id):]
	}
	if state[id] == visited {
		return nil
	}

	state[id] = visiting
	for _, next := range s.nodes[id].because {
		cycle := s.visit(next, state, stack)
		if cycle != nil {
			return cycle
		}
	}
	state[id] = visited

	return nil
}

// nodeIDs lists beliefs then decisions in load order, so output and error
// messages are stable.
func (s *Set) nodeIDs() []string {
	ids := []string{}
	for _, belief := range s.Beliefs {
		ids = append(ids, belief.ID)
	}
	for _, decision := range s.Decisions {
		ids = append(ids, decision.ID)
	}

	return ids
}
