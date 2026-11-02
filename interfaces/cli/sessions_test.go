package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSessionStore(t *testing.T) {
	dir := t.TempDir()

	// A missing file is an empty store, without an error.
	store, err := loadSessionStore(dir)
	if err != nil || store.Get("main") != "" {
		t.Fatalf("missing file: %v, %q", err, store.Get("main"))
	}

	err = store.Set(threadSessionKey("#dm", 10), "s1")
	if err != nil {
		t.Fatal(err)
	}

	// Another loop (the task mode) writes its key after this one loaded.
	other, _ := loadSessionStore(dir)
	err = other.Set(taskSessionKey("fix-login"), "t1")
	if err != nil {
		t.Fatal(err)
	}

	err = store.Set(mainSessionKey, "m1")
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]string{}
	readJSONFile(t, filepath.Join(dir, sessionsFile), &got)
	want := map[string]string{"#dm:10": "s1", "task:fix-login": "t1", "main": "m1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("file = %v", got)
	}

	reloaded, err := loadSessionStore(dir)
	if err != nil || reloaded.Get("task:fix-login") != "t1" || reloaded.Get("#dm:10") != "s1" {
		t.Fatalf("reloaded: %v, %v", err, reloaded.sessions)
	}

	// A corrupt file is an empty store with an error to log, and the next
	// Set replaces it.
	err = os.WriteFile(filepath.Join(dir, sessionsFile), []byte("{not json"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	corrupt, err := loadSessionStore(dir)
	if err == nil || corrupt == nil || corrupt.Get("main") != "" {
		t.Fatalf("corrupt file: %v, %+v", err, corrupt)
	}

	err = corrupt.Set(mainSessionKey, "m2")
	if err != nil {
		t.Fatal(err)
	}

	got = map[string]string{}
	readJSONFile(t, filepath.Join(dir, sessionsFile), &got)
	if !reflect.DeepEqual(got, map[string]string{"main": "m2"}) {
		t.Fatalf("after corrupt: %v", got)
	}
}
