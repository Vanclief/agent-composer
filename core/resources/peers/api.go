// Package peers lets one agc server allocate sandboxes on others
// (docs/decisions/0018-peer-servers.md). A primary holds tasks, messaging,
// knowledge and the panel; each peer is another agc rest with its own msb
// and secrets file. On the primary, every peer is a Remote: a
// sandboxes.Runtime over the peer's /api/peer routes. On a peer, the API
// serves those routes over its local msb runtime and forwards each VM's
// socket to the primary. GET /api/servers lists local and the peers.
package peers

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/executor/sandbox"
)

// API is both roles of a server: the Remotes of its peers, and the peer
// routes it serves when it is a peer itself. One server may be both.
type API struct {
	// sandboxes is this server's sandboxes API, for the in-use sums.
	sandboxes *sandboxes.API
	// local is this server's msb runtime, which the peer routes drive.
	local sandboxes.Runtime
	// list finds the VMs a primary made here after a restart
	// (sandbox.List); tests replace it.
	list func(ctx context.Context, labelKey string) ([]sandbox.Info, error)

	secretsPath string
	// socketDir holds one socket per VM run for a primary, <name>.sock,
	// apart from this server's own sandboxes' sockets.
	socketDir string
	// primary is the primary's root URL from primary: { url }; empty
	// when this server is no peer.
	primary string

	remotes []*Remote
	// proxy carries forwarded socket requests to the primary; no
	// timeout, since GET /wait blocks.
	proxy *http.Transport

	mu       sync.Mutex
	forwards map[string]*forward
}

// NewAPI builds the peers API from the config's servers: and primary:.
func NewAPI(ctrl *controller.Controller, sbx *sandboxes.API, local sandboxes.Runtime, secretsPath string, socketDir string) (*API, error) {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	return New(ctrl.Config.Servers, ctrl.Config.Primary.URL, sbx, local, secretsPath, socketDir)
}

// New is NewAPI over explicit settings, for tests. primaryURL is empty
// on a server that is no peer.
func New(servers []controller.ServerSettings, primaryURL string, sbx *sandboxes.API, local sandboxes.Runtime, secretsPath string, socketDir string) (*API, error) {
	peers, err := controller.ValidatePeers(servers)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	primary := ""
	if primaryURL != "" {
		primary, err = controller.ValidateServerURL(primaryURL)
		if err != nil {
			return nil, ez.New(ez.EINVALID, "primary: "+ez.ErrorMessage(err), nil)
		}
	}

	api := &API{
		sandboxes:   sbx,
		local:       local,
		list:        sandbox.List,
		secretsPath: secretsPath,
		socketDir:   socketDir,
		primary:     primary,
		proxy:       &http.Transport{ResponseHeaderTimeout: 0, IdleConnTimeout: 90 * time.Second},
		forwards:    map[string]*forward{},
	}

	for _, peer := range peers {
		api.remotes = append(api.remotes, NewRemote(peer.Name, peer.URL, secretsPath))
	}

	return api, nil
}

// Runtimes are the peers' runtimes by name, for sandboxes.API.SetServers.
func (api *API) Runtimes() map[string]sandboxes.Runtime {
	runtimes := map[string]sandboxes.Runtime{}
	for _, remote := range api.remotes {
		runtimes[remote.Name] = remote
	}

	return runtimes
}

// SetList replaces how the API finds the VMs it runs for a primary.
func (api *API) SetList(list func(ctx context.Context, labelKey string) ([]sandbox.Info, error)) {
	api.list = list
}

// SecretsPath is the secrets file holding AGC_PEER_TOKEN.
func (api *API) SecretsPath() string {
	return api.secretsPath
}

func sortRemotes(remotes []*Remote) {
	sort.Slice(remotes, func(i, j int) bool {
		return remotes[i].Name < remotes[j].Name
	})
}
