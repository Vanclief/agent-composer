package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/vanclief/agent-composer/core/resources/skills"
)

// Skills repository (ADR 0008 "Skills").

// ListSkills lists the skills with their descriptions.
func (c *Client) ListSkills(ctx context.Context) (*skills.ListResponse, error) {
	out := &skills.ListResponse{}
	err := c.do(ctx, http.MethodGet, "/api/skills", nil, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// GetSkill reads the skill's SKILL.md, served as markdown.
func (c *Client) GetSkill(ctx context.Context, name string) (*skills.SkillText, error) {
	data, err := c.send(ctx, http.MethodGet, "/api/skills/"+url.PathEscape(name), nil, "text/markdown")
	if err != nil {
		return nil, err
	}

	return &skills.SkillText{Name: name, Text: string(data)}, nil
}
