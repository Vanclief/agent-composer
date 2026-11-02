package triggers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/core/controller"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// testToken is a fixture value written to a temporary secrets file.
const testToken = "test-token-not-a-secret"

type post struct {
	Channel string
	Body    string
}

type fakePoster struct {
	mu    sync.Mutex
	posts []post
}

func (f *fakePoster) PostAsSystem(ctx context.Context, channel, body string) (*msgmodels.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.posts = append(f.posts, post{Channel: channel, Body: body})
	return &msgmodels.Message{Channel: channel, Body: body}, nil
}

func (f *fakePoster) take() []post {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := f.posts
	f.posts = nil
	return out
}

func ts(t0 time.Time, d time.Duration) string {
	return t0.Add(d).UTC().Format(time.RFC3339)
}

// fakeGitHub serves one repo, acme/app, with one event of every kind
// after t0, one before the window, and one by an ignored user.
func fakeGitHub(t *testing.T, t0 time.Time) *httptest.Server {
	t.Helper()

	base := "https://github.com/acme/app"
	api := "/repos/acme/app"

	pull12 := map[string]any{
		"number": 12, "title": "Add DMs", "html_url": base + "/pull/12",
		"updated_at": ts(t0, 40*time.Second), "head": map[string]any{"ref": "agc/dms-whatsapp"},
	}
	pull13 := map[string]any{
		"number": 13, "title": "Old work", "html_url": base + "/pull/13",
		"updated_at": ts(t0, 50*time.Second), "head": map[string]any{"ref": "agc/gone"},
		"merged_at": ts(t0, 50*time.Second), "merged_by": map[string]any{"login": "franco"},
	}

	routes := map[string]any{
		api + "/issues/comments": []any{
			map[string]any{
				"id": 1, "user": map[string]any{"login": "franco"}, "body": "Please   rebase\non master",
				"html_url": base + "/pull/12#issuecomment-1", "issue_url": "https://api/repos/acme/app/issues/12",
				"created_at": ts(t0, 10*time.Second),
			},
			map[string]any{
				"id": 2, "user": map[string]any{"login": "ana"}, "body": strings.Repeat("x", 500),
				"html_url": base + "/issues/7#issuecomment-2", "issue_url": "https://api/repos/acme/app/issues/7",
				"created_at": ts(t0, 20*time.Second),
			},
			map[string]any{
				"id": 3, "user": map[string]any{"login": "Dependabot"}, "body": "bump",
				"html_url": base + "/issues/7#issuecomment-3", "issue_url": "https://api/repos/acme/app/issues/7",
				"created_at": ts(t0, 21*time.Second),
			},
			map[string]any{
				"id": 4, "user": map[string]any{"login": "ana"}, "body": "edited long ago",
				"html_url": base + "/issues/7#issuecomment-4", "issue_url": "https://api/repos/acme/app/issues/7",
				"created_at": ts(t0, -48*time.Hour),
			},
		},
		api + "/pulls/comments": []any{
			map[string]any{
				"id": 5, "user": map[string]any{"login": "ana"}, "body": "nit", "path": "main.go",
				"html_url": base + "/pull/13#discussion_r5", "pull_request_url": "https://api/repos/acme/app/pulls/13",
				"created_at": ts(t0, 30*time.Second),
			},
		},
		api + "/pulls":    []any{pull13, pull12},
		api + "/pulls/12": pull12,
		api + "/pulls/13": pull13,
		api + "/pulls/12/reviews": []any{
			map[string]any{
				"id": 6, "user": map[string]any{"login": "ana"}, "body": "", "state": "APPROVED",
				"html_url": base + "/pull/12#pullrequestreview-6", "submitted_at": ts(t0, 35*time.Second),
			},
			map[string]any{
				"id": 7, "user": map[string]any{"login": "ana"}, "body": "", "state": "COMMENTED",
				"html_url": base + "/pull/12#pullrequestreview-7", "submitted_at": ts(t0, 36*time.Second),
			},
		},
		api + "/pulls/13/reviews": []any{},
		api + "/issues": []any{
			map[string]any{
				"number": 8, "title": "Crash on start", "body": "It panics", "user": map[string]any{"login": "ana"},
				"html_url": base + "/issues/8", "created_at": ts(t0, 5*time.Second),
			},
			map[string]any{
				"number": 13, "title": "Old work", "user": map[string]any{"login": "bob"},
				"html_url": base + "/pull/13", "created_at": ts(t0, -72*time.Hour),
				"closed_at": ts(t0, 50*time.Second), "pull_request": map[string]any{"merged_at": ts(t0, 50*time.Second)},
			},
		},
		api + "/actions/runs": map[string]any{
			"workflow_runs": []any{
				map[string]any{
					"id": 99, "name": "CI", "display_title": "Add DMs", "head_branch": "agc/dms-whatsapp",
					"status": "completed", "conclusion": "failure", "run_attempt": 1,
					"html_url": base + "/actions/runs/99", "updated_at": ts(t0, 45*time.Second),
					"triggering_actor": map[string]any{"login": "franco"},
					"pull_requests":    []any{map[string]any{"number": 12}},
				},
			},
		},
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testToken {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
			return
		}

		body, ok := routes[r.URL.Path]
		if !ok {
			t.Errorf("unexpected GitHub request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if r.URL.Query().Get("page") != "" && r.URL.Query().Get("page") != "1" {
			body = []any{}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func writeSecrets(t *testing.T, dir, content string) string {
	t.Helper()

	path := filepath.Join(dir, "secrets.env")
	err := os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	return path
}

func newTestAPI(t *testing.T, server *httptest.Server, poster *fakePoster, secretsPath, statePath string, clock *time.Time) *API {
	t.Helper()

	settings := controller.GitHubTriggers{Repos: []string{"acme/app"}, IgnoreUsers: []string{"dependabot"}}
	tasks := func(ctx context.Context, name string) (bool, error) {
		return name == "dms-whatsapp", nil
	}

	api := New(settings, poster, tasks, secretsPath, statePath)
	api.base = server.URL
	api.client = server.Client()
	api.now = func() time.Time {
		return *clock
	}

	return api
}

func TestPollPostsEachEventOnceAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	server := fakeGitHub(t, t0)
	defer server.Close()

	secretsPath := writeSecrets(t, dir, "GITHUB_TOKEN="+testToken+"\n")
	statePath := StatePath(dir)
	poster := &fakePoster{}
	clock := t0
	api := newTestAPI(t, server, poster, secretsPath, statePath, &clock)
	ctx := context.Background()

	// The first poll only sets the cursors: no history is posted.
	api.PollOnce(ctx)
	got := poster.take()
	if len(got) != 0 {
		t.Fatalf("first poll posted %d messages, want none", len(got))
	}

	clock = t0.Add(time.Minute)
	api.PollOnce(ctx)
	got = poster.take()

	want := []post{
		{"#main", "[github] ana opened issue #8 in acme/app: Crash on start: It panics\nhttps://github.com/acme/app/issues/8"},
		{"#main", "[github] franco merged PR #13 (agc/gone) in acme/app: Old work\nhttps://github.com/acme/app/pull/13"},
		{"#dms-whatsapp", "[github] franco commented on PR #12 (agc/dms-whatsapp) in acme/app: Please rebase on master\nhttps://github.com/acme/app/pull/12#issuecomment-1"},
		{"#main", "[github] ana commented on issue #7 in acme/app: " + strings.Repeat("x", MaxText) + "...\nhttps://github.com/acme/app/issues/7#issuecomment-2"},
		{"#main", "[github] ana commented on main.go in PR #13 (agc/gone) in acme/app: nit\nhttps://github.com/acme/app/pull/13#discussion_r5"},
		{"#dms-whatsapp", "[github] ana approved PR #12 (agc/dms-whatsapp) in acme/app\nhttps://github.com/acme/app/pull/12#pullrequestreview-6"},
		{"#dms-whatsapp", "[github] franco ran workflow \"CI\" on PR #12 (agc/dms-whatsapp): failure in acme/app: Add DMs\nhttps://github.com/acme/app/actions/runs/99"},
	}

	if len(got) != len(want) {
		for _, p := range got {
			t.Logf("%s %q", p.Channel, p.Body)
		}
		t.Fatalf("second poll posted %d messages, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("post %d =\n  %s %q\nwant\n  %s %q", i, got[i].Channel, got[i].Body, want[i].Channel, want[i].Body)
		}
	}

	// The same events again, within the overlap: nothing posts twice.
	clock = t0.Add(2 * time.Minute)
	api.PollOnce(ctx)
	got = poster.take()
	if len(got) != 0 {
		t.Fatalf("third poll posted %d messages, want none", len(got))
	}

	// A restart reads the cursor file and posts nothing twice either.
	restarted := newTestAPI(t, server, poster, secretsPath, statePath, &clock)
	clock = t0.Add(3 * time.Minute)
	restarted.PollOnce(ctx)
	got = poster.take()
	if len(got) != 0 {
		t.Fatalf("poll after a restart posted %d messages, want none", len(got))
	}

	views, err := restarted.Show()
	if err != nil {
		t.Fatal(err)
	}

	if len(views) != len(GitHubKinds) {
		t.Fatalf("show listed %d sources, want %d", len(views), len(GitHubKinds))
	}

	posted := 0
	for _, view := range views {
		if !view.Configured || view.Cursor == nil || !view.Cursor.Equal(clock) || view.LastError != "" {
			t.Errorf("source %+v, want configured, cursor %s and no error", view, clock)
		}
		posted += view.Posted
	}

	if posted != len(want) {
		t.Errorf("show counted %d posted, want %d", posted, len(want))
	}
}

func TestPollWithoutTokenRecordsTheError(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	server := fakeGitHub(t, t0)
	defer server.Close()

	poster := &fakePoster{}
	clock := t0
	api := newTestAPI(t, server, poster, writeSecrets(t, dir, "OTHER=1\n"), StatePath(dir), &clock)

	api.PollOnce(context.Background())

	views, err := api.Show()
	if err != nil {
		t.Fatal(err)
	}

	for _, view := range views {
		if view.Cursor != nil {
			t.Errorf("source %s has a cursor without a token", view.Source)
		}
		if !strings.Contains(view.LastError, "GITHUB_TOKEN is not set") {
			t.Errorf("source %s last_error = %q, want it to name GITHUB_TOKEN", view.Source, view.LastError)
		}
	}
}

func TestPollRejectedTokenKeepsTheCursor(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	server := fakeGitHub(t, t0)
	defer server.Close()

	secretsPath := writeSecrets(t, dir, "GITHUB_TOKEN="+testToken+"\n")
	poster := &fakePoster{}
	clock := t0
	api := newTestAPI(t, server, poster, secretsPath, StatePath(dir), &clock)
	ctx := context.Background()

	api.PollOnce(ctx)

	writeSecrets(t, dir, "GITHUB_TOKEN=wrong\n")
	clock = t0.Add(time.Minute)
	api.PollOnce(ctx)

	got := poster.take()
	if len(got) != 0 {
		t.Fatalf("a rejected poll posted %d messages", len(got))
	}

	views, err := api.Show()
	if err != nil {
		t.Fatal(err)
	}

	for _, view := range views {
		if view.Cursor == nil || !view.Cursor.Equal(t0) {
			t.Errorf("source %s cursor = %v, want it kept at %s", view.Source, view.Cursor, t0)
		}
		if !strings.Contains(view.LastError, "401") || strings.Contains(view.LastError, "wrong") {
			t.Errorf("source %s last_error = %q, want the status without the token", view.Source, view.LastError)
		}
	}

	// The token works again: the events of the failed minute post now.
	writeSecrets(t, dir, "GITHUB_TOKEN="+testToken+"\n")
	clock = t0.Add(2 * time.Minute)
	api.PollOnce(ctx)

	got = poster.take()
	if len(got) != 7 {
		t.Fatalf("the poll after the failure posted %d messages, want 7", len(got))
	}
}

func TestRoute(t *testing.T) {
	api := New(controller.GitHubTriggers{}, nil, func(ctx context.Context, name string) (bool, error) {
		return name == "done-task", nil
	}, "", filepath.Join(t.TempDir(), "c.json"))
	ctx := context.Background()

	cases := map[string]string{
		"agc/done-task": "#done-task",
		"agc/missing":   "#main",
		"feature/x":     "#main",
		"":              "#main",
		"agc/Bad Name":  "#main",
		"agc/":          "#main",
	}

	for branch, want := range cases {
		got := api.Route(ctx, Event{Branch: branch})
		if got != want {
			t.Errorf("Route(%q) = %s, want %s", branch, got, want)
		}
	}
}

func TestCut(t *testing.T) {
	got := Cut("  a\n\n b  ", 10)
	if got != "a b" {
		t.Errorf("Cut folded to %q", got)
	}

	got = Cut("héllo wörld", 5)
	if got != "héllo..." {
		t.Errorf("Cut = %q, want héllo...", got)
	}
}
