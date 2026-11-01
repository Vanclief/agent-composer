package knowledge

import (
	"slices"

	"github.com/vanclief/ez"
)

// Reason is a belief, the evidence it cites and the reasons it rests on.
type Reason struct {
	Belief   Belief     `json:"belief"`
	Evidence []Evidence `json:"evidence,omitempty"`
	Because  []Reason   `json:"because,omitempty"`
}

// Why follows a belief down its because chain to the evidence.
func (s *Set) Why(id string) (Reason, error) {
	belief, found := s.belief(id)
	if !found {
		return Reason{}, ez.New(ez.ENOTFOUND, "belief "+id+" not found", nil)
	}

	reason := Reason{Belief: belief}
	for _, evidenceID := range belief.Evidence {
		reason.Evidence = append(reason.Evidence, s.evidenceByID[evidenceID])
	}

	for _, next := range belief.Because {
		child, err := s.Why(next)
		if err != nil {
			return Reason{}, ez.Wrap(err)
		}
		reason.Because = append(reason.Because, child)
	}

	return reason, nil
}

// Label says how a belief is grounded. It is computed from the evidence
// each time it is asked for and never stored.
type Label string

const (
	// Grounded beliefs have live evidence under every leaf.
	Grounded Label = "grounded"
	// Opinion beliefs have a leaf with no evidence at all.
	Opinion Label = "opinion"
	// InDoubt beliefs have a leaf whose evidence is all retracted.
	InDoubt Label = "in doubt"
)

// Status is how a belief, or whatever rests on some beliefs and decisions,
// is grounded. It is judged on the leaves of the because tree: the beliefs
// that rest on no other belief. When some leaves are opinions and others
// in doubt, Label is InDoubt and both lists are filled.
type Status struct {
	Label Label `json:"label"`
	// Opinions are the leaves with no evidence.
	Opinions []string `json:"opinions,omitempty"`
	// Doubted are the leaves whose evidence is all retracted, and
	// Retracted is that evidence.
	Doubted   []string `json:"doubted,omitempty"`
	Retracted []string `json:"retracted,omitempty"`
}

// Status computes the status of the belief id.
func (s *Set) Status(id string) (Status, error) {
	_, found := s.belief(id)
	if !found {
		return Status{}, ez.New(ez.ENOTFOUND, "belief "+id+" not found", nil)
	}

	return s.StatusOf([]string{id}), nil
}

// StatusOf computes the status of what rests on the beliefs and decisions
// in because, such as a rule or a decision. An empty because has no
// leaves, so it is grounded.
func (s *Set) StatusOf(because []string) Status {
	status := Status{Label: Grounded}
	for _, leaf := range s.leaves(because, map[string]bool{}) {
		cited := s.nodes[leaf].evidence
		live := 0
		for _, evidenceID := range cited {
			if s.evidenceByID[evidenceID].Retracted == nil {
				live++
			}
		}

		switch {
		case len(cited) == 0:
			status.Opinions = append(status.Opinions, leaf)
		case live == 0:
			status.Doubted = append(status.Doubted, leaf)
			for _, evidenceID := range cited {
				if !slices.Contains(status.Retracted, evidenceID) {
					status.Retracted = append(status.Retracted, evidenceID)
				}
			}
		}
	}

	switch {
	case len(status.Doubted) > 0:
		status.Label = InDoubt
	case len(status.Opinions) > 0:
		status.Label = Opinion
	}

	return status
}

// leaves lists the beliefs under ids that rest on nothing else, depth first
// and each once.
func (s *Set) leaves(ids []string, seen map[string]bool) []string {
	found := []string{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true

		next := s.nodes[id]
		if next.kind == "belief" && len(next.because) == 0 {
			found = append(found, id)
			continue
		}
		found = append(found, s.leaves(next.because, seen)...)
	}

	return found
}

// Doubted is what falls if a belief or a piece of evidence is wrong:
// everything resting on it, directly or through other beliefs and
// decisions. It is a listing for a human to recheck; nothing is changed.
type Doubted struct {
	Beliefs   []Belief   `json:"beliefs"`
	Decisions []Decision `json:"decisions"`
	Rules     []Rule     `json:"rules"`
}

// Doubt lists the beliefs and decisions that rest on id, and the rules that
// rest on it or on any of them. id is a belief, or a piece of evidence, in
// which case the beliefs citing it come first.
func (s *Set) Doubt(id string) (Doubted, error) {
	falls := map[string]bool{}
	_, isBelief := s.belief(id)
	_, isEvidence := s.evidenceByID[id]
	switch {
	case isBelief:
		falls[id] = true
	case isEvidence:
		for _, belief := range s.Citing(id) {
			falls[belief.ID] = true
		}
	default:
		return Doubted{}, ez.New(ez.ENOTFOUND, "belief or evidence "+id+" not found", nil)
	}

	// Mark whatever rests on a marked id until nothing more is added.
	added := true
	for added {
		added = false
		for _, next := range s.nodeIDs() {
			if !falls[next] && restsOn(s.nodes[next].because, falls) {
				falls[next] = true
				added = true
			}
		}
	}

	doubted := Doubted{Beliefs: []Belief{}, Decisions: []Decision{}, Rules: []Rule{}}
	for _, belief := range s.Beliefs {
		if falls[belief.ID] && belief.ID != id {
			doubted.Beliefs = append(doubted.Beliefs, belief)
		}
	}
	for _, decision := range s.Decisions {
		if falls[decision.ID] {
			doubted.Decisions = append(doubted.Decisions, decision)
		}
	}
	for _, ruleset := range s.Rulesets {
		for _, rule := range ruleset.Rules {
			if restsOn(rule.Because, falls) {
				doubted.Rules = append(doubted.Rules, rule)
			}
		}
	}

	return doubted, nil
}

// Citing lists the beliefs that cite the piece of evidence id.
func (s *Set) Citing(id string) []Belief {
	citing := []Belief{}
	for _, belief := range s.Beliefs {
		if slices.Contains(belief.Evidence, id) {
			citing = append(citing, belief)
		}
	}

	return citing
}

func restsOn(because []string, falls map[string]bool) bool {
	for _, id := range because {
		if falls[id] {
			return true
		}
	}

	return false
}

func (s *Set) belief(id string) (Belief, bool) {
	for _, belief := range s.Beliefs {
		if belief.ID == id {
			return belief, true
		}
	}

	return Belief{}, false
}
