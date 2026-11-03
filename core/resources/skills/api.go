// Package skills serves the skills repository read only: the skills a
// sandbox spec may name, which agc writes into every sandbox it prepares
// (package skills, docs/decisions/0008-tasks-allocate-compute.md
// "Skills"). The directory is read on every request, no cache.
package skills

import (
	"context"

	"github.com/vanclief/ez"

	repo "github.com/vanclief/agent-composer/skills"
)

type API struct {
	repo repo.Repo
}

// NewAPI returns the skills API over dir, which holds <name>/SKILL.md.
func NewAPI(dir string) *API {
	return &API{repo: repo.Repo{Dir: dir}}
}

// Dir is the skills directory.
func (api *API) Dir() string {
	return api.repo.Dir
}

// ListRequest lists the skills.
type ListRequest struct{}

func (r ListRequest) Validate() error {
	return nil
}

// ListResponse is every skill with its description, sorted by name.
type ListResponse struct {
	Skills []repo.Skill `json:"skills"`
}

func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	list, err := api.repo.List()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &ListResponse{Skills: list}, nil
}

// GetRequest names a skill; the name comes from the path.
type GetRequest struct {
	Name string `json:"-"`
}

func (r GetRequest) Validate() error {
	if r.Name == "" {
		return ez.New(ez.EINVALID, "name is required", nil)
	}

	return nil
}

// SkillText is a skill's SKILL.md.
type SkillText struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// Get returns the skill's SKILL.md, ENOTFOUND when it is absent.
func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*SkillText, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	text, err := api.repo.Text(request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &SkillText{Name: request.Name, Text: text}, nil
}
