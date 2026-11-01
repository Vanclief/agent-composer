package knowledge

import (
	"context"

	"github.com/vanclief/ez"
)

type ListRequest struct{}

func (r ListRequest) Validate() error {
	return nil
}

// KnowledgeSet is everything in the knowledge directory, in load order.
type KnowledgeSet struct {
	Dir       string     `json:"dir"`
	Evidence  []Evidence `json:"evidence"`
	Beliefs   []Belief   `json:"beliefs"`
	Decisions []Decision `json:"decisions"`
	Rules     []Rule     `json:"rules"`
}

func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*KnowledgeSet, error) {
	set, err := api.load()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	response := &KnowledgeSet{
		Dir:       api.dir,
		Evidence:  []Evidence{},
		Decisions: decisionViews(set, set.Decisions),
		Rules:     []Rule{},
	}

	for _, evidence := range set.Evidence {
		response.Evidence = append(response.Evidence, evidenceView(set, evidence))
	}

	response.Beliefs, err = beliefViews(set, set.Beliefs)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	for _, ruleset := range set.Rulesets {
		for _, rule := range ruleset.Rules {
			response.Rules = append(response.Rules, ruleView(set, ruleset.Subject, rule))
		}
	}

	return response, nil
}
