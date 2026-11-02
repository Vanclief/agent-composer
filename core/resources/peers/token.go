package peers

import (
	"crypto/subtle"
	"strings"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/secrets"
)

// TokenScheme prefixes the peer token in the Authorization header.
const TokenScheme = "Bearer "

// loadToken reads AGC_PEER_TOKEN from the secrets file at path, read on
// every call so a change needs no restart. Empty when unset.
func loadToken(path string) (string, error) {
	store, err := secrets.Load(path)
	if err != nil {
		return "", ez.Wrap(err)
	}

	token, _ := store.Lookup(secrets.PeerToken)

	return strings.TrimSpace(token), nil
}

// CheckToken checks header, an Authorization header, carries this
// server's AGC_PEER_TOKEN. With no token set the peer routes are off.
// Errors never echo the token.
func CheckToken(secretsPath string, header string) error {
	token, err := loadToken(secretsPath)
	if err != nil {
		return ez.Wrap(err)
	}

	if token == "" {
		return ez.New(ez.EUNAVAILABLE, "peer routes are off here: "+secrets.PeerToken+" is not set in "+secretsPath, nil)
	}

	given, ok := strings.CutPrefix(header, TokenScheme)
	if !ok || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(given)), []byte(token)) != 1 {
		return ez.New(ez.ENOTAUTHENTICATED, "the peer token is missing or wrong", nil)
	}

	return nil
}
