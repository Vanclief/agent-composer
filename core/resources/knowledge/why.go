package knowledge

import (
	"context"
	"strings"

	"github.com/vanclief/ez"

	kb "github.com/vanclief/agent-composer/knowledge"
)

type WhyRequest struct {
	ID string `json:"id"`
}

func (r WhyRequest) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return ez.New(ez.EINVALID, "id is required", nil)
	}

	return nil
}

// ReasonEvidence is a piece of evidence a belief in a Reason cites.
type ReasonEvidence struct {
	ID        string         `json:"id"`
	Text      string         `json:"text"`
	Who       string         `json:"who"`
	When      string         `json:"when"`
	Retracted *kb.Retraction `json:"retracted"`
}

// Reason is a belief, the evidence it cites and the beliefs it rests on.
type Reason struct {
	ID       string           `json:"id"`
	Text     string           `json:"text"`
	Evidence []ReasonEvidence `json:"evidence"`
	Because  []Reason         `json:"because"`
}

// Why follows a belief down its because chain to the evidence.
func (api *API) Why(ctx context.Context, requester interface{}, request *WhyRequest) (*Reason, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	set, err := api.load()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	reason, err := set.Why(request.ID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	view := reasonView(reason)

	return &view, nil
}

func reasonView(reason kb.Reason) Reason {
	view := Reason{
		ID:       reason.Belief.ID,
		Text:     reason.Belief.Belief,
		Evidence: []ReasonEvidence{},
		Because:  []Reason{},
	}

	for _, evidence := range reason.Evidence {
		view.Evidence = append(view.Evidence, ReasonEvidence{
			ID:        evidence.ID,
			Text:      evidence.Evidence,
			Who:       evidence.Who,
			When:      evidence.When,
			Retracted: evidence.Retracted,
		})
	}

	for _, next := range reason.Because {
		view.Because = append(view.Because, reasonView(next))
	}

	return view
}
