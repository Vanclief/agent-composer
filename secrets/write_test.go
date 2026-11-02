package secrets

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSetCreatesFileOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "secrets.env")

	store, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	err = store.Set("ACCOUNT_WORK", "made-up value")
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %04o, want 0600", info.Mode().Perm())
	}

	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := again.Lookup("ACCOUNT_WORK")
	if !ok || value != "made-up value" {
		t.Fatalf("reloaded value is not the one written")
	}
}

func TestSetAndRemoveKeepOtherLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.env")
	original := "# my secrets\nGITHUB_TOKEN=gh-made-up\n\nexport ACCOUNT_OLD=x\n"
	err := os.WriteFile(path, []byte(original), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	store, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	values := map[string]string{
		"ACCOUNT_A": `it's`,
		"ACCOUNT_B": `say "hi"`,
		"ACCOUNT_C": "plain=with=equals",
	}
	for name, value := range values {
		err = store.Set(name, value)
		if err != nil {
			t.Fatal(err)
		}
	}

	err = store.Remove("ACCOUNT_OLD")
	if err != nil {
		t.Fatal(err)
	}

	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range values {
		got, ok := again.Lookup(name)
		if !ok || got != want {
			t.Fatalf("%s did not round-trip", name)
		}
	}
	if again.Has("ACCOUNT_OLD") || !again.Has("GITHUB_TOKEN") {
		t.Fatalf("names = %v", again.Names())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "# my secrets\nGITHUB_TOKEN=gh-made-up\n\n") {
		t.Fatalf("other lines changed:\n%s", data)
	}

	// Replacing keeps one line.
	err = again.Set("ACCOUNT_A", "second")
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "ACCOUNT_A=") != 1 {
		t.Fatalf("ACCOUNT_A is set more than once")
	}
}

func TestSetRefusesValuesLoadCannotRead(t *testing.T) {
	store, err := Load(filepath.Join(t.TempDir(), "secrets.env"))
	if err != nil {
		t.Fatal(err)
	}

	cases := []string{"", " padded", "two\nlines", `both ' and "`}
	for _, value := range cases {
		err = store.Set("ACCOUNT_X", value)
		if err == nil {
			t.Fatalf("value %q was accepted", value)
		}
		if value != "" && strings.Contains(err.Error(), value) {
			t.Fatalf("the error spells the value")
		}
	}
}
