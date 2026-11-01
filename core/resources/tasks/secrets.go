package tasks

import (
	"context"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/secrets"
)

// Secrets are listed by name only. No request returns a value
// (docs/decisions/0004-secrets-are-placeholders-bound-to-hosts.md).

type ListSecretsRequest struct{}

func (r ListSecretsRequest) Validate() error {
	return nil
}

// SecretView is one configured secret: its name and the hosts it is bound
// to by default, empty when the name is not in secrets.KnownHosts.
type SecretView struct {
	Name  string   `json:"name"`
	Hosts []string `json:"hosts"`
}

type ListSecretsResponse struct {
	Secrets []SecretView `json:"secrets"`
}

// ListSecrets returns the names in the secrets file, sorted, with their
// known hosts.
func (api *API) ListSecrets(ctx context.Context, requester interface{}, request *ListSecretsRequest) (*ListSecretsResponse, error) {
	store, err := secrets.Load(api.secretsPath)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	names := store.Names()
	views := make([]SecretView, 0, len(names))
	for _, name := range names {
		hosts := append([]string{}, secrets.KnownHosts[name]...)
		views = append(views, SecretView{Name: name, Hosts: hosts})
	}

	return &ListSecretsResponse{Secrets: views}, nil
}
