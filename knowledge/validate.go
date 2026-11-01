package knowledge

import (
	"slices"
	"strings"
)

// node is a belief or a decision in the because graph, with its own
// reasons first and its alternatives after. Only beliefs cite evidence.
type node struct {
	kind string
	file string
	sets []Support
}

// because lists every belief and decision the node rests on, in any set.
func (n node) because() []string {
	all := []string{}
	for _, set := range n.sets {
		all = append(all, set.Because...)
	}

	return all
}

// Validate checks ids, required fields and that every because, every cited
// piece of evidence and every contradiction resolves, and rejects cycles
// among beliefs and decisions, alternatives included. Errors name the file
// and the id.
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
		s.nodes[belief.ID] = node{kind: "belief", file: belief.File, sets: belief.Supports()}
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
		s.nodes[decision.ID] = node{kind: "decision", file: decision.File, sets: decision.Supports()}
	}

	for _, ruleset := range s.Rulesets {
		err := validateRuleset(owners, ruleset)
		if err != nil {
			return err
		}
	}

	for _, action := range s.Actions {
		err := claim(owners, action.File, "action", action.ID)
		if err != nil {
			return err
		}

		err = validateAction(action)
		if err != nil {
			return err
		}
	}

	err := s.resolveAll()
	if err != nil {
		return err
	}

	err = s.resolveContradictions()
	if err != nil {
		return err
	}
	s.proper = properWords(s.texts())

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

	if isModel(evidence.Who) {
		msg := "who must be a person or a mechanical check, not a model; put the model in translated_by and the real source in source, or record this as an opinion"
		return invalid(evidence.File, "evidence", evidence.ID, msg)
	}

	if strings.TrimSpace(evidence.TranslatedBy) != "" && strings.TrimSpace(evidence.Source) == "" {
		return invalid(evidence.File, "evidence", evidence.ID, "translated_by needs source: the real source the model read")
	}

	retracted := evidence.Retracted
	if retracted != nil && (strings.TrimSpace(retracted.Who) == "" || strings.TrimSpace(retracted.When) == "" || strings.TrimSpace(retracted.Why) == "") {
		return invalid(evidence.File, "evidence", evidence.ID, "retracted: needs who, when and why")
	}

	return nil
}

// isModel says whether who names a model or an agent harness rather than a
// witness. An LLM is a great parser and a terrible sensor: a model can
// translate a source into evidence but never be the source. The names are
// the harness ids and model families in runtime/harnesses/catalog.go
// (codex_cli, claude_code, pi; claude-*, gpt-*) plus generic words for a
// model; keep them in step when the catalog grows.
func isModel(who string) bool {
	name := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(who)), "_", "-")
	models := []string{"claude-code", "codex-cli", "pi", "claude", "codex", "gpt", "gemini", "llm", "model", "agent"}
	if slices.Contains(models, name) {
		return true
	}

	for _, prefix := range []string{"claude-", "gpt-", "gemini-"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return false
}

// validateBelief checks the text and the alternatives: a belief with no
// because and no evidence is allowed, and shows up as an opinion, but an
// alternative is only an alternative to reasons the belief already has.
func validateBelief(belief Belief) error {
	if strings.TrimSpace(belief.Belief) == "" {
		return invalid(belief.File, "belief", belief.ID, "belief: text is required")
	}

	if len(belief.Or) > 0 && len(belief.Because) == 0 && len(belief.Evidence) == 0 {
		return invalid(belief.File, "belief", belief.ID, "or: is an alternative to because: or evidence:, which are empty")
	}

	for _, set := range belief.Or {
		if len(set.Because) == 0 && len(set.Evidence) == 0 {
			return invalid(belief.File, "belief", belief.ID, "each or: entry needs because: or evidence:")
		}
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

	return validateOr(decision.File, "decision", decision.ID, decision.Or)
}

func validateAction(action Action) error {
	if strings.TrimSpace(action.Action) == "" {
		return invalid(action.File, "action", action.ID, "action: text is required")
	}

	if len(action.Because) == 0 {
		return invalid(action.File, "action", action.ID, "because: is required")
	}

	if action.Opened == nil || !complete(action.Opened) {
		return invalid(action.File, "action", action.ID, "opened: needs who and when")
	}

	if action.Done != nil && !complete(action.Done) {
		return invalid(action.File, "action", action.ID, "done: needs who and when")
	}

	return validateOr(action.File, "action", action.ID, action.Or)
}

// validateOr checks the alternatives of a decision, rule or action: each
// names beliefs or decisions and cites no evidence.
func validateOr(filePath string, kind string, id string, sets []Support) error {
	for _, set := range sets {
		if len(set.Because) == 0 {
			return invalid(filePath, kind, id, "each or: entry needs because:")
		}
		if len(set.Evidence) > 0 {
			return invalid(filePath, kind, id, "only beliefs cite evidence, so or: takes because: only")
		}
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

		if len(rule.Or) > 0 && len(rule.Because) == 0 {
			return invalid(rule.File, "rule", rule.ID, "or: is an alternative to because:, which is empty")
		}

		err = validateOr(rule.File, "rule", rule.ID, rule.Or)
		if err != nil {
			return err
		}
	}

	return nil
}

// resolveAll checks every because and every cited piece of evidence, in
// every set: beliefs rest on beliefs and cite evidence, decisions, rules
// and actions rest on beliefs or decisions, and an action serves at least
// one decision.
func (s *Set) resolveAll() error {
	for _, belief := range s.Beliefs {
		for _, set := range belief.Supports() {
			err := s.resolve(belief.File, "belief", belief.ID, set.Because, true)
			if err != nil {
				return err
			}

			for _, evidenceID := range set.Evidence {
				_, exists := s.evidenceByID[evidenceID]
				if !exists {
					return invalid(belief.File, "belief", belief.ID, "evidence names "+evidenceID+", which is not loaded evidence")
				}
			}
		}
	}

	for _, decision := range s.Decisions {
		for _, set := range decision.Supports() {
			err := s.resolve(decision.File, "decision", decision.ID, set.Because, false)
			if err != nil {
				return err
			}
		}
	}

	for _, ruleset := range s.Rulesets {
		for _, rule := range ruleset.Rules {
			for _, set := range rule.Supports() {
				err := s.resolve(rule.File, "rule", rule.ID, set.Because, false)
				if err != nil {
					return err
				}
			}
		}
	}

	for _, action := range s.Actions {
		for _, set := range action.Supports() {
			err := s.resolve(action.File, "action", action.ID, set.Because, false)
			if err != nil {
				return err
			}
		}

		serves := slices.ContainsFunc(action.Because, func(id string) bool {
			return s.nodes[id].kind == "decision"
		})
		if !serves {
			return invalid(action.File, "action", action.ID, "because: needs at least one decision")
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

// resolveContradictions checks that every contradicts names a belief or a
// piece of evidence other than itself, and records each contradiction on
// both sides.
func (s *Set) resolveContradictions() error {
	s.contradicts = map[string][]string{}

	record := func(filePath string, kind string, id string, targets []string) error {
		for _, target := range targets {
			_, isBelief := s.belief(target)
			_, isEvidence := s.evidenceByID[target]
			if target == id || (!isBelief && !isEvidence) {
				return invalid(filePath, kind, id, "contradicts names "+target+", which is not another loaded belief or evidence")
			}

			if !slices.Contains(s.contradicts[id], target) {
				s.contradicts[id] = append(s.contradicts[id], target)
				s.contradicts[target] = append(s.contradicts[target], id)
			}
		}

		return nil
	}

	for _, evidence := range s.Evidence {
		err := record(evidence.File, "evidence", evidence.ID, evidence.Contradicts)
		if err != nil {
			return err
		}
	}

	for _, belief := range s.Beliefs {
		err := record(belief.File, "belief", belief.ID, belief.Contradicts)
		if err != nil {
			return err
		}
	}

	return nil
}

const (
	unvisited = iota
	visiting
	visited
)

// findCycle walks the because graph depth first from every node, through
// every set of reasons.
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
	for _, next := range s.nodes[id].because() {
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

// texts lists every text loaded, for properWords.
func (s *Set) texts() []string {
	texts := []string{}
	for _, evidence := range s.Evidence {
		texts = append(texts, evidence.Evidence)
	}
	for _, belief := range s.Beliefs {
		texts = append(texts, belief.Belief)
	}
	for _, decision := range s.Decisions {
		texts = append(texts, decision.Decision)
	}
	for _, ruleset := range s.Rulesets {
		for _, rule := range ruleset.Rules {
			texts = append(texts, rule.Rule)
		}
	}
	for _, action := range s.Actions {
		texts = append(texts, action.Action)
	}

	return texts
}
