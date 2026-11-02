package rest

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

func writeVolumeFile(t *testing.T, root string, rel string, body string, modified time.Time) {
	t.Helper()

	full := filepath.Join(root, filepath.FromSlash(rel))
	err := os.MkdirAll(filepath.Dir(full), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(full, []byte(body), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Chtimes(full, modified, modified)
	if err != nil {
		t.Fatal(err)
	}
}

// The volume directory under the temp root stands in for msb's; nothing
// goes through a VM, so the runtime is never asked.
func TestSandboxTranscriptRoutes(t *testing.T) {
	routes := newTaskRoutes(t, sandboxes.SandboxRuntime{}, "")

	owner := &taskmodels.Task{Title: "Review 42", Name: "review-42", Brief: "b"}
	err := owner.Insert(context.Background(), routes.db)
	if err != nil {
		t.Fatal(err)
	}

	for _, sb := range []*taskmodels.Sandbox{
		{TaskID: owner.ID, Name: "review-42", Spec: []byte(`{}`), Volume: "agc-review-42"},
		{TaskID: owner.ID, Name: "old-box", Spec: []byte(`{}`)},
	} {
		err = sb.Insert(context.Background(), routes.db)
		if err != nil {
			t.Fatal(err)
		}
	}

	volume := filepath.Join(routes.volumes, "agc-review-42")
	older := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	claude := ".claude/projects/-home-agent-agent-composer/0190a7c2.jsonl"
	codex := ".codex/sessions/2026/11/02/rollout-1.jsonl"
	writeVolumeFile(t, volume, claude, `{"type":"user"}`+"\n", older)
	writeVolumeFile(t, volume, codex, `{"type":"session_meta"}`+"\n", newer)
	writeVolumeFile(t, volume, ".claude/settings.json", `{}`, newer)
	writeVolumeFile(t, volume, ".claude/projects/x/notes.txt", "not a transcript", newer)

	// A secret outside the volume, and links to it the guest could plant.
	secret := filepath.Join(routes.volumes, "secret.jsonl")
	writeVolumeFile(t, routes.volumes, "secret.jsonl", "host secret", newer)
	for _, link := range []string{".claude/projects/x/escape.jsonl", ".codex/sessions/inside.jsonl"} {
		target := secret
		if strings.HasSuffix(link, "inside.jsonl") {
			target = filepath.Join(volume, filepath.FromSlash(claude))
		}
		err = os.Symlink(target, filepath.Join(volume, filepath.FromSlash(link)))
		if err != nil {
			t.Fatal(err)
		}
	}

	// A deleted sandbox: no row, its volume left behind.
	writeVolumeFile(t, filepath.Join(routes.volumes, "agc-gone"), ".claude/projects/p/s.jsonl", `{"gone":true}`+"\n", older)

	routes.check(t, []routeCase{
		// Newest first; links and non-transcripts are not listed.
		{http.MethodGet, "/api/sandboxes/review-42/transcripts", "", http.StatusOK,
			`{"volume":"agc-review-42","transcripts":[{"path":"` + codex + `","size":24,"modified_at":"2026-11-02T10:00:00Z"},{"path":"` + claude + `","size":16,"modified_at":"2026-11-02T09:00:00Z"}]}`},
		{http.MethodGet, "/api/sandboxes/review-42/transcripts/" + claude, "", http.StatusOK, `{"type":"user"}`},
		{http.MethodGet, "/api/sandboxes/review-42/transcripts/" + codex, "", http.StatusOK, `{"type":"session_meta"}`},
		{http.MethodGet, "/api/sandboxes/gone/transcripts", "", http.StatusOK, `"volume":"agc-gone","transcripts":[{"path":".claude/projects/p/s.jsonl"`},
		{http.MethodGet, "/api/sandboxes/gone/transcripts/.claude/projects/p/s.jsonl", "", http.StatusOK, `{"gone":true}`},
		{http.MethodGet, "/api/sandboxes/never/transcripts", "", http.StatusNotFound, "no volume agc-never is left"},
		{http.MethodGet, "/api/sandboxes/old-box/transcripts", "", http.StatusNotFound, "booted before sandboxes had one"},
		{http.MethodGet, "/api/sandboxes/review-42/transcripts/.claude/projects/x/missing.jsonl", "", http.StatusNotFound, "not found on volume agc-review-42"},
		// Confined to the transcript directories of the volume.
		{http.MethodGet, "/api/sandboxes/review-42/transcripts/.claude/settings.json", "", http.StatusBadRequest, "is not under"},
		{http.MethodGet, "/api/sandboxes/review-42/transcripts/.claude/projects/%2e%2e/%2e%2e/%2e%2e/secret.jsonl", "", http.StatusBadRequest, "without .."},
		{http.MethodGet, "/api/sandboxes/review-42/transcripts/.claude/projects/..%2f..%2f..%2fsecret.jsonl", "", http.StatusBadRequest, "without .."},
		{http.MethodGet, "/api/sandboxes/review-42/transcripts/%2Fetc%2Fpasswd", "", http.StatusBadRequest, "without .."},
		{http.MethodGet, "/api/sandboxes/review-42/transcripts/.claude/projects//x/escape.jsonl", "", http.StatusBadRequest, "without .."},
		{http.MethodGet, "/api/sandboxes/review-42/transcripts/.claude/projects/x/escape.jsonl", "", http.StatusBadRequest, "not a regular file"},
		{http.MethodGet, "/api/sandboxes/review-42/transcripts/.codex/sessions/inside.jsonl", "", http.StatusBadRequest, "not a regular file"},
		{http.MethodGet, "/api/sandboxes/..%2fagc-gone/transcripts", "", http.StatusNotFound, ""},
	})

	status, body := routes.do(http.MethodGet, "/api/sandboxes/review-42/transcripts/.claude/projects/x/escape.jsonl", "")
	if status == http.StatusOK || strings.Contains(body, "host secret") {
		t.Fatalf("a link led out of the volume: %d %s", status, body)
	}
}
