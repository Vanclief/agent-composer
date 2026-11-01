package knowledge

import (
	"github.com/vanclief/ez"
)

// Reason is a belief and the reasons it rests on. Leaves are observed.
type Reason struct {
	Belief  Belief   `json:"belief"`
	Because []Reason `json:"because,omitempty"`
}

// Why follows a belief down its because chain to the observations.
func (s *Set) Why(id string) (Reason, error) {
	belief, found := s.belief(id)
	if !found {
		return Reason{}, ez.New(ez.ENOTFOUND, "belief "+id+" not found", nil)
	}

	reason := Reason{Belief: belief}
	for _, next := range belief.Because {
		child, err := s.Why(next)
		if err != nil {
			return Reason{}, ez.Wrap(err)
		}
		reason.Because = append(reason.Because, child)
	}

	return reason, nil
}

// Doubted is what falls if a belief is wrong: everything resting on it,
// directly or through other beliefs and decisions. It is a listing for a
// human to recheck; nothing is changed.
type Doubted struct {
	Beliefs   []Belief   `json:"beliefs"`
	Decisions []Decision `json:"decisions"`
	Rules     []Rule     `json:"rules"`
}

// Doubt lists the beliefs and decisions that rest on the belief id, and
// the rules that rest on it or on any of them.
func (s *Set) Doubt(id string) (Doubted, error) {
	_, found := s.belief(id)
	if !found {
		return Doubted{}, ez.New(ez.ENOTFOUND, "belief "+id+" not found", nil)
	}

	// Mark whatever rests on a marked id until nothing more is added.
	falls := map[string]bool{id: true}
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
