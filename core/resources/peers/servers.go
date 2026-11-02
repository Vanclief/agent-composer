package peers

import (
	"context"
	"sync"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
)

// hostTimeout bounds asking one peer for its host report.
const hostTimeout = 3 * time.Second

// ListServersRequest lists the servers sandboxes can run on (GET
// /api/servers).
type ListServersRequest struct{}

func (r ListServersRequest) Validate() error {
	return nil
}

// Capacity is cpus and memory.
type Capacity struct {
	CPUs      int    `json:"cpus"`
	MemoryMiB uint64 `json:"memory_mib"`
}

// ServerView is one server as the panel's pool view needs it.
type ServerView struct {
	// Name is local for this server, else the name in servers:.
	Name string `json:"name"`
	// URL is the peer's root, empty for local.
	URL string `json:"url"`
	// Reachable is whether the server answered its host report now; local
	// always does.
	Reachable bool `json:"reachable"`
	// Error says why a peer is not reachable.
	Error string `json:"error,omitempty"`
	// Sandboxes counts its running sandboxes.
	Sandboxes int `json:"sandboxes"`
	// InUse sums the specs of its running sandboxes, from this server's
	// rows.
	InUse Capacity `json:"in_use"`
	// Free is the host's cpus less those in use (never below zero) and the
	// host's own free memory report; zero when unreachable.
	Free Capacity `json:"free"`
	// Host is the server's own report of its host, null when unreachable.
	Host *HostReport `json:"host"`
}

type ListServersResponse struct {
	Servers []ServerView `json:"servers"`
}

// ListServers lists local, then the peers by name, each with what its
// running sandboxes use and what its host has free. Peers are asked at
// once, each for at most hostTimeout.
func (api *API) ListServers(ctx context.Context, requester interface{}, request *ListServersRequest) (*ListServersResponse, error) {
	usage := map[string]sandboxes.Usage{}
	if api.sandboxes != nil {
		var err error

		usage, err = api.sandboxes.Usage(ctx)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	local := readHost(ctx)
	views := []ServerView{serverView(controller.LocalServer, "", &local, nil, usage)}

	remotes := append([]*Remote(nil), api.remotes...)
	sortRemotes(remotes)

	peerViews := make([]ServerView, len(remotes))

	var wg sync.WaitGroup
	for i, remote := range remotes {
		wg.Add(1)
		go func() {
			defer wg.Done()

			hostCtx, cancel := context.WithTimeout(ctx, hostTimeout)
			defer cancel()

			report, err := remote.Host(hostCtx)
			peerViews[i] = serverView(remote.Name, remote.URL, report, err, usage)
		}()
	}
	wg.Wait()

	return &ListServersResponse{Servers: append(views, peerViews...)}, nil
}

func serverView(name string, baseURL string, report *HostReport, err error, usage map[string]sandboxes.Usage) ServerView {
	used := usage[name]

	view := ServerView{
		Name:      name,
		URL:       baseURL,
		Reachable: err == nil,
		Sandboxes: used.Sandboxes,
		InUse:     Capacity{CPUs: used.CPUs, MemoryMiB: used.MemoryMiB},
	}

	if err != nil {
		view.Error = ez.ErrorMessage(err)
		return view
	}

	view.Host = report

	freeCPUs := report.CPUs - used.CPUs
	if freeCPUs < 0 {
		freeCPUs = 0
	}
	view.Free = Capacity{CPUs: freeCPUs, MemoryMiB: report.FreeMemoryMiB}

	return view
}
