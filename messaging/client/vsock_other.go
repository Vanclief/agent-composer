//go:build !linux

package client

import (
	"context"
	"net"
	"net/http"

	"github.com/vanclief/ez"
)

// DialVsock returns a client that always fails: vsock exists only
// inside a Linux Task VM.
func DialVsock(port uint32) *http.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return nil, ez.New(ez.EUNAVAILABLE, "vsock is only available inside a Linux Task VM", nil)
		},
	}

	return &http.Client{Transport: transport}
}
