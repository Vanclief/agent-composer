package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillCommandsAgainstAServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/skills":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"skills":[{"name":"rtsdesign","description":"Design UI like games. Use when asked."}]}`))
		case "GET /api/skills/rtsdesign":
			w.Header().Set("Content-Type", "text/markdown")
			_, _ = w.Write([]byte("---\nname: rtsdesign\n---\nBody.\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"skill nope not found"}}`))
		}
	}))
	defer server.Close()

	out, err := runAgc(t, "skill", "list", "--server", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "rtsdesign  Design UI like games." {
		t.Fatalf("list = %q", out)
	}

	out, err = runAgc(t, "skill", "show", "rtsdesign", "--server", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if out != "---\nname: rtsdesign\n---\nBody.\n" {
		t.Fatalf("show = %q", out)
	}

	_, err = runAgc(t, "skill", "show", "nope", "--server", server.URL)
	if err == nil {
		t.Fatal("show nope succeeded")
	}
}

func TestSkillCommandsLocally(t *testing.T) {
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, "rtsdesign"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(dir, "rtsdesign", "SKILL.md"), []byte("---\ndescription: Games.\n---\nBody.\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGC_SKILLS_DIR", dir)
	t.Setenv("AGC_SERVER", "")

	out, err := runAgc(t, "skill", "list")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "rtsdesign  Games." {
		t.Fatalf("list = %q", out)
	}

	out, err = runAgc(t, "skill", "show", "rtsdesign")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out, "Body.\n") {
		t.Fatalf("show = %q", out)
	}
}
