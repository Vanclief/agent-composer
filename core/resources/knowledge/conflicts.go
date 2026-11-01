package knowledge

import (
	"context"

	"github.com/vanclief/ez"

	kb "github.com/vanclief/agent-composer/knowledge"
)

type ConflictsRequest struct{}

func (r ConflictsRequest) Validate() error {
	return nil
}

// ConflictSet is every contested contradiction, in the order declared.
type ConflictSet struct {
	Conflicts []Conflict `json:"conflicts"`
}

// Conflict is a contradiction where at least one side is contested.
// ARestsOn and BRestsOn are the leaves under each side: the live evidence,
// then the opinions. A piece of evidence rests on itself. Nothing picks a
// side.
type Conflict struct {
	A        ConflictSide `json:"a"`
	B        ConflictSide `json:"b"`
	ARestsOn []string     `json:"a_rests_on"`
	BRestsOn []string     `json:"b_rests_on"`
}

// ConflictSide is a belief or a piece of evidence (Kind) in a conflict.
type ConflictSide struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// Conflicts lists every contested contradiction with what each side rests
// on.
func (api *API) Conflicts(ctx context.Context, requester interface{}, request *ConflictsRequest) (*ConflictSet, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	set, err := api.load()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	texts := map[string]string{}
	for _, evidence := range set.Evidence {
		texts[evidence.ID] = evidence.Evidence
	}
	for _, belief := range set.Beliefs {
		texts[belief.ID] = belief.Belief
	}

	response := &ConflictSet{Conflicts: []Conflict{}}
	for _, conflict := range set.Conflicts() {
		response.Conflicts = append(response.Conflicts, Conflict{
			A:        ConflictSide{ID: conflict.A.ID, Kind: conflict.A.Kind, Text: texts[conflict.A.ID]},
			B:        ConflictSide{ID: conflict.B.ID, Kind: conflict.B.Kind, Text: texts[conflict.B.ID]},
			ARestsOn: restsOn(conflict.A),
			BRestsOn: restsOn(conflict.B),
		})
	}

	return response, nil
}

func restsOn(side kb.Side) []string {
	leaves := []string{}
	for _, evidence := range side.Evidence {
		leaves = append(leaves, evidence.ID)
	}

	return append(leaves, side.Opinions...)
}
