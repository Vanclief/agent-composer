package secrets

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/vanclief/ez"
)

const secretValue = "sk-very-secret-value-123"

func writeSecrets(t *testing.T, content string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secrets.env")
	err := os.WriteFile(path, []byte(content), mode)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(path, mode)
	if err != nil {
		t.Fatal(err)
	}

	return path
}

func TestDefaultPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGENT_COMPOSER_HOME", home)

	path, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "config", "secrets.env")
	if path != want {
		t.Fatalf("DefaultPath() = %q, want %q", path, want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	store, err := Load(filepath.Join(t.TempDir(), "secrets.env"))
	if err != nil {
		t.Fatal(err)
	}
	if len(store.Names()) != 0 {
		t.Fatalf("Names() = %v, want empty", store.Names())
	}
}

func TestLoadRejectsOpenMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode check is skipped on windows")
	}
	path := writeSecrets(t, "GITHUB_TOKEN="+secretValue+"\n", 0o644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() succeeded on a 0644 file")
	}
	if ez.ErrorCode(err) != ez.ENOTAUTHORIZED {
		t.Fatalf("code = %q, want %q", ez.ErrorCode(err), ez.ENOTAUTHORIZED)
	}
	msg := err.Error()
	if !strings.Contains(msg, path) || !strings.Contains(msg, "0644") || !strings.Contains(msg, "chmod 600") {
		t.Fatalf("message %q should name the path, the mode and chmod 600", msg)
	}
	if strings.Contains(msg, secretValue) {
		t.Fatalf("message %q leaks the secret value", msg)
	}
}

func TestLoadParses(t *testing.T) {
	content := strings.Join([]string{
		"# a comment",
		"",
		"PLAIN=plain-value",
		"  SPACED  =   spaced value  ",
		`DOUBLE="double # quoted"`,
		`SINGLE='single'`,
		`MISMATCHED="half'`,
		"export EXPORTED=exported",
		"WITH_EQUALS=a=b=c",
		"EMPTY=",
		"DUP=first",
		"DUP=last",
	}, "\n")
	path := writeSecrets(t, content, 0o600)

	store, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"PLAIN":       "plain-value",
		"SPACED":      "spaced value",
		"DOUBLE":      "double # quoted",
		"SINGLE":      "single",
		"MISMATCHED":  `"half'`,
		"EXPORTED":    "exported",
		"WITH_EQUALS": "a=b=c",
		"EMPTY":       "",
		"DUP":         "last",
	}
	if !reflect.DeepEqual(store.values, want) {
		t.Fatalf("values = %v, want %v", store.values, want)
	}
}

func TestLoadBadLine(t *testing.T) {
	path := writeSecrets(t, "# header\nGOOD=1\n"+secretValue+"\n", 0o600)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() accepted a line without =")
	}
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("code = %q, want %q", ez.ErrorCode(err), ez.EINVALID)
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("message %q should name line 3", err.Error())
	}
	if strings.Contains(err.Error(), secretValue) {
		t.Fatalf("message %q leaks the line content", err.Error())
	}
}

func TestNamesSorted(t *testing.T) {
	path := writeSecrets(t, "ZETA=1\nALPHA=2\nMID=3\n", 0o600)

	store, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ALPHA", "MID", "ZETA"}
	if !reflect.DeepEqual(store.Names(), want) {
		t.Fatalf("Names() = %v, want %v", store.Names(), want)
	}
}

func TestParseRequestValid(t *testing.T) {
	cases := map[string]Request{
		"GITHUB_TOKEN":                              {Name: "GITHUB_TOKEN", Env: "GITHUB_TOKEN"},
		" A1_B ":                                    {Name: "A1_B", Env: "A1_B"},
		"STRIPE_KEY@api.stripe.com":                 {Name: "STRIPE_KEY", Env: "STRIPE_KEY", Hosts: []string{"api.stripe.com"}},
		"X@a.example.com, *.example.org":            {Name: "X", Env: "X", Hosts: []string{"a.example.com", "*.example.org"}},
		"LOCAL@localhost":                           {Name: "LOCAL", Env: "LOCAL", Hosts: []string{"localhost"}},
		"ANTHROPIC_WORK as CLAUDE_CODE_OAUTH_TOKEN": {Name: "ANTHROPIC_WORK", Env: "CLAUDE_CODE_OAUTH_TOKEN"},
		" OPENAI_PERSONAL  as  CODEX_API_KEY@api.openai.com ": {Name: "OPENAI_PERSONAL", Env: "CODEX_API_KEY", Hosts: []string{"api.openai.com"}},
		"A as _PRIVATE_1@x.example.com,y.example.com":         {Name: "A", Env: "_PRIVATE_1", Hosts: []string{"x.example.com", "y.example.com"}},
	}
	for input, want := range cases {
		got, err := ParseRequest(input)
		if err != nil {
			t.Fatalf("ParseRequest(%q): %v", input, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ParseRequest(%q) = %+v, want %+v", input, got, want)
		}
	}
}

func TestParseRequestInvalid(t *testing.T) {
	cases := map[string]string{
		"":                        `""`,
		"github_token":            "github_token",
		"1TOKEN":                  "1TOKEN",
		"TOKEN-X":                 "TOKEN-X",
		"TOKEN@":                  `""`,
		"TOKEN@https://api.x.com": "https://api.x.com",
		"TOKEN@api.x.com/v1":      "api.x.com/v1",
		"TOKEN@api.x.com:443":     "api.x.com:443",
		"TOKEN@API.X.COM":         "API.X.COM",
		"TOKEN@a.com,,b.com":      `""`,
		"TOKEN@foo.*.com":         "foo.*.com",
		"TOKEN@-bad.com":          "-bad.com",
		"TOKEN@a.com@b.com":       "a.com@b.com",
		"WORK as claude_token":    `"WORK as claude_token"`,
		"WORK as 1TOKEN":          `"WORK as 1TOKEN"`,
		"WORK as TOKEN-X@a.com":   `"WORK as TOKEN-X@a.com"`,
		"WORK as":                 `"WORK as"`,
		"WORK AS TOKEN":           `"WORK AS TOKEN"`,
		"WORK as A B":             `"WORK as A B"`,
		"work as TOKEN":           "work",
		"WORK as TOKEN@HOST.COM":  "HOST.COM",
	}
	for input, badPart := range cases {
		_, err := ParseRequest(input)
		if err == nil {
			t.Fatalf("ParseRequest(%q) succeeded", input)
		}
		if ez.ErrorCode(err) != ez.EINVALID {
			t.Fatalf("ParseRequest(%q) code = %q, want %q", input, ez.ErrorCode(err), ez.EINVALID)
		}
		if !strings.Contains(err.Error(), badPart) {
			t.Fatalf("ParseRequest(%q) message %q should name %s", input, err.Error(), badPart)
		}
	}
}

func loadStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := writeSecrets(t, "GITHUB_TOKEN="+secretValue+"\nCUSTOM="+secretValue+"\n", 0o600)
	store, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	return store, path
}

func TestResolve(t *testing.T) {
	store, _ := loadStore(t)

	got, err := store.Resolve([]Request{
		{Name: "GITHUB_TOKEN"},
		{Name: "CUSTOM", Hosts: []string{"api.custom.dev"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []Binding{
		{Name: "GITHUB_TOKEN", Env: "GITHUB_TOKEN", Value: secretValue, Hosts: []string{"api.github.com", "github.com"}},
		{Name: "CUSTOM", Env: "CUSTOM", Value: secretValue, Hosts: []string{"api.custom.dev"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Resolve() = %+v, want %+v", got, want)
	}

	got[0].Hosts[0] = "mutated"
	if KnownHosts["GITHUB_TOKEN"][0] != "api.github.com" {
		t.Fatal("Resolve() returned a slice aliasing KnownHosts")
	}
}

func TestResolveExplicitHostsOverrideKnown(t *testing.T) {
	store, _ := loadStore(t)

	got, err := store.Resolve([]Request{{Name: "GITHUB_TOKEN", Hosts: []string{"ghe.example.com"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[0].Hosts, []string{"ghe.example.com"}) {
		t.Fatalf("Hosts = %v, want [ghe.example.com]", got[0].Hosts)
	}
}

func TestResolveUnknownNameWithoutHosts(t *testing.T) {
	store, _ := loadStore(t)

	_, err := store.Resolve([]Request{{Name: "CUSTOM"}})
	if err == nil {
		t.Fatal("Resolve() succeeded without hosts")
	}
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("code = %q, want %q", ez.ErrorCode(err), ez.EINVALID)
	}
	if !strings.Contains(err.Error(), "CUSTOM@host") {
		t.Fatalf("message %q should tell the user to write CUSTOM@host", err.Error())
	}
	if strings.Contains(err.Error(), secretValue) {
		t.Fatalf("message %q leaks the secret value", err.Error())
	}
}

func TestResolveMissingFromStore(t *testing.T) {
	store, path := loadStore(t)

	_, err := store.Resolve([]Request{{Name: "OPENAI_API_KEY"}})
	if err == nil {
		t.Fatal("Resolve() succeeded for a name not in the store")
	}
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("code = %q, want %q", ez.ErrorCode(err), ez.ENOTFOUND)
	}
	if !strings.Contains(err.Error(), "OPENAI_API_KEY") || !strings.Contains(err.Error(), path) {
		t.Fatalf("message %q should name the secret and the file", err.Error())
	}
}

func TestResolveCombinesProblems(t *testing.T) {
	store, path := loadStore(t)

	_, err := store.Resolve([]Request{
		{Name: "GITHUB_TOKEN"},
		{Name: "CUSTOM"},
		{Name: "OPENAI_API_KEY"},
		{Name: "OTHER"},
	})
	if err == nil {
		t.Fatal("Resolve() succeeded with problems")
	}
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("code = %q, want first problem's code %q", ez.ErrorCode(err), ez.EINVALID)
	}
	msg := err.Error()
	for _, part := range []string{"CUSTOM@host", "OPENAI_API_KEY is not set in " + path, "OTHER is not set", "OTHER@host"} {
		if !strings.Contains(msg, part) {
			t.Fatalf("message %q should contain %q", msg, part)
		}
	}
	if strings.Contains(msg, secretValue) {
		t.Fatalf("message %q leaks the secret value", msg)
	}
}

func TestResolveAccount(t *testing.T) {
	path := writeSecrets(t, "ANTHROPIC_WORK="+secretValue+"\nGITHUB_BOT="+secretValue+"\nCUSTOM="+secretValue+"\n", 0o600)
	store, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	requests := []Request{}
	for _, entry := range []string{
		"ANTHROPIC_WORK as CLAUDE_CODE_OAUTH_TOKEN",
		"GITHUB_BOT as GITHUB_TOKEN@ghe.example.com",
		"CUSTOM as GITHUB_TOKEN",
	} {
		request, err := ParseRequest(entry)
		if err != nil {
			t.Fatal(err)
		}
		requests = append(requests, request)
	}

	got, err := store.Resolve(requests)
	if err != nil {
		t.Fatal(err)
	}
	want := []Binding{
		{Name: "ANTHROPIC_WORK", Env: "CLAUDE_CODE_OAUTH_TOKEN", Value: secretValue, Hosts: []string{"api.anthropic.com"}},
		{Name: "GITHUB_BOT", Env: "GITHUB_TOKEN", Value: secretValue, Hosts: []string{"ghe.example.com"}},
		{Name: "CUSTOM", Env: "GITHUB_TOKEN", Value: secretValue, Hosts: []string{"api.github.com", "github.com"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Resolve() = %+v, want %+v", got, want)
	}
}

func TestResolveAccountHostsFallBackToName(t *testing.T) {
	path := writeSecrets(t, "GITHUB_TOKEN="+secretValue+"\n", 0o600)
	store, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.Resolve([]Request{{Name: "GITHUB_TOKEN", Env: "GH_TOKEN"}})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Env != "GH_TOKEN" || !reflect.DeepEqual(got[0].Hosts, []string{"api.github.com", "github.com"}) {
		t.Fatalf("Resolve() = %+v, want GH_TOKEN bound to the GITHUB_TOKEN hosts", got[0])
	}
}

func TestResolveAccountWithoutHosts(t *testing.T) {
	store, _ := loadStore(t)

	_, err := store.Resolve([]Request{{Name: "CUSTOM", Env: "MY_VAR"}})
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("code = %q, want %q", ez.ErrorCode(err), ez.EINVALID)
	}
	if !strings.Contains(err.Error(), "CUSTOM as MY_VAR@host") {
		t.Fatalf("message %q should tell the user to write CUSTOM as MY_VAR@host", err.Error())
	}
	if strings.Contains(err.Error(), secretValue) {
		t.Fatalf("message %q leaks the secret value", err.Error())
	}
}
