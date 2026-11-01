package task

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vanclief/ez"
)

const savedYAML = "# reviews the backend\ntitle: Backend review\nrepo: github.com/o/n\n"

func TestStoreSaveGetListDelete(t *testing.T) {
	store := Store{Dir: filepath.Join(t.TempDir(), "config", "specs")}

	list, err := store.List()
	if err != nil || len(list) != 0 {
		t.Fatalf("missing dir: got %+v, err %v", list, err)
	}

	saved, err := store.Save("backend-review", savedYAML)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Name != "backend-review" || saved.YAML != savedYAML {
		t.Fatalf("saved = %+v", saved)
	}

	info, err := os.Stat(filepath.Join(store.Dir, "backend-review.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v", info.Mode().Perm())
	}

	got, err := store.Get("backend-review")
	if err != nil || got.YAML != savedYAML {
		t.Fatalf("get: got %+v, err %v", got, err)
	}

	_, err = store.Save("api-review", "image: debian\n")
	if err != nil {
		t.Fatal(err)
	}

	// Files that are not saved specs are left out of the list.
	for _, other := range []string{"notes.txt", "Bad Name.yaml"} {
		err = os.WriteFile(filepath.Join(store.Dir, other), []byte("x"), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}

	list, err = store.List()
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, spec := range list {
		names = append(names, spec.Name)
	}
	if !reflect.DeepEqual(names, []string{"api-review", "backend-review"}) {
		t.Fatalf("names = %q", names)
	}

	err = store.Delete("backend-review")
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Get("backend-review")
	if ez.ErrorCode(err) != ez.ENOTFOUND || ez.ErrorMessage(err) != "saved spec backend-review not found" {
		t.Fatalf("get after delete: expected ENOTFOUND, got %v", err)
	}

	err = store.Delete("backend-review")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("delete missing: expected ENOTFOUND, got %v", err)
	}
}

func TestStoreSaveRejectsBadInput(t *testing.T) {
	store := Store{Dir: t.TempDir()}

	cases := []struct {
		name string
		yaml string
		want string
	}{
		{"Bad Name", "image: debian\n", "is invalid"},
		{"ok", "image: debian\ncpu: 2\n", "cpu"},
		{"ok", "ref: master\n", "ref is set but repo is not"},
		{"ok", "repo: [unclosed\n", ""},
	}

	for _, tc := range cases {
		_, err := store.Save(tc.name, tc.yaml)
		if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), tc.want) {
			t.Fatalf("Save(%q, %q): expected EINVALID containing %q, got %v", tc.name, tc.yaml, tc.want, err)
		}
	}

	entries, err := os.ReadDir(store.Dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("rejected saves left files: %v, err %v", entries, err)
	}
}
