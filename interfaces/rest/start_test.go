package rest

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/panel"
)

func TestSPADeepLinks(t *testing.T) {
	t.Parallel()

	e := echo.New()
	err := useSPA(e)
	if err != nil {
		t.Fatal(err)
	}

	paths := []string{
		"/",
		"/workflow",
		"/workflow/example",
		"/runs",
		"/runs/example",
	}
	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			e.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", response.Code, http.StatusOK)
			}
			contentType := response.Header().Get(echo.HeaderContentType)
			if !strings.Contains(contentType, echo.MIMETextHTML) {
				t.Errorf("content type = %q, want HTML", contentType)
			}
			if !strings.Contains(response.Body.String(), `<div id="root"></div>`) {
				t.Error("response does not contain the SPA root")
			}
		})
	}
}

func TestSPALegacyRedirects(t *testing.T) {
	t.Parallel()

	e := echo.New()
	err := useSPA(e)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name     string
		path     string
		location string
	}{
		{
			name:     "index",
			path:     "/index.html",
			location: "/",
		},
		{
			name:     "workflow",
			path:     "/workflow.html",
			location: "/",
		},
		{
			name:     "execution bookmark",
			path:     "/index.html?execution_id=%20abc-123%20",
			location: "/runs/abc-123",
		},
		{
			name:     "workflow bookmark",
			path:     "/workflow.html?id=example&run=abc-123",
			location: "/workflow/example?run=abc-123",
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(
				http.MethodGet,
				testCase.path,
				nil,
			)
			response := httptest.NewRecorder()
			e.ServeHTTP(response, request)

			if response.Code != http.StatusMovedPermanently {
				t.Errorf(
					"status = %d, want %d",
					response.Code,
					http.StatusMovedPermanently,
				)
			}
			location := response.Header().Get(echo.HeaderLocation)
			if location != testCase.location {
				t.Errorf(
					"location = %q, want %q",
					location,
					testCase.location,
				)
			}
		})
	}
}

func TestSPAAllowsAPIRoutesToTakePrecedence(t *testing.T) {
	t.Parallel()

	e := echo.New()
	e.GET("/api/probe", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]bool{"ok": true})
	})
	err := useSPA(e)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/probe", nil)
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", response.Code, http.StatusOK)
	}
	contentType := response.Header().Get(echo.HeaderContentType)
	if !strings.Contains(contentType, echo.MIMEApplicationJSON) {
		t.Errorf("content type = %q, want JSON", contentType)
	}
	body := strings.TrimSpace(response.Body.String())
	if body != `{"ok":true}` {
		t.Errorf("body = %q, want API response", body)
	}
}

func TestSPADoesNotRewriteMissingAPIRoutes(t *testing.T) {
	t.Parallel()

	e := echo.New()
	err := useSPA(e)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/missing", nil)
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if strings.Contains(response.Body.String(), `<div id="root"></div>`) {
		t.Error("missing API route was rewritten to the SPA")
	}
}

func TestPanel(t *testing.T) {
	t.Parallel()

	index := "<!doctype html><div id=\"root\"></div>"
	script := "console.log('panel')"
	dist := fstest.MapFS{
		".keep":                  {},
		"index.html":             {Data: []byte(index)},
		"assets/index-abc123.js": {Data: []byte(script)},
	}

	e := echo.New()
	e.GET("/api/tasks", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]bool{"ok": true})
	})
	err := useSPA(e)
	if err != nil {
		t.Fatal(err)
	}
	servePanel(e, dist)

	testCases := []struct {
		path     string
		status   int
		body     string
		location string
	}{
		{path: "/panel", status: http.StatusMovedPermanently, location: "/panel/"},
		{path: "/panel/", status: http.StatusOK, body: index},
		{path: "/panel/index.html", status: http.StatusOK, body: index},
		{path: "/panel/assets/index-abc123.js", status: http.StatusOK, body: script},
		{path: "/panel/assets", status: http.StatusOK, body: index},
		{path: "/panel/tasks/review-42/terminals", status: http.StatusOK, body: index},
		{path: "/panel/no/such/page", status: http.StatusOK, body: index},
		{path: "/api/tasks", status: http.StatusOK, body: `{"ok":true}` + "\n"},
	}
	for _, testCase := range testCases {
		request := httptest.NewRequest(http.MethodGet, testCase.path, nil)
		response := httptest.NewRecorder()
		e.ServeHTTP(response, request)

		if response.Code != testCase.status {
			t.Fatalf("%s: status = %d, want %d", testCase.path, response.Code, testCase.status)
		}
		if response.Body.String() != testCase.body && testCase.body != "" {
			t.Fatalf("%s: body is not the expected %d bytes, got %.80q", testCase.path, len(testCase.body), response.Body.String())
		}
		location := response.Header().Get(echo.HeaderLocation)
		if location != testCase.location {
			t.Fatalf("%s: location = %q, want %q", testCase.path, location, testCase.location)
		}
	}
}

func TestPanelNotBuilt(t *testing.T) {
	t.Parallel()

	// A fresh checkout embeds only dist/.keep.
	dist := fstest.MapFS{".keep": {}}

	e := echo.New()
	servePanel(e, dist)

	paths := []string{"/panel/", "/panel/tasks", "/panel/.keep"}
	for _, path := range paths {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		e.ServeHTTP(response, request)

		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: status = %d, want %d", path, response.Code, http.StatusServiceUnavailable)
		}
		body := response.Body.String()
		if body != "control panel not built: run make panel" {
			t.Fatalf("%s: body = %q", path, body)
		}
	}
}

func TestUsePanelServesEmbeddedBuild(t *testing.T) {
	t.Parallel()

	e := echo.New()
	err := usePanel(e)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/panel/", nil)
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)

	// Built: the React index. Not built (only dist/.keep embedded): 503.
	_, err = fs.Stat(panel.Dist, "dist/index.html")
	if err != nil {
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("unbuilt panel: status = %d, want %d", response.Code, http.StatusServiceUnavailable)
		}
		return
	}
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), `<div id="root"></div>`) {
		t.Fatalf("body is not the panel index: %.80q", response.Body.String())
	}
}
