package rest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/vanclief/compose/components/ratelimit"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/skills"
	"github.com/vanclief/agent-composer/interfaces/rest/handler"
	restserver "github.com/vanclief/agent-composer/interfaces/rest/server"
)

// newSkillRoutes serves the API routes with dir as the skills directory.
func newSkillRoutes(t *testing.T, dir string) *echo.Echo {
	t.Helper()

	server := &restserver.Server{
		Ctrl:        &controller.Controller{},
		RateLimiter: ratelimit.NewWindowCounter(10, 100),
		SkillsAPI:   skills.NewAPI(dir),
	}

	e := echo.New()
	addAPIRoutes(e, handler.NewHandler(server))

	return e
}

// shippedSkill is the repo's skills/rtsdesign/SKILL.md.
func shippedSkill(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "skills", "rtsdesign", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func TestSkillRoutes(t *testing.T) {
	dir := t.TempDir()
	shipped := shippedSkill(t)
	files := map[string]string{
		"rtsdesign/SKILL.md": shipped,
		"bare/SKILL.md":      "No frontmatter.\n",
		"Not_A_Skill/x.md":   "ignored\n",
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		err := os.MkdirAll(filepath.Dir(path), 0o755)
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile(path, []byte(content), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}

	e := newSkillRoutes(t, dir)

	var list struct {
		Skills []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"skills"`
	}
	decodeKnowledge(t, getKnowledge(t, e, "/api/skills", http.StatusOK), &list)

	names := []string{}
	for _, skill := range list.Skills {
		names = append(names, skill.Name)
	}
	if !reflect.DeepEqual(names, []string{"bare", "rtsdesign"}) {
		t.Fatalf("names = %q", names)
	}
	if list.Skills[0].Description != "" {
		t.Fatalf("bare description = %q", list.Skills[0].Description)
	}
	description := list.Skills[1].Description
	if !strings.HasPrefix(description, "Design UI the way strategy and factory games do") || !strings.HasSuffix(description, "no medieval or sci-fi styling.") || strings.Contains(description, "\n") {
		t.Fatalf("rtsdesign description = %q", description)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/skills/rtsdesign", nil)
	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != shipped {
		t.Fatalf("GET rtsdesign: %d %.80q", recorder.Code, recorder.Body.String())
	}
	if !strings.HasPrefix(recorder.Header().Get(echo.HeaderContentType), "text/markdown") {
		t.Fatalf("content type = %q", recorder.Header().Get(echo.HeaderContentType))
	}

	getKnowledge(t, e, "/api/skills/missing", http.StatusNotFound)
	getKnowledge(t, e, "/api/skills/Not_A_Skill", http.StatusNotFound)
}

func TestSkillRoutesMissingDir(t *testing.T) {
	e := newSkillRoutes(t, filepath.Join(t.TempDir(), "missing"))

	body := getKnowledge(t, e, "/api/skills", http.StatusOK)
	if strings.TrimSpace(string(body)) != `{"skills":[]}` {
		t.Fatalf("body = %s", body)
	}

	getKnowledge(t, e, "/api/skills/rtsdesign", http.StatusNotFound)
}
