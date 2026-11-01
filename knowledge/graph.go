package knowledge

import (
	"slices"

	"github.com/vanclief/ez"
)

// Reason is a belief, the evidence it cites and the reasons it rests on,
// with its alternatives.
type Reason struct {
	Belief   Belief        `json:"belief"`
	Evidence []Evidence    `json:"evidence,omitempty"`
	Because  []Reason      `json:"because,omitempty"`
	Or       []Alternative `json:"or,omitempty"`
}

// Alternative is one other set of reasons a belief can stand on.
type Alternative struct {
	Evidence []Evidence `json:"evidence,omitempty"`
	Because  []Reason   `json:"because,omitempty"`
}

// Why follows a belief down its because chains, alternatives included, to
// the evidence.
func (s *Set) Why(id string) (Reason, error) {
	belief, found := s.belief(id)
	if !found {
		return Reason{}, ez.New(ez.ENOTFOUND, "belief "+id+" not found", nil)
	}

	reason := Reason{Belief: belief}
	for i, set := range belief.Supports() {
		alternative, err := s.why(set)
		if err != nil {
			return Reason{}, ez.Wrap(err)
		}

		if i == 0 {
			reason.Evidence = alternative.Evidence
			reason.Because = alternative.Because
			continue
		}
		reason.Or = append(reason.Or, alternative)
	}

	return reason, nil
}

func (s *Set) why(set Support) (Alternative, error) {
	alternative := Alternative{}
	for _, evidenceID := range set.Evidence {
		alternative.Evidence = append(alternative.Evidence, s.evidenceByID[evidenceID])
	}

	for _, next := range set.Because {
		child, err := s.Why(next)
		if err != nil {
			return Alternative{}, ez.Wrap(err)
		}
		alternative.Because = append(alternative.Because, child)
	}

	return alternative, nil
}

// WhyNode is a belief, decision, rule or action, the evidence it cites
// (only beliefs cite evidence), the beliefs and decisions it rests on and
// its alternatives, each in Or.
type WhyNode struct {
	Kind     string     `json:"kind"`
	ID       string     `json:"id"`
	Text     string     `json:"text"`
	File     string     `json:"file"`
	Evidence []Evidence `json:"evidence,omitempty"`
	Because  []WhyNode  `json:"because,omitempty"`
	Or       []WhySet   `json:"or,omitempty"`
}

// WhySet is one other set of reasons a WhyNode can stand on.
type WhySet struct {
	Evidence []Evidence `json:"evidence,omitempty"`
	Because  []WhyNode  `json:"because,omitempty"`
}

// WhyAny is Why for any kind: it follows a belief, decision, rule or
// action down its because chains, alternatives included, to the evidence.
func (s *Set) WhyAny(id string) (WhyNode, error) {
	found, sets, exists := s.item(id)
	if !exists {
		return WhyNode{}, ez.New(ez.ENOTFOUND, "belief, decision, rule or action "+id+" not found", nil)
	}

	for i, set := range sets {
		alternative := WhySet{}
		for _, evidenceID := range set.Evidence {
			alternative.Evidence = append(alternative.Evidence, s.evidenceByID[evidenceID])
		}

		for _, next := range set.Because {
			child, err := s.WhyAny(next)
			if err != nil {
				return WhyNode{}, ez.Wrap(err)
			}
			alternative.Because = append(alternative.Because, child)
		}

		if i == 0 {
			found.Evidence = alternative.Evidence
			found.Because = alternative.Because
			continue
		}
		found.Or = append(found.Or, alternative)
	}

	return found, nil
}

// Kind says what id is: evidence, belief, decision, rule or action.
func (s *Set) Kind(id string) (string, bool) {
	_, isEvidence := s.evidenceByID[id]
	if isEvidence {
		return "evidence", true
	}

	found, _, exists := s.item(id)

	return found.Kind, exists
}

// item finds the belief, decision, rule or action id, with no reasons
// filled in, and its sets of reasons, its own first.
func (s *Set) item(id string) (WhyNode, []Support, bool) {
	for _, belief := range s.Beliefs {
		if belief.ID == id {
			return WhyNode{Kind: "belief", ID: id, Text: belief.Belief, File: belief.File}, belief.Supports(), true
		}
	}
	for _, decision := range s.Decisions {
		if decision.ID == id {
			return WhyNode{Kind: "decision", ID: id, Text: decision.Decision, File: decision.File}, decision.Supports(), true
		}
	}
	for _, ruleset := range s.Rulesets {
		for _, rule := range ruleset.Rules {
			if rule.ID == id {
				return WhyNode{Kind: "rule", ID: id, Text: rule.Rule, File: rule.File}, rule.Supports(), true
			}
		}
	}
	for _, action := range s.Actions {
		if action.ID == id {
			return WhyNode{Kind: "action", ID: id, Text: action.Action, File: action.File}, action.Supports(), true
		}
	}

	return WhyNode{}, nil, false
}

// Label says how a belief is grounded. It is computed from the evidence
// each time it is asked for and never stored.
type Label string

const (
	// Grounded beliefs have live evidence under every leaf of some set of
	// reasons.
	Grounded Label = "grounded"
	// Judgment beliefs rest, in some set of reasons, on a leaf that comes
	// from someone's experience and cites no evidence, and on no opinion
	// or doubted leaf: expert testimony, stronger than an opinion and
	// weaker than evidence.
	Judgment Label = "judgment"
	// Opinion beliefs have, in every set of reasons, a leaf with no
	// evidence at all.
	Opinion Label = "opinion"
	// Contested beliefs are grounded or opinions, and something grounded
	// contradicts them or the evidence they cite.
	Contested Label = "contested"
	// InDoubt beliefs have, in every set of reasons, a leaf whose evidence
	// is all retracted or set aside.
	InDoubt Label = "in doubt"
)

// Status is how a belief, or whatever rests on some beliefs and decisions,
// is grounded. Each set of reasons is judged on the leaves under it: the
// beliefs that rest on no other belief. The thing is grounded if one set
// is. Otherwise the reasons of every set are listed, and Label is the
// first that applies of in doubt, contested, opinion, judgment; all
// reasons are kept whatever the label.
type Status struct {
	Label Label `json:"label"`
	// Opinions are the leaves with no evidence and no experience.
	Opinions []string `json:"opinions,omitempty"`
	// Judgments are the leaves with no evidence that come from someone's
	// experience.
	Judgments []string `json:"judgments,omitempty"`
	// Doubted are the beliefs whose evidence is all retracted or set
	// aside, or that are set aside themselves. Retracted is the retracted
	// evidence under them and Without the ids set aside with Set.Without.
	Doubted   []string `json:"doubted,omitempty"`
	Retracted []string `json:"retracted,omitempty"`
	Without   []string `json:"without,omitempty"`
	// ContestedBy are the grounded beliefs and live evidence that
	// contradict the belief or the evidence it cites.
	ContestedBy []string `json:"contested_by,omitempty"`
}

// Status computes the status of the belief id.
func (s *Set) Status(id string) (Status, error) {
	_, found := s.belief(id)
	if !found {
		return Status{}, ez.New(ez.ENOTFOUND, "belief "+id+" not found", nil)
	}

	status := s.judge(id)
	if status.Label == InDoubt {
		return status, nil
	}

	contestedBy := s.contestedBy(id)
	if len(contestedBy) > 0 {
		status.ContestedBy = contestedBy
		status.Label = Contested
	}

	return status, nil
}

// StatusOf computes the status of what rests on the sets of reasons in
// sets, such as a rule, a decision or an action: grounded if one set is.
// A set with no because has no leaves, so it is grounded.
func (s *Set) StatusOf(sets []Support) Status {
	return s.judgeSets("", sets, false)
}

// judge computes how the belief or decision id is grounded, leaving
// contradictions out.
func (s *Set) judge(id string) Status {
	if s.without[id] {
		return Status{Label: InDoubt, Doubted: []string{id}, Without: []string{id}}
	}

	found := s.nodes[id]

	return s.judgeSets(id, found.sets, found.kind == "belief")
}

func (s *Set) judgeSets(owner string, sets []Support, isBelief bool) Status {
	merged := Status{}
	for _, set := range sets {
		status := s.judgeSet(owner, set, isBelief)
		if status.Label == Grounded {
			return Status{Label: Grounded}
		}
		merge(&merged, status)
	}
	merged.Label = labelFor(merged)

	return merged
}

// judgeSet judges one set of reasons: all of it must hold. A belief's set
// with no because and no evidence is a judgment when the belief comes from
// someone's experience, and an opinion otherwise.
func (s *Set) judgeSet(owner string, set Support, isBelief bool) Status {
	status := Status{}
	leaf := isBelief && len(set.Because) == 0 && len(set.Evidence) == 0
	switch {
	case leaf && s.nodes[owner].judged:
		status.Judgments = []string{owner}
	case leaf:
		status.Opinions = []string{owner}
	}

	if len(set.Evidence) > 0 && !slices.ContainsFunc(set.Evidence, s.live) {
		status.Doubted = []string{owner}
		for _, evidenceID := range set.Evidence {
			if s.without[evidenceID] {
				status.Without = appendNew(status.Without, evidenceID)
				continue
			}
			status.Retracted = appendNew(status.Retracted, evidenceID)
		}
	}

	for _, next := range set.Because {
		merge(&status, s.judge(next))
	}
	status.Label = labelFor(status)

	return status
}

// live says whether a piece of evidence grounds anything: it is neither
// retracted nor set aside.
func (s *Set) live(evidenceID string) bool {
	return s.evidenceByID[evidenceID].Retracted == nil && !s.without[evidenceID]
}

func merge(into *Status, from Status) {
	for _, id := range from.Opinions {
		into.Opinions = appendNew(into.Opinions, id)
	}
	for _, id := range from.Judgments {
		into.Judgments = appendNew(into.Judgments, id)
	}
	for _, id := range from.Doubted {
		into.Doubted = appendNew(into.Doubted, id)
	}
	for _, id := range from.Retracted {
		into.Retracted = appendNew(into.Retracted, id)
	}
	for _, id := range from.Without {
		into.Without = appendNew(into.Without, id)
	}
}

func labelFor(status Status) Label {
	switch {
	case len(status.Doubted) > 0:
		return InDoubt
	case len(status.Opinions) > 0:
		return Opinion
	case len(status.Judgments) > 0:
		return Judgment
	}

	return Grounded
}

func appendNew(list []string, id string) []string {
	if slices.Contains(list, id) {
		return list
	}

	return append(list, id)
}

// stands says whether id can contest something: live evidence, or a
// grounded belief.
func (s *Set) stands(id string) bool {
	_, isEvidence := s.evidenceByID[id]
	if isEvidence {
		return s.live(id)
	}

	return s.judge(id).Label == Grounded
}

// contestedBy lists what stands against the belief id: the beliefs and
// evidence contradicting it, or contradicting live evidence it cites.
func (s *Set) contestedBy(id string) []string {
	targets := []string{id}
	for _, set := range s.nodes[id].sets {
		for _, evidenceID := range set.Evidence {
			if s.live(evidenceID) {
				targets = appendNew(targets, evidenceID)
			}
		}
	}

	found := []string{}
	for _, target := range targets {
		for _, other := range s.contradicts[target] {
			if other != id && s.stands(other) {
				found = appendNew(found, other)
			}
		}
	}

	return found
}

// Without returns the same set judged as if the evidence and beliefs in
// ids were retracted or absent: a what-if. Nothing is written, and s is
// unchanged.
func (s *Set) Without(ids []string) (*Set, error) {
	view := *s
	view.without = map[string]bool{}
	for _, id := range ids {
		_, isBelief := s.belief(id)
		_, isEvidence := s.evidenceByID[id]
		if !isBelief && !isEvidence {
			return nil, ez.New(ez.ENOTFOUND, "without: "+id+" is not a loaded belief or evidence", nil)
		}
		view.without[id] = true
	}

	return &view, nil
}

// Doubted is what falls if a belief or a piece of evidence is wrong:
// everything resting on it, directly or through other beliefs and
// decisions, in every set of its reasons. Standing lists what rests on it
// in some sets but still has another. It is a listing for a human to
// recheck; nothing is changed.
type Doubted struct {
	Beliefs   []Belief   `json:"beliefs"`
	Decisions []Decision `json:"decisions"`
	Rules     []Rule     `json:"rules"`
	Actions   []Action   `json:"actions"`
	Standing  []Standing `json:"standing"`
}

// Standing is a belief, decision, rule or open action that rests on the
// doubted id in some of its sets of reasons and not in Other.
type Standing struct {
	Kind  string    `json:"kind"`
	ID    string    `json:"id"`
	Text  string    `json:"text"`
	File  string    `json:"file"`
	Other []Support `json:"other"`
}

// Doubt lists the beliefs and decisions that rest on id, the rules and
// open actions that rest on it or on any of them, and, apart, those that
// rest on it but have another set of reasons that does not. id is a
// belief, a decision, a rule (nothing rests on a rule, so its listing is
// empty) or a piece of evidence, in which case the beliefs citing it come
// first.
func (s *Set) Doubt(id string) (Doubted, error) {
	kind, _ := s.Kind(id)
	if !slices.Contains([]string{"belief", "evidence", "decision", "rule"}, kind) {
		return Doubted{}, ez.New(ez.ENOTFOUND, "belief, evidence, decision or rule "+id+" not found", nil)
	}

	falls := map[string]bool{id: true}
	reaches := func(set Support) bool {
		return slices.Contains(set.Evidence, id) || slices.ContainsFunc(set.Because, func(next string) bool {
			return falls[next]
		})
	}

	// Mark whatever rests on a marked id in every set until nothing more
	// is added.
	added := true
	for added {
		added = false
		for _, next := range s.nodeIDs() {
			if falls[next] {
				continue
			}
			sets := s.nodes[next].sets
			if !slices.ContainsFunc(sets, func(set Support) bool { return !reaches(set) }) {
				falls[next] = true
				added = true
			}
		}
	}

	// other lists the sets that do not reach id, or nil when every one
	// does, and says whether any does.
	other := func(sets []Support) ([]Support, bool) {
		rest := []Support{}
		for _, set := range sets {
			if !reaches(set) {
				rest = append(rest, set)
			}
		}

		return rest, len(rest) < len(sets)
	}

	doubted := Doubted{Beliefs: []Belief{}, Decisions: []Decision{}, Rules: []Rule{}, Actions: []Action{}, Standing: []Standing{}}
	for _, belief := range s.Beliefs {
		rest, rests := other(belief.Supports())
		switch {
		case belief.ID == id:
			// The doubted belief itself is not listed.
		case falls[belief.ID]:
			doubted.Beliefs = append(doubted.Beliefs, belief)
		case rests:
			doubted.Standing = append(doubted.Standing, Standing{Kind: "belief", ID: belief.ID, Text: belief.Belief, File: belief.File, Other: rest})
		}
	}
	for _, decision := range s.Decisions {
		rest, rests := other(decision.Supports())
		switch {
		case decision.ID == id:
			// The doubted decision itself is not listed.
		case falls[decision.ID]:
			doubted.Decisions = append(doubted.Decisions, decision)
		case rests:
			doubted.Standing = append(doubted.Standing, Standing{Kind: "decision", ID: decision.ID, Text: decision.Decision, File: decision.File, Other: rest})
		}
	}
	for _, ruleset := range s.Rulesets {
		for _, rule := range ruleset.Rules {
			rest, rests := other(rule.Supports())
			switch {
			case rests && len(rest) == 0:
				doubted.Rules = append(doubted.Rules, rule)
			case rests:
				doubted.Standing = append(doubted.Standing, Standing{Kind: "rule", ID: rule.ID, Text: rule.Rule, File: rule.File, Other: rest})
			}
		}
	}
	for _, action := range s.Actions {
		if action.Done != nil {
			continue
		}

		rest, rests := other(action.Supports())
		switch {
		case rests && len(rest) == 0:
			doubted.Actions = append(doubted.Actions, action)
		case rests:
			doubted.Standing = append(doubted.Standing, Standing{Kind: "action", ID: action.ID, Text: action.Action, File: action.File, Other: rest})
		}
	}

	return doubted, nil
}

// Citing lists the beliefs that cite the piece of evidence id, in any set.
func (s *Set) Citing(id string) []Belief {
	citing := []Belief{}
	for _, belief := range s.Beliefs {
		cites := slices.ContainsFunc(belief.Supports(), func(set Support) bool {
			return slices.Contains(set.Evidence, id)
		})
		if cites {
			citing = append(citing, belief)
		}
	}

	return citing
}

// Side is one side of a contradiction: a belief or a piece of evidence,
// with the live evidence and the opinions it comes down to. A piece of
// evidence comes down to itself.
type Side struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"`
	Evidence []Evidence `json:"evidence"`
	Opinions []string   `json:"opinions"`
}

// Conflict is a contradiction where at least one side is contested: it is
// not in doubt, and the other side is a grounded belief or live evidence.
type Conflict struct {
	A Side `json:"a"`
	B Side `json:"b"`
}

// Conflicts lists every contested contradiction, in the order they are
// declared, each once. It never picks a side.
func (s *Set) Conflicts() []Conflict {
	pairs := [][2]string{}
	for _, evidence := range s.Evidence {
		for _, other := range evidence.Contradicts {
			pairs = append(pairs, [2]string{evidence.ID, other})
		}
	}
	for _, belief := range s.Beliefs {
		for _, other := range belief.Contradicts {
			pairs = append(pairs, [2]string{belief.ID, other})
		}
	}

	conflicts := []Conflict{}
	seen := map[[2]string]bool{}
	for _, pair := range pairs {
		if seen[pair] || seen[[2]string{pair[1], pair[0]}] {
			continue
		}
		seen[pair] = true

		contested := (s.holds(pair[0]) && s.stands(pair[1])) || (s.holds(pair[1]) && s.stands(pair[0]))
		if contested {
			conflicts = append(conflicts, Conflict{A: s.side(pair[0]), B: s.side(pair[1])})
		}
	}

	return conflicts
}

// holds says whether id can be contested: live evidence, or a belief not
// in doubt.
func (s *Set) holds(id string) bool {
	_, isEvidence := s.evidenceByID[id]
	if isEvidence {
		return s.live(id)
	}

	return s.judge(id).Label != InDoubt
}

func (s *Set) side(id string) Side {
	evidence, isEvidence := s.evidenceByID[id]
	if isEvidence {
		return Side{ID: id, Kind: "evidence", Evidence: []Evidence{evidence}, Opinions: []string{}}
	}

	side := Side{ID: id, Kind: "belief", Evidence: []Evidence{}, Opinions: []string{}}
	s.leavesOf(id, &side, map[string]bool{})

	return side
}

// leavesOf adds the live evidence and the opinions under the belief id, in
// every set of its reasons, to side.
func (s *Set) leavesOf(id string, side *Side, seen map[string]bool) {
	if seen[id] {
		return
	}
	seen[id] = true

	for _, set := range s.nodes[id].sets {
		if len(set.Because) == 0 && len(set.Evidence) == 0 {
			side.Opinions = appendNew(side.Opinions, id)
		}

		for _, evidenceID := range set.Evidence {
			evidence := s.evidenceByID[evidenceID]
			if s.live(evidenceID) && !slices.ContainsFunc(side.Evidence, func(e Evidence) bool { return e.ID == evidenceID }) {
				side.Evidence = append(side.Evidence, evidence)
			}
		}

		for _, next := range set.Because {
			s.leavesOf(next, side, seen)
		}
	}
}

func (s *Set) belief(id string) (Belief, bool) {
	for _, belief := range s.Beliefs {
		if belief.ID == id {
			return belief, true
		}
	}

	return Belief{}, false
}
