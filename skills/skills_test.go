package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanclief/ez"
)

func TestDescription(t *testing.T) {
	cases := map[string]string{
		"---\nname: a\ndescription: One line.\n---\nBody\n": "One line.",
		"---\ndescription: >-\n  Folded\n  text.\n---\n":    "Folded text.",
		"---\r\ndescription: CRLF.\r\n---\r\n":              "CRLF.",
		"No frontmatter.\n":                                 "",
		"---\nname: a\n---\n":                               "",
		"---\ndescription: [unclosed\n---\n":                "",
		"---\ndescription: never closed\n":                  "",
	}
	for text, want := range cases {
		got := Description(text)
		if got != want {
			t.Errorf("Description(%q) = %q, want %q", text, got, want)
		}
	}
}

// The shipped rtsdesign skill is in the repository with a description.
func TestShippedSkills(t *testing.T) {
	repo := Repo{Dir: "."}

	list, err := repo.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "rtsdesign" || !strings.HasPrefix(list[0].Description, "Design UI the way") {
		t.Fatalf("list = %+v", list)
	}

	files, err := repo.Files("rtsdesign")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != FileName {
		t.Fatalf("files = %+v", files)
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"beta", "alpha"} {
		err := os.MkdirAll(filepath.Join(dir, name), 0o755)
		if err != nil {
			t.Fatal(err)
		}
	}
	err := os.WriteFile(filepath.Join(dir, "file"), nil, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	repo := Repo{Dir: dir}

	err = repo.Check([]string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}

	err = repo.Check([]string{"alpha", "file"})
	if err == nil || ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), `skill "file"`) || !strings.Contains(ez.ErrorMessage(err), "known skills: alpha, beta") {
		t.Fatalf("err = %v", err)
	}

	err = Repo{}.Check([]string{"alpha"})
	if err == nil || !strings.Contains(ez.ErrorMessage(err), "known skills: none") {
		t.Fatalf("empty repo err = %v", err)
	}

	_, err = repo.Text("alpha")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("text of a skill with no SKILL.md: %v", err)
	}
}
