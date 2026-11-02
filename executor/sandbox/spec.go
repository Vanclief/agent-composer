// Package sandbox runs programs inside a microsandbox VM, one per Task
// (docs/decisions/0003-tasks-run-in-microsandbox-vms.md).
//
// The SDK needs cgo. Builds without cgo get the same API, and every call
// returns an ez.EUNAVAILABLE error.
package sandbox

import (
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"

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

	// DefaultDiskMiB and MinDiskMiB bound the private disk (Spec.DiskMiB).
	DefaultDiskMiB = 20 * 1024
	MinDiskMiB     = 2 * 1024
	maxSizeMiB     = 1<<32 - 1
)

var sizeRe = regexp.MustCompile(`^([0-9]+)([MGT])$`)

// ParseSize parses a size such as 512M, 10G or 1T (binary units, the
// suffix msb uses) into MiB.
func ParseSize(size string) (uint32, error) {
	match := sizeRe.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(size)))
	if match == nil {
		return 0, ez.New(ez.EINVALID, fmt.Sprintf("size %q must be a whole number followed by M, G or T, such as 10G", size), nil)
	}

	n, err := strconv.ParseUint(match[1], 10, 64)
	if err != nil {
		return 0, ez.New(ez.EINVALID, fmt.Sprintf("size %q is too large", size), nil)
	}

	mib := n
	switch match[2] {
	case "G":
		mib = n * 1024
	case "T":
		mib = n * 1024 * 1024
	}

	if n > maxSizeMiB || mib > maxSizeMiB {
		return 0, ez.New(ez.EINVALID, fmt.Sprintf("size %q is too large", size), nil)
	}

	return uint32(mib), nil
}

// FormatSize writes MiB as msb reads it: whole gibibytes as G, else M.
func FormatSize(mib uint32) string {
	if mib%1024 == 0 {
		return fmt.Sprintf("%dG", mib/1024)
	}

	return fmt.Sprintf("%dM", mib)
}

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
// Named volumes are created on first use and kept when the sandbox is
// removed. QuotaMiB, when set, caps what the guest may add to the volume
// in one boot (msb counts from the size at the boot's first write, so it
// does not bound the volume's total size across boots).
type Volume struct {
	Source    string
	Dest      string
	ReadWrite bool
	HostPath  bool
	QuotaMiB  uint32
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
	// DiskDest, when set, gets a private ext4 disk of DiskMiB, owned by
	// the sandbox: it survives stop and start and is removed with the
	// sandbox. The disk's root directory belongs to root.
	DiskDest string
	DiskMiB  uint32
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
	if s.DiskDest != "" && s.DiskMiB == 0 {
		s.DiskMiB = DefaultDiskMiB
	}
	if s.DiskDest != "" && s.DiskMiB < MinDiskMiB {
		return ez.New(ez.EINVALID, fmt.Sprintf("disk is %s, the minimum is %s", FormatSize(s.DiskMiB), FormatSize(MinDiskMiB)), nil)
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
