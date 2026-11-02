package peers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vanclief/ez"
)

func TestCheckToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.env")

	// No file: the peer routes are off.
	err := CheckToken(path, "Bearer x")
	if ez.ErrorCode(err) != ez.EUNAVAILABLE {
		t.Fatalf("no token: %v, want unavailable", err)
	}

	err = os.WriteFile(path, []byte("AGC_PEER_TOKEN=fixture-token\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"":                    ez.ENOTAUTHENTICATED,
		"fixture-token":       ez.ENOTAUTHENTICATED,
		"Bearer other":        ez.ENOTAUTHENTICATED,
		"Bearer fixture-toke": ez.ENOTAUTHENTICATED,
	}
	for header, code := range cases {
		err = CheckToken(path, header)
		if ez.ErrorCode(err) != code {
			t.Fatalf("%q: %v, want %s", header, err, code)
		}
	}

	err = CheckToken(path, "Bearer fixture-token")
	if err != nil {
		t.Fatal(err)
	}
}
