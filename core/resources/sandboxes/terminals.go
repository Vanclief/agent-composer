package sandboxes

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/executor"
)

// Terminal is one interactive job in a sandbox.
type Terminal struct {
	ID      string   `json:"id"`
	Command []string `json:"command"`
	Running bool     `json:"running"`
}

// TerminalExecutor finds a sandbox by id or name and connects to its VM.
// release detaches the handle. The VM and its jobs keep running.
func (api *API) TerminalExecutor(ctx context.Context, id uuid.UUID, name string) (executor.TerminalExecutor, func(), error) {
	record, err := api.find(ctx, id, name)
	if err != nil {
		return nil, nil, ez.Wrap(err)
	}

	sb, err := api.Executor(ctx, record.Name)
	if err != nil {
		return nil, nil, ez.Wrap(err)
	}

	release := func() {
		err := sb.Detach(context.Background())
		if err != nil {
			log.Warn().Err(err).Str("sandbox", record.Name).Msg("cannot release sandbox handle")
		}
	}

	return sb, release, nil
}

type ListTerminalsRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (r ListTerminalsRequest) Validate() error {
	return validateRef(r.ID, r.Name)
}

type ListTerminalsResponse struct {
	Terminals []Terminal `json:"terminals"`
}

// ListTerminals lists the jobs in a sandbox as the runtime reports them.
func (api *API) ListTerminals(ctx context.Context, requester interface{}, request *ListTerminalsRequest) (*ListTerminalsResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	exe, release, err := api.TerminalExecutor(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	defer release()

	terminals, err := exe.ListTerminals(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	views := []Terminal{}
	for _, terminal := range terminals {
		views = append(views, Terminal{ID: terminal.ID, Command: terminal.Command, Running: terminal.Running})
	}

	return &ListTerminalsResponse{Terminals: views}, nil
}

// StartTerminalRequest names the sandbox by ID or Name, from the path.
type StartTerminalRequest struct {
	ID   uuid.UUID `json:"-"`
	Name string    `json:"-"`
	// Command is split on whitespace, e.g. "claude --resume".
	Command string `json:"command"`
	// Dir is the working directory. Empty means the VM user's default.
	Dir string `json:"dir"`
}

func (r StartTerminalRequest) Validate() error {
	err := validateRef(r.ID, r.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	if len(strings.Fields(r.Command)) == 0 {
		return ez.New(ez.EINVALID, "command is required", nil)
	}

	return nil
}

type StartTerminalResponse struct {
	ID string `json:"id"`
}

// StartTerminal starts a TTY job in a sandbox that outlives the request.
func (api *API) StartTerminal(ctx context.Context, requester interface{}, request *StartTerminalRequest) (*StartTerminalResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	exe, release, err := api.TerminalExecutor(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	defer release()

	fields := strings.Fields(request.Command)
	terminal, err := exe.StartTerminal(ctx, executor.Command{Name: fields[0], Args: fields[1:], Dir: request.Dir})
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &StartTerminalResponse{ID: terminal.ID}, nil
}
