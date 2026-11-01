package rest

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	controlpanel "github.com/vanclief/agent-composer/control_panel"
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

	index, err := fs.ReadFile(controlpanel.FS, "index.html")
	if err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	e.GET("/api/tasks", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]bool{"ok": true})
	})
	err = useSPA(e)
	if err != nil {
		t.Fatal(err)
	}
	usePanel(e)

	testCases := []struct {
		path     string
		status   int
		body     string
		location string
	}{
		{path: "/panel", status: http.StatusMovedPermanently, location: "/panel/"},
		{path: "/panel/", status: http.StatusOK, body: string(index)},
		{path: "/panel/index.html", status: http.StatusOK, body: string(index)},
		{path: "/panel/no/such/page", status: http.StatusOK, body: string(index)},
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
