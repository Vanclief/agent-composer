package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/vanclief/ez"
	"golang.org/x/sys/unix"
)

// DialVsock returns an http.Client whose every connection goes to the
// host (CID 2) on the given vsock port. The URL host is ignored.
func DialVsock(port uint32) *http.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialVsock(port)
		},
	}

	return &http.Client{Transport: transport}
}

// dialVsock connects an AF_VSOCK socket to the host. net.FileConn
// rejects vsock sockets (it only knows inet and unix families), so the
// socket is wrapped in a pollable *os.File and given the two address
// methods net.Conn needs.
func dialVsock(port uint32) (net.Conn, error) {
	fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, ez.New(ez.EUNAVAILABLE, "cannot open a vsock socket", err)
	}

	err = unix.Connect(fd, &unix.SockaddrVM{CID: unix.VMADDR_CID_HOST, Port: port})
	if err != nil {
		_ = unix.Close(fd)
		return nil, ez.New(ez.EUNAVAILABLE, fmt.Sprintf("cannot connect to the host on vsock port %d", port), err)
	}

	// Non-blocking lets os.NewFile register the fd with the runtime
	// poller, which gives Read, Write and deadlines without blocking
	// an OS thread.
	err = unix.SetNonblock(fd, true)
	if err != nil {
		_ = unix.Close(fd)
		return nil, ez.Wrap(err)
	}

	file := os.NewFile(uintptr(fd), fmt.Sprintf("vsock:%d", port))

	return &vsockConn{File: file, port: port}, nil
}

type vsockConn struct {
	*os.File
	port uint32
}

func (c *vsockConn) LocalAddr() net.Addr {
	return vsockAddr{cid: unix.VMADDR_CID_ANY}
}

func (c *vsockConn) RemoteAddr() net.Addr {
	return vsockAddr{cid: unix.VMADDR_CID_HOST, port: c.port}
}

type vsockAddr struct {
	cid  uint32
	port uint32
}

func (a vsockAddr) Network() string {
	return "vsock"
}

func (a vsockAddr) String() string {
	return fmt.Sprintf("%d:%d", a.cid, a.port)
}
