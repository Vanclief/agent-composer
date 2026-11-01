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
	ID           string         `json:"id"`
	Text         string         `json:"text"`
	Who          string         `json:"who"`
	When         string         `json:"when"`
	TranslatedBy string         `json:"translated_by"`
	Retracted    *kb.Retraction `json:"retracted"`
}

// Reason is a belief, decision, rule or action (Kind), the evidence it
// cites, what it rests on, and its alternative sets of reasons in Or. A
// piece of evidence in an alternative is a Reason of kind evidence, with
// Who, When, TranslatedBy and Retracted set and nothing under it; those
// fields are empty on every other kind.
type Reason struct {
	ID           string           `json:"id"`
	Kind         string           `json:"kind"`
	Text         string           `json:"text"`
	Who          string           `json:"who,omitempty"`
	When         string           `json:"when,omitempty"`
	TranslatedBy string           `json:"translated_by,omitempty"`
	Retracted    *kb.Retraction   `json:"retracted,omitempty"`
	Evidence     []ReasonEvidence `json:"evidence"`
	Because      []Reason         `json:"because"`
	Or           [][]Reason       `json:"or"`
}

// Why follows a belief, decision, rule or action down its because chains,
// alternatives included, to the evidence.
func (api *API) Why(ctx context.Context, requester interface{}, request *WhyRequest) (*Reason, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	set, err := api.load()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	node, err := set.WhyAny(request.ID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	view := reasonView(node)

	return &view, nil
}

func reasonView(node kb.WhyNode) Reason {
	view := Reason{
		ID:       node.ID,
		Kind:     node.Kind,
		Text:     node.Text,
		Evidence: []ReasonEvidence{},
		Because:  []Reason{},
		Or:       [][]Reason{},
	}

	for _, evidence := range node.Evidence {
		view.Evidence = append(view.Evidence, ReasonEvidence{
			ID:           evidence.ID,
			Text:         evidence.Evidence,
			Who:          evidence.Who,
			When:         evidence.When,
			TranslatedBy: evidence.TranslatedBy,
			Retracted:    evidence.Retracted,
		})
	}

	for _, next := range node.Because {
		view.Because = append(view.Because, reasonView(next))
	}

	for _, alternative := range node.Or {
		nodes := []Reason{}
		for _, next := range alternative.Because {
			nodes = append(nodes, reasonView(next))
		}
		for _, evidence := range alternative.Evidence {
			nodes = append(nodes, Reason{
				ID:           evidence.ID,
				Kind:         "evidence",
				Text:         evidence.Evidence,
				Who:          evidence.Who,
				When:         evidence.When,
				TranslatedBy: evidence.TranslatedBy,
				Retracted:    evidence.Retracted,
				Evidence:     []ReasonEvidence{},
				Because:      []Reason{},
				Or:           [][]Reason{},
			})
		}
		view.Or = append(view.Or, nodes)
	}

	return view
}
