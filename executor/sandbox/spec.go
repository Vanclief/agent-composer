// Package sandbox runs programs inside a microsandbox VM, one per Task
// (docs/decisions/0003-tasks-run-in-microsandbox-vms.md).
//
// The SDK needs cgo. Builds without cgo get the same API, and every call
// returns an ez.EUNAVAILABLE error.
package sandbox

import (
	"fmt"
	"runtime"

	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/ez"
)

var _ executor.TerminalExecutor = (*Executor)(nil)

const (
	defaultImage     = "agc-agent"
	defaultCPUs      = 2
	defaultMemoryMiB = 4096
	defaultUser      = "agent"
	minMemoryMiB     = 256
	maxNameBytes     = 128
)

// Secret is a placeholder secret (docs/decisions/0004-secrets-are-placeholders-bound-to-hosts.md).
// The guest sees a placeholder in the environment variable Name, and the runtime
// swaps in Value only in request headers sent to Hosts. Value is passed to
// msb through its environment and never stored by the runtime.
type Secret struct {
	Name  string
	Value string
	Hosts []string
}

// Volume mounts a microsandbox named volume, or a host directory when
// HostPath is set, at Dest. Mounts are read-only unless ReadWrite is set.
// Named volumes are created on first use.
type Volume struct {
	Source    string
	Dest      string
	ReadWrite bool
	HostPath  bool
}

// Spec describes a sandbox to create. Validate fills in the defaults.
type Spec struct {
	Name      string
	Image     string
	CPUs      uint8
	MemoryMiB uint32
	User      string
	Env       map[string]string
	Secrets   []Secret
	Volumes   []Volume
	// VsockPath is a host unix socket exposed to the guest on VsockPort.
	// Empty means no vsock route.
	VsockPath string
	VsockPort uint32
	Labels    map[string]string
}

// Info is one sandbox as the runtime reports it. Status is the runtime's own
// word, passed through verbatim.
type Info struct {
	Name   string
	Image  string
	Status string
}

// Validate applies defaults and rejects a spec the runtime cannot honor.
func (s *Spec) Validate() error {
	if s.Name == "" {
		return ez.New(ez.EINVALID, "name is required", nil)
	}
	if len(s.Name) > maxNameBytes {
		return ez.New(ez.EINVALID, fmt.Sprintf("name is longer than %d bytes", maxNameBytes), nil)
	}
	if s.Image == "" {
		s.Image = defaultImage
	}
	if s.CPUs == 0 {
		s.CPUs = defaultCPUs
	}
	if int(s.CPUs) > runtime.NumCPU() {
		return ez.New(ez.EINVALID, fmt.Sprintf("cpus is %d but this host has %d", s.CPUs, runtime.NumCPU()), nil)
	}
	if s.MemoryMiB == 0 {
		s.MemoryMiB = defaultMemoryMiB
	}
	if s.MemoryMiB < minMemoryMiB {
		return ez.New(ez.EINVALID, fmt.Sprintf("memory is %d MiB, the minimum is %d MiB", s.MemoryMiB, minMemoryMiB), nil)
	}
	if s.User == "" {
		s.User = defaultUser
	}
	for i, v := range s.Volumes {
		if v.Source == "" {
			return ez.New(ez.EINVALID, fmt.Sprintf("volumes[%d]: source is required", i), nil)
		}
		if v.Dest == "" {
			return ez.New(ez.EINVALID, fmt.Sprintf("volumes[%d] (%s): dest is required", i, v.Source), nil)
		}
	}
	for i, secret := range s.Secrets {
		if secret.Name == "" {
			return ez.New(ez.EINVALID, fmt.Sprintf("secrets[%d]: name is required", i), nil)
		}
		if len(secret.Hosts) == 0 {
			return ez.New(ez.EINVALID, fmt.Sprintf("secrets[%d] (%s): hosts are required", i, secret.Name), nil)
		}
	}
	if s.VsockPath != "" && s.VsockPort == 0 {
		return ez.New(ez.EINVALID, "vsock port is required when vsock path is set", nil)
	}
	return nil
}
