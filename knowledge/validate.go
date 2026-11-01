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
	// judged says the belief comes from someone's experience, so as a
	// leaf it is a judgment rather than an opinion.
	judged bool
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
		s.nodes[belief.ID] = node{kind: "belief", file: belief.File, sets: belief.Supports(), judged: belief.Experience != nil}
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

	for _, proposal := range s.Proposals {
		err := claim(owners, proposal.File, "proposal", proposal.ID)
		if err != nil {
			return err
		}

		err = validateProposal(proposal)
		if err != nil {
			return err
		}
	}

	for _, gate := range s.Gates {
		err := claim(owners, gate.File, "gate", gate.ID)
		if err != nil {
			return err
		}

		err = validateGate(gate)
		if err != nil {
			return err
		}
	}

	err := s.resolveAll()
	if err != nil {
		return err
	}

	err = s.resolveProposals(owners)
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

// validateBelief checks the text, the alternatives and the experience: a
// belief with no because and no evidence is allowed, and shows up as an
// opinion or, with experience, a judgment, but an alternative is only an
// alternative to reasons the belief already has. By is free text.
func validateBelief(belief Belief) error {
	if strings.TrimSpace(belief.Belief) == "" {
		return invalid(belief.File, "belief", belief.ID, "belief: text is required")
	}

	if strings.TrimSpace(belief.When) != "" && strings.TrimSpace(belief.By) == "" {
		return invalid(belief.File, "belief", belief.ID, "when: says when by: held it, and by: is empty")
	}

	if len(belief.Or) > 0 && len(belief.Because) == 0 && len(belief.Evidence) == 0 {
		return invalid(belief.File, "belief", belief.ID, "or: is an alternative to because: or evidence:, which are empty")
	}

	for _, set := range belief.Or {
		if len(set.Because) == 0 && len(set.Evidence) == 0 {
			return invalid(belief.File, "belief", belief.ID, "each or: entry needs because: or evidence:")
		}
	}

	experience := belief.Experience
	if experience != nil && (strings.TrimSpace(experience.Of) == "" || strings.TrimSpace(experience.In) == "") {
		return invalid(belief.File, "belief", belief.ID, "experience: needs of and in")
	}

	if experience != nil && isModel(experience.Of) {
		msg := "experience: of must be a person, not a model; a model's own conclusion is an opinion"
		return invalid(belief.File, "belief", belief.ID, msg)
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

// validateJudge checks a rule's judge block: someone is trusted, and each
// name is a harness id or a person, never a model name that agc cannot run.
func validateJudge(rule Rule) error {
	if rule.Judge == nil {
		return nil
	}

	if len(rule.Judge.Trusted) == 0 {
		return invalid(rule.File, "rule", rule.ID, "judge: needs trusted: harness ids or people")
	}

	for _, name := range rule.Judge.Trusted {
		if strings.TrimSpace(name) == "" {
			return invalid(rule.File, "rule", rule.ID, "judge: trusted has an empty name")
		}
		if isModel(name) && !slices.Contains(harnessIDs, name) {
			msg := "judge: trusted names " + name + ", a model but not a harness id; use one of " + strings.Join(harnessIDs, ", ") + ", or a person"
			return invalid(rule.File, "rule", rule.ID, msg)
		}
	}

	return nil
}

// validateProposal checks a proposal's kind, text, proposer and verdict.
// Only a listed kind may be proposed; a goal is refused by name, since
// what we want is never proposed. A proposal is open, accepted or
// rejected, never two of those, and a rejection says why.
func validateProposal(proposal Proposal) error {
	kind := strings.TrimSpace(proposal.Proposes)
	if kind == "goal" {
		return invalid(proposal.File, "proposal", proposal.ID, "proposes: a goal is never proposed; what we want stays ours, propose how to get there")
	}
	if !slices.Contains(Proposable, kind) {
		return invalid(proposal.File, "proposal", proposal.ID, "proposes: must be one of "+strings.Join(Proposable, ", "))
	}

	if strings.TrimSpace(proposal.Proposal) == "" {
		return invalid(proposal.File, "proposal", proposal.ID, "proposal: text is required")
	}

	if strings.TrimSpace(proposal.By) == "" || strings.TrimSpace(proposal.When) == "" {
		return invalid(proposal.File, "proposal", proposal.ID, "needs by and when")
	}

	if proposal.Accepted != nil && proposal.Rejected != nil {
		return invalid(proposal.File, "proposal", proposal.ID, "is both accepted and rejected")
	}

	if proposal.Accepted != nil && !complete(proposal.Accepted) {
		return invalid(proposal.File, "proposal", proposal.ID, "accepted: needs who and when")
	}

	rejected := proposal.Rejected
	if rejected != nil && (strings.TrimSpace(rejected.Who) == "" || strings.TrimSpace(rejected.When) == "" || strings.TrimSpace(rejected.Why) == "") {
		return invalid(proposal.File, "proposal", proposal.ID, "rejected: needs who, when and why")
	}

	if strings.TrimSpace(proposal.Became) != "" && proposal.Accepted == nil {
		return invalid(proposal.File, "proposal", proposal.ID, "became: says what it was recorded as, and it is not accepted")
	}

	return nil
}

// validateGate checks a gate's kind, tier and sample. A gate on gate
// changes is refused: those always need a human, and writing one would
// let the system loosen its own gates.
func validateGate(gate Gate) error {
	kind := strings.TrimSpace(gate.Proposes)
	if kind == "gate" {
		return invalid(gate.File, "gate", gate.ID, "proposes: a gate change always needs a human; no gate covers it")
	}
	if !slices.Contains(Proposable, kind) {
		return invalid(gate.File, "gate", gate.ID, "proposes: must be one of "+strings.Join(Proposable, ", "))
	}

	if !slices.Contains(tiers, gate.Tier) {
		return invalid(gate.File, "gate", gate.ID, "tier: must be decide, observe or delegated")
	}

	if gate.Sample < 0 || (gate.Sample > 0 && gate.Tier != Observe) {
		return invalid(gate.File, "gate", gate.ID, "sample: audits about one in that many proposals, and only an observe gate audits")
	}

	for _, name := range gate.By {
		if strings.TrimSpace(name) == "" {
			return invalid(gate.File, "gate", gate.ID, "by: has an empty name")
		}
	}

	return nil
}

// resolveProposals checks what proposals and gates rest on, and that a
// proposal's became names something loaded.
func (s *Set) resolveProposals(owners map[string]string) error {
	for _, proposal := range s.Proposals {
		err := s.resolve(proposal.File, "proposal", proposal.ID, proposal.Because, false)
		if err != nil {
			return err
		}

		for _, evidenceID := range proposal.Evidence {
			_, exists := s.evidenceByID[evidenceID]
			if !exists {
				return invalid(proposal.File, "proposal", proposal.ID, "evidence names "+evidenceID+", which is not loaded evidence")
			}
		}

		_, exists := owners[proposal.Became]
		if proposal.Became != "" && !exists {
			return invalid(proposal.File, "proposal", proposal.ID, "became names "+proposal.Became+", which is not loaded")
		}
	}

	for _, gate := range s.Gates {
		err := s.resolve(gate.File, "gate", gate.ID, gate.Because, false)
		if err != nil {
			return err
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

		err = validateJudge(rule)
		if err != nil {
			return err
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
	for _, proposal := range s.Proposals {
		texts = append(texts, proposal.Proposal)
	}

	return texts
}
