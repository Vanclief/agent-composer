package controller

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/vanclief/ez"
)

// LocalServer is the name of the agc server a process runs on. It needs no
// entry in servers: and a sandbox row stores it as the empty string
// (ADR 0018).
const LocalServer = "local"

var serverNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ServerSettings is one peer in the servers: block of the config file: an
// agc rest on another machine whose msb this server allocates sandboxes on
// (ADR 0018).
type ServerSettings struct {
	Name string `mapstructure:"name" json:"name"`
	URL  string `mapstructure:"url" json:"url"`
}

// PrimarySettings is the primary: block of a peer's config file: the agc
// rest that holds tasks and messaging, which the peer forwards its
// sandboxes' sockets to (ADR 0018).
type PrimarySettings struct {
	URL string `mapstructure:"url" json:"url"`
}

// Peers returns the configured peers, checked: each has a name of
// lowercase letters, digits and dashes other than local, unique, and an
// http or https URL. The URLs lose a trailing slash.
func (controller *Controller) Peers() ([]ServerSettings, error) {
	return ValidatePeers(controller.Config.Servers)
}

// ValidatePeers checks configured as Peers does.
func ValidatePeers(configured []ServerSettings) ([]ServerSettings, error) {
	peers := []ServerSettings{}
	seen := map[string]bool{}

	for i, server := range configured {
		name := strings.TrimSpace(server.Name)
		if !serverNameRe.MatchString(name) {
			return nil, ez.New(ez.EINVALID, fmt.Sprintf("servers[%d]: name %q must be lowercase letters, digits and dashes", i, server.Name), nil)
		}

		if name == LocalServer {
			return nil, ez.New(ez.EINVALID, fmt.Sprintf("servers[%d]: %s is this server and needs no entry", i, LocalServer), nil)
		}

		if seen[name] {
			return nil, ez.New(ez.EINVALID, fmt.Sprintf("servers[%d]: %s is listed twice", i, name), nil)
		}
		seen[name] = true

		base, err := ValidateServerURL(server.URL)
		if err != nil {
			return nil, ez.New(ez.EINVALID, fmt.Sprintf("servers[%d] (%s): %s", i, name, ez.ErrorMessage(err)), nil)
		}

		peers = append(peers, ServerSettings{Name: name, URL: base})
	}

	sort.Slice(peers, func(i, j int) bool {
		return peers[i].Name < peers[j].Name
	})

	return peers, nil
}

// ValidateServerURL checks raw is an http or https URL with a host and no
// query, and returns it without a trailing slash.
func ValidateServerURL(raw string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")

	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.RawQuery != "" {
		return "", ez.New(ez.EINVALID, fmt.Sprintf("url %q must be http:// or https:// with a host, such as http://192.168.1.20:1202", raw), nil)
	}

	return trimmed, nil
}

// ListenAddress is where agc rest binds: listen: from the config file, or
// all interfaces on the app port, as before listen: existed.
func (controller *Controller) ListenAddress() string {
	listen := strings.TrimSpace(controller.Config.Listen)
	if listen != "" {
		return listen
	}

	return ":" + controller.Config.App.Port
}
