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
		"GITHUB_TOKEN":                   {Name: "GITHUB_TOKEN"},
		" A1_B ":                         {Name: "A1_B"},
		"STRIPE_KEY@api.stripe.com":      {Name: "STRIPE_KEY", Hosts: []string{"api.stripe.com"}},
		"X@a.example.com, *.example.org": {Name: "X", Hosts: []string{"a.example.com", "*.example.org"}},
		"LOCAL@localhost":                {Name: "LOCAL", Hosts: []string{"localhost"}},
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
		{Name: "GITHUB_TOKEN", Value: secretValue, Hosts: []string{"api.github.com", "github.com"}},
		{Name: "CUSTOM", Value: secretValue, Hosts: []string{"api.custom.dev"}},
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

func TestPeerTokenIsBoundToNoHost(t *testing.T) {
	store, _ := loadStore(t)

	// Neither by name nor with explicit hosts does a spec get it.
	for _, req := range []Request{{Name: PeerToken}, {Name: PeerToken, Hosts: []string{"example.com"}}} {
		_, err := store.Resolve([]Request{req})
		if err == nil || !strings.Contains(err.Error(), "agc's own") {
			t.Fatalf("Resolve(%+v) = %v, want refused as agc's own", req, err)
		}

		_, err = Bind([]Request{req})
		if err == nil || !strings.Contains(err.Error(), "agc's own") {
			t.Fatalf("Bind(%+v) = %v, want refused as agc's own", req, err)
		}
	}

	_, err := store.Fill([]Binding{{Name: PeerToken, Hosts: []string{"example.com"}}}, "mini")
	if err == nil || !strings.Contains(err.Error(), "never given to a sandbox") {
		t.Fatalf("Fill() = %v, want refused", err)
	}
}

func TestBindAndFill(t *testing.T) {
	store, path := loadStore(t)

	bound, err := Bind([]Request{{Name: "GITHUB_TOKEN"}, {Name: "MISSING", Hosts: []string{"api.missing.dev"}}})
	if err != nil {
		t.Fatal(err)
	}
	if bound[0].Value != "" || !reflect.DeepEqual(bound[0].Hosts, []string{"api.github.com", "github.com"}) {
		t.Fatalf("Bind() = %+v, want hosts and no value", bound)
	}

	_, err = store.Fill(bound, "mini")
	if err == nil || ez.ErrorCode(err) != ez.ENOTFOUND || !strings.Contains(err.Error(), "secret MISSING is not set on server mini: add it to "+path) {
		t.Fatalf("Fill() = %v, want MISSING named with the server", err)
	}
	if strings.Contains(err.Error(), secretValue) {
		t.Fatal("Fill() error carries a value")
	}

	filled, err := store.Fill(bound[:1], "mini")
	if err != nil {
		t.Fatal(err)
	}
	if filled[0].Value != secretValue {
		t.Fatal("Fill() did not set the value")
	}
}
