package knowledge

import (
	"context"
	"strings"

	"github.com/vanclief/ez"
)

// ListRequest lists the knowledge directory. Without, when set, judges
// everything as if those evidence and belief ids were retracted or absent:
// a what-if, nothing is written.
type ListRequest struct {
	Without []string `json:"without"`
}

func (r ListRequest) Validate() error {
	for _, id := range r.Without {
		if strings.TrimSpace(id) == "" {
			return ez.New(ez.EINVALID, "without names an empty id", nil)
		}
	}

	return nil
}

// KnowledgeSet is everything in the knowledge directory, in load order.
// Without echoes the ids set aside, [] when none.
type KnowledgeSet struct {
	Dir       string     `json:"dir"`
	Without   []string   `json:"without"`
	Evidence  []Evidence `json:"evidence"`
	Beliefs   []Belief   `json:"beliefs"`
	Decisions []Decision `json:"decisions"`
	Rules     []Rule     `json:"rules"`
	Actions   []Action   `json:"actions"`
}

func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*KnowledgeSet, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	set, err := api.load()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if len(request.Without) > 0 {
		set, err = set.Without(request.Without)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	response := &KnowledgeSet{
		Dir:       api.dir,
		Without:   list(request.Without),
		Evidence:  []Evidence{},
		Decisions: decisionViews(set, set.Decisions),
		Rules:     []Rule{},
		Actions:   actionViews(set, set.Actions),
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
