package peers

import (
	"context"
	"strings"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/secrets"
	"github.com/vanclief/agent-composer/task"
)

// The peer routes, /api/peer/*, served by a peer to its primary under
// AGC_PEER_TOKEN (ADR 0018). They drive this server's msb directly: the
// peer keeps no sandbox rows, the primary does. Each method is the
// sandboxes.Runtime or executor call of the same name.

// Exec operations, one per executor.TerminalExecutor method.
const (
	OpRun           = "run"
	OpLookPath      = "look_path"
	OpWriteFile     = "write_file"
	OpReadFile      = "read_file"
	OpRemoveFile    = "remove_file"
	OpStartTerminal = "start_terminal"
	OpListTerminals = "list_terminals"
)

// ResizeMessage is the text frame that resizes a terminal.
type ResizeMessage struct {
	Type string `json:"type"`
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

func validateServer(server string) error {
	if strings.TrimSpace(server) == "" {
		return ez.New(ez.EINVALID, "server is required: the name the primary gives this peer", nil)
	}

	return nil
}

func validateVMName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ez.New(ez.EINVALID, "name is required", nil)
	}

	if strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return ez.New(ez.EINVALID, "name "+name+" is not a sandbox name", nil)
	}

	return nil
}

// HostRequest reads this server's host report (GET /api/peer/host).
type HostRequest struct{}

func (r HostRequest) Validate() error {
	return nil
}

// Host reports this host, for the primary's GET /api/servers.
func (api *API) Host(ctx context.Context, requester interface{}, request *HostRequest) (*HostReport, error) {
	report := readHost(ctx)

	return &report, nil
}

// CheckSecretsRequest asks whether every secret is set here (POST
// /api/peer/secrets/check), before the primary inserts a sandbox.
type CheckSecretsRequest struct {
	Server  string   `json:"server"`
	Secrets []Secret `json:"secrets"`
}

func (r CheckSecretsRequest) Validate() error {
	return validateServer(r.Server)
}

type CheckSecretsResponse struct {
	OK bool `json:"ok"`
}

// CheckSecrets fails naming each secret this server's secrets file lacks.
func (api *API) CheckSecrets(ctx context.Context, requester interface{}, request *CheckSecretsRequest) (*CheckSecretsResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	_, err = api.fill(request.Server, request.Secrets)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &CheckSecretsResponse{OK: true}, nil
}

// fill reads the secrets' values from this server's secrets file.
func (api *API) fill(server string, wire []Secret) ([]secrets.Binding, error) {
	if len(wire) == 0 {
		return nil, nil
	}

	store, err := secrets.Load(api.secretsPath)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	filled, err := store.Fill(bindingsOf(wire), server)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return filled, nil
}

// VMResponse is a VM's name and msb's status word.
type VMResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// CreateVMRequest creates and boots a VM here for the primary (POST
// /api/peer/vms).
type CreateVMRequest struct {
	Server string `json:"server"`
	Spec   VMSpec `json:"spec"`
}

func (r CreateVMRequest) Validate() error {
	err := validateServer(r.Server)
	if err != nil {
		return ez.Wrap(err)
	}

	return validateVMName(r.Spec.Name)
}

// CreateVM fills the secrets from this server's file, starts the VM's
// socket forwarder, then creates the VM with its vsock on that socket.
func (api *API) CreateVM(ctx context.Context, requester interface{}, request *CreateVMRequest) (*VMResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.requirePrimary()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	filled, err := api.fill(request.Server, request.Spec.Secrets)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	name := request.Spec.Name

	labels := map[string]string{}
	for k, v := range request.Spec.Labels {
		labels[k] = v
	}
	labels[task.LabelServer] = request.Server
	request.Spec.Labels = labels

	spec := request.Spec.sandboxSpec(api.socketPath(name), filled)

	err = api.startForward(name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	sb, err := api.local.Create(ctx, spec)
	if err != nil {
		api.stopForward(name)
		return nil, ez.Wrap(err)
	}

	_ = sb.Detach(context.WithoutCancel(ctx))

	return &VMResponse{Name: name, Status: sandboxes.StatusRunning}, nil
}

// VMStatusRequest reads a VM's status (GET /api/peer/vms/{name}).
type VMStatusRequest struct {
	Name string `json:"-"`
}

func (r VMStatusRequest) Validate() error {
	return validateVMName(r.Name)
}

func (api *API) VMStatus(ctx context.Context, requester interface{}, request *VMStatusRequest) (*VMResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	status, err := api.local.Status(ctx, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &VMResponse{Name: request.Name, Status: status}, nil
}

// StartVMRequest boots a stopped VM (POST /api/peer/vms/{name}/start).
type StartVMRequest struct {
	Name    string   `json:"-"`
	Server  string   `json:"server"`
	Secrets []Secret `json:"secrets"`
}

func (r StartVMRequest) Validate() error {
	err := validateServer(r.Server)
	if err != nil {
		return ez.Wrap(err)
	}

	return validateVMName(r.Name)
}

// StartVM reads the secrets again, since their values may have changed,
// and brings the forwarder back before the VM.
func (api *API) StartVM(ctx context.Context, requester interface{}, request *StartVMRequest) (*VMResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.requirePrimary()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	filled, err := api.fill(request.Server, request.Secrets)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.startForward(request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.local.Start(ctx, request.Name, task.SandboxSecrets(filled))
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.VMStatus(ctx, requester, &VMStatusRequest{Name: request.Name})
}

// StopVMRequest shuts a VM down and keeps its disk (POST
// /api/peer/vms/{name}/stop).
type StopVMRequest struct {
	Name string `json:"-"`
}

func (r StopVMRequest) Validate() error {
	return validateVMName(r.Name)
}

func (api *API) StopVM(ctx context.Context, requester interface{}, request *StopVMRequest) (*VMResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.local.Stop(ctx, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.stopForward(request.Name)

	return &VMResponse{Name: request.Name, Status: sandboxes.StatusStopped}, nil
}

// RemoveVMRequest deletes a stopped VM and its disk; its volumes stay on
// this server (DELETE /api/peer/vms/{name}).
type RemoveVMRequest struct {
	Name string `json:"-"`
}

func (r RemoveVMRequest) Validate() error {
	return validateVMName(r.Name)
}

func (api *API) RemoveVM(ctx context.Context, requester interface{}, request *RemoveVMRequest) (*VMResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.local.Remove(ctx, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.stopForward(request.Name)
	api.removeSocket(request.Name)

	return &VMResponse{Name: request.Name, Status: sandboxes.StatusDeleted}, nil
}

// ExecRequest is one executor call in a running VM (POST
// /api/peer/vms/{name}/exec): Op says which; Command is for run and
// start_terminal, Path for the file calls and look_path (the program's
// name), Data for write_file.
type ExecRequest struct {
	Name    string   `json:"-"`
	Op      string   `json:"op"`
	Command *Command `json:"command,omitempty"`
	Path    string   `json:"path,omitempty"`
	Data    []byte   `json:"data,omitempty"`
}

func (r ExecRequest) Validate() error {
	err := validateVMName(r.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	switch r.Op {
	case OpRun, OpStartTerminal:
		if r.Command == nil || r.Command.Name == "" {
			return ez.New(ez.EINVALID, r.Op+" needs command.name", nil)
		}
	case OpLookPath, OpWriteFile, OpReadFile, OpRemoveFile:
		if r.Path == "" {
			return ez.New(ez.EINVALID, r.Op+" needs path", nil)
		}
	case OpListTerminals:
	default:
		return ez.New(ez.EINVALID, "op "+r.Op+" is not one of run, look_path, write_file, read_file, remove_file, start_terminal, list_terminals", nil)
	}

	return nil
}

// ExecResponse carries what the op returns: result for run, data for
// read_file, terminal for start_terminal, terminals for list_terminals.
type ExecResponse struct {
	Result    *Result    `json:"result,omitempty"`
	Data      []byte     `json:"data,omitempty"`
	Terminal  *Terminal  `json:"terminal,omitempty"`
	Terminals []Terminal `json:"terminals,omitempty"`
}

// Exec connects to the VM, makes the call and releases the handle.
func (api *API) Exec(ctx context.Context, requester interface{}, request *ExecRequest) (*ExecResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	sb, err := api.local.Connect(ctx, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	defer func() {
		_ = sb.Detach(context.WithoutCancel(ctx))
	}()

	return execOn(ctx, sb, request)
}

func execOn(ctx context.Context, sb executor.TerminalExecutor, request *ExecRequest) (*ExecResponse, error) {
	switch request.Op {
	case OpRun:
		result, err := sb.Run(ctx, request.Command.executor())
		if err != nil {
			return nil, ez.Wrap(err)
		}
		return &ExecResponse{Result: &Result{Stdout: result.Stdout, Stderr: result.Stderr, ExitCode: result.ExitCode}}, nil
	case OpLookPath:
		return &ExecResponse{}, sb.LookPath(ctx, request.Path)
	case OpWriteFile:
		return &ExecResponse{}, sb.WriteFile(ctx, request.Path, request.Data)
	case OpReadFile:
		data, err := sb.ReadFile(ctx, request.Path)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		return &ExecResponse{Data: data}, nil
	case OpRemoveFile:
		return &ExecResponse{}, sb.RemoveFile(ctx, request.Path)
	case OpStartTerminal:
		terminal, err := sb.StartTerminal(ctx, request.Command.executor())
		if err != nil {
			return nil, ez.Wrap(err)
		}
		wire := toTerminal(*terminal)
		return &ExecResponse{Terminal: &wire}, nil
	default:
		terminals, err := sb.ListTerminals(ctx)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		wire := []Terminal{}
		for _, terminal := range terminals {
			wire = append(wire, toTerminal(terminal))
		}
		return &ExecResponse{Terminals: wire}, nil
	}
}

// ConnectTerminal connects to the VM for the terminal WebSocket (GET
// /api/peer/vms/{name}/terminals/{job}/ws). release detaches.
func (api *API) ConnectTerminal(ctx context.Context, name string) (executor.TerminalExecutor, func(), error) {
	err := validateVMName(name)
	if err != nil {
		return nil, nil, ez.Wrap(err)
	}

	sb, err := api.local.Connect(ctx, name)
	if err != nil {
		return nil, nil, ez.Wrap(err)
	}

	release := func() {
		_ = sb.Detach(context.Background())
	}

	return sb, release, nil
}

func (api *API) requirePrimary() error {
	if api.primary == "" {
		return ez.New(ez.EUNAVAILABLE, "this server has no primary: { url } in its config, so a VM here could not reach messaging; set it to the primary's URL", nil)
	}

	return nil
}
