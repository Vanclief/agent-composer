package peers

import (
	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/executor/sandbox"
	"github.com/vanclief/agent-composer/secrets"
)

// The peer routes carry sandbox.Spec, executor.Command and their results
// as JSON. These types are the wire shapes; a secret on the wire has a name
// and hosts, never a value (ADR 0018).

// Secret is a secret the VM gets as a placeholder: the peer reads its
// value from its own secrets file.
type Secret struct {
	Name  string   `json:"name"`
	Hosts []string `json:"hosts"`
}

// Volume is sandbox.Volume. A host path is a path on the peer.
type Volume struct {
	Source    string `json:"source"`
	Dest      string `json:"dest"`
	ReadWrite bool   `json:"read_write"`
	HostPath  bool   `json:"host_path"`
	QuotaMiB  uint32 `json:"quota_mib"`
}

// VMSpec is sandbox.Spec without secret values and without the vsock
// path, which is the peer's own.
type VMSpec struct {
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	CPUs      uint8             `json:"cpus"`
	MemoryMiB uint32            `json:"memory_mib"`
	User      string            `json:"user"`
	Env       map[string]string `json:"env"`
	Secrets   []Secret          `json:"secrets"`
	Volumes   []Volume          `json:"volumes"`
	DiskDest  string            `json:"disk_dest"`
	DiskMiB   uint32            `json:"disk_mib"`
	VsockPort uint32            `json:"vsock_port"`
	Labels    map[string]string `json:"labels"`
}

func toVMSpec(spec sandbox.Spec) VMSpec {
	volumes := []Volume{}
	for _, v := range spec.Volumes {
		volumes = append(volumes, Volume(v))
	}

	return VMSpec{
		Name:      spec.Name,
		Image:     spec.Image,
		CPUs:      spec.CPUs,
		MemoryMiB: spec.MemoryMiB,
		User:      spec.User,
		Env:       spec.Env,
		Secrets:   toSecrets(spec.Secrets),
		Volumes:   volumes,
		DiskDest:  spec.DiskDest,
		DiskMiB:   spec.DiskMiB,
		VsockPort: spec.VsockPort,
		Labels:    spec.Labels,
	}
}

// sandboxSpec is the spec the peer boots, its vsock at vsockPath and its
// secrets filled from the peer's store.
func (s VMSpec) sandboxSpec(vsockPath string, filled []secrets.Binding) sandbox.Spec {
	volumes := []sandbox.Volume{}
	for _, v := range s.Volumes {
		volumes = append(volumes, sandbox.Volume(v))
	}

	vmSecrets := []sandbox.Secret{}
	for _, binding := range filled {
		vmSecrets = append(vmSecrets, sandbox.Secret{Name: binding.Name, Value: binding.Value, Hosts: binding.Hosts})
	}

	if s.VsockPort == 0 {
		vsockPath = ""
	}

	return sandbox.Spec{
		Name:      s.Name,
		Image:     s.Image,
		CPUs:      s.CPUs,
		MemoryMiB: s.MemoryMiB,
		User:      s.User,
		Env:       s.Env,
		Secrets:   vmSecrets,
		Volumes:   volumes,
		DiskDest:  s.DiskDest,
		DiskMiB:   s.DiskMiB,
		VsockPath: vsockPath,
		VsockPort: s.VsockPort,
		Labels:    s.Labels,
	}
}

// toSecrets drops the values.
func toSecrets(vmSecrets []sandbox.Secret) []Secret {
	out := []Secret{}
	for _, secret := range vmSecrets {
		out = append(out, Secret{Name: secret.Name, Hosts: secret.Hosts})
	}

	return out
}

func bindingsOf(wire []Secret) []secrets.Binding {
	bindings := []secrets.Binding{}
	for _, secret := range wire {
		bindings = append(bindings, secrets.Binding{Name: secret.Name, Hosts: secret.Hosts})
	}

	return bindings
}

// Command is executor.Command.
type Command struct {
	Name  string            `json:"name"`
	Args  []string          `json:"args"`
	Dir   string            `json:"dir"`
	Env   map[string]string `json:"env"`
	Stdin []byte            `json:"stdin"`
	User  string            `json:"user"`
}

func toCommand(cmd executor.Command) *Command {
	return &Command{Name: cmd.Name, Args: cmd.Args, Dir: cmd.Dir, Env: cmd.Env, Stdin: cmd.Stdin, User: cmd.User}
}

func (c Command) executor() executor.Command {
	return executor.Command{Name: c.Name, Args: c.Args, Dir: c.Dir, Env: c.Env, Stdin: c.Stdin, User: c.User}
}

// Result is executor.Result.
type Result struct {
	Stdout   []byte `json:"stdout"`
	Stderr   []byte `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// Terminal is executor.Terminal.
type Terminal struct {
	ID      string   `json:"id"`
	Command []string `json:"command"`
	Running bool     `json:"running"`
}

func toTerminal(t executor.Terminal) Terminal {
	return Terminal{ID: t.ID, Command: t.Command, Running: t.Running}
}

func (t Terminal) executor() executor.Terminal {
	return executor.Terminal{ID: t.ID, Command: t.Command, Running: t.Running}
}
