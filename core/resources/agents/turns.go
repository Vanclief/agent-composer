package agents

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/task"
)

// ADR 0016 item 5: an agent's turns are read from the harness session files
// on its volume, on every request; agc keeps no copy and no index.

const (
	// DefaultTurnsLimit and MaxTurnsLimit bound a page of turns.
	DefaultTurnsLimit = 50
	MaxTurnsLimit     = 500
	// MaxToolResult is how much of a tool result a turn keeps, in bytes.
	MaxToolResult = 4096
	// maxAnswerSummary is how many characters of the answer's first line a
	// summary keeps.
	maxAnswerSummary = 200
	// SessionsFile maps agc's `<channel>:<root id>` (or `main`) to a
	// harness session id, relative to the volume (the guest's
	// /home/agent/.harness/agc-sessions.json). Written by the agent loop.
	SessionsFile = "agc-sessions.json"
)

// Tokens are a turn's token counts, summed over its model calls.
type Tokens struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
}

func (t *Tokens) add(o Tokens) {
	t.Input += o.Input
	t.Output += o.Output
	t.CacheRead += o.CacheRead
	t.CacheWrite += o.CacheWrite
}

// ToolCall is one tool the model called in a turn and what came back.
// Result is cut to MaxToolResult bytes, Truncated says so.
type ToolCall struct {
	Name      string          `json:"name"`
	Args      json.RawMessage `json:"args"`
	Result    string          `json:"result"`
	Truncated bool            `json:"truncated"`
	// Error is set when the harness marked the result as an error.
	Error bool `json:"error,omitempty"`
}

// Turn is one user message the harness answered: the text injected, every
// tool call, the final assistant text and, when the file has them, the
// token counts.
type Turn struct {
	N         int        `json:"n"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   time.Time  `json:"ended_at"`
	Session   string     `json:"session"`
	Harness   string     `json:"harness"`
	Channel   string     `json:"channel"`
	Thread    string     `json:"thread"`
	Injected  string     `json:"injected"`
	ToolCalls []ToolCall `json:"tool_calls"`
	Answer    string     `json:"answer"`
	Tokens    *Tokens    `json:"tokens"`
}

// TurnSummary is a turn in a list: the tool calls counted and the answer's
// first line.
type TurnSummary struct {
	N         int       `json:"n"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Session   string    `json:"session"`
	Harness   string    `json:"harness"`
	Channel   string    `json:"channel"`
	Thread    string    `json:"thread"`
	ToolCalls int       `json:"tool_calls"`
	Answer    string    `json:"answer"`
	Tokens    *Tokens   `json:"tokens"`
}

// Summary is the turn as a list shows it.
func (t Turn) Summary() TurnSummary {
	return TurnSummary{
		N:         t.N,
		StartedAt: t.StartedAt,
		EndedAt:   t.EndedAt,
		Session:   t.Session,
		Harness:   t.Harness,
		Channel:   t.Channel,
		Thread:    t.Thread,
		ToolCalls: len(t.ToolCalls),
		Answer:    firstLine(t.Answer, maxAnswerSummary),
		Tokens:    t.Tokens,
	}
}

// SandboxVolumes finds a task sandbox's volume directory on the host;
// *sandboxes.API is one.
type SandboxVolumes interface {
	VolumeDir(ctx context.Context, id uuid.UUID, name string) (string, error)
}

// TurnsAPI serves the turns of agents (volume agc-<agent>) and of task
// sandboxes (the sandbox's volume).
type TurnsAPI struct {
	volumeRoot string
	sandboxes  SandboxVolumes
}

// NewTurnsAPI reads agent volumes under volumeRoot (msb's volume directory,
// sandboxes.VolumeRoot) and sandbox volumes through sandboxes.
func NewTurnsAPI(volumeRoot string, sandboxes SandboxVolumes) *TurnsAPI {
	return &TurnsAPI{volumeRoot: volumeRoot, sandboxes: sandboxes}
}

type ListTurnsResponse struct {
	Turns []TurnSummary `json:"turns"`
	// Skipped counts the lines of the session files that are not JSON
	// objects (a torn write, a corrupt file): they are left out.
	Skipped int `json:"skipped"`
}

// TurnsPage is the paging both list requests share.
type TurnsPage struct {
	// Limit is how many turns to return, newest first (default 50).
	Limit int `json:"limit"`
	// Before, when set, returns only turns with n < Before.
	Before int `json:"before"`
}

func (p TurnsPage) validate() error {
	if p.Limit < 0 || p.Limit > MaxTurnsLimit {
		return ez.New(ez.EINVALID, "limit must be between 1 and "+strconv.Itoa(MaxTurnsLimit), nil)
	}

	if p.Before < 0 {
		return ez.New(ez.EINVALID, "before must be a turn number", nil)
	}

	return nil
}

type ListTurnsRequest struct {
	Name string `json:"name"`
	TurnsPage
}

func (r ListTurnsRequest) Validate() error {
	err := validateAgentName(r.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	return r.TurnsPage.validate()
}

type GetTurnRequest struct {
	Name string `json:"name"`
	N    int    `json:"n"`
}

func (r GetTurnRequest) Validate() error {
	err := validateAgentName(r.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	return validateTurnNumber(r.N)
}

// ListTurns lists the agent's turns, newest first, read from the session
// files on its volume agc-<name>.
func (api *TurnsAPI) ListTurns(ctx context.Context, requester interface{}, request *ListTurnsRequest) (*ListTurnsResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	dir, err := api.agentVolume(request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return listTurns(dir, request.TurnsPage)
}

// GetTurn returns one of the agent's turns in full.
func (api *TurnsAPI) GetTurn(ctx context.Context, requester interface{}, request *GetTurnRequest) (*Turn, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	dir, err := api.agentVolume(request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return getTurn(dir, request.N)
}

type ListSandboxTurnsRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	TurnsPage
}

func (r ListSandboxTurnsRequest) Validate() error {
	err := validateSandboxRef(r.ID, r.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	return r.TurnsPage.validate()
}

type GetSandboxTurnRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	N    int       `json:"n"`
}

func (r GetSandboxTurnRequest) Validate() error {
	err := validateSandboxRef(r.ID, r.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	return validateTurnNumber(r.N)
}

// ListSandboxTurns lists a task sandbox's turns, newest first, from its
// volume, so a finished task's reasoning reads like an agent's.
func (api *TurnsAPI) ListSandboxTurns(ctx context.Context, requester interface{}, request *ListSandboxTurnsRequest) (*ListTurnsResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	dir, err := api.sandboxes.VolumeDir(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return listTurns(dir, request.TurnsPage)
}

// GetSandboxTurn returns one of a task sandbox's turns in full.
func (api *TurnsAPI) GetSandboxTurn(ctx context.Context, requester interface{}, request *GetSandboxTurnRequest) (*Turn, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	dir, err := api.sandboxes.VolumeDir(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return getTurn(dir, request.N)
}

// agentVolume is the host directory of the agent's volume, agc-<name> by
// construction (ADR 0016 item 5), ENOTFOUND when it is not on the host.
func (api *TurnsAPI) agentVolume(name string) (string, error) {
	volume := task.VolumeName(name)
	dir := filepath.Join(api.volumeRoot, volume)

	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return "", ez.New(ez.ENOTFOUND, "agent "+name+" has no volume "+volume, err)
	}
	if err != nil {
		return "", ez.New(ez.EINTERNAL, "cannot read volume "+volume, err)
	}
	if !info.IsDir() {
		return "", ez.New(ez.ENOTFOUND, "agent "+name+" has no volume "+volume, nil)
	}

	return dir, nil
}

// validateAgentName keeps a name from the URL from leaving the volume root.
func validateAgentName(name string) error {
	if name == "" {
		return ez.New(ez.EINVALID, "name is required", nil)
	}

	if !fs.ValidPath(name) || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return ez.New(ez.EINVALID, "name "+name+" is not a valid agent name", nil)
	}

	return nil
}

func validateSandboxRef(id uuid.UUID, name string) error {
	if id == uuid.Nil && name == "" {
		return ez.New(ez.EINVALID, "id or name is required", nil)
	}

	if id != uuid.Nil && name != "" {
		return ez.New(ez.EINVALID, "pass only one of id or name", nil)
	}

	return nil
}

func validateTurnNumber(n int) error {
	if n < 1 {
		return ez.New(ez.EINVALID, "n must be a turn number, 1 or more", nil)
	}

	return nil
}

// firstLine is the first non-empty line of s, cut to max characters.
func firstLine(s string, max int) string {
	s = strings.TrimSpace(s)
	line, _, _ := strings.Cut(s, "\n")
	line = strings.TrimSpace(line)

	runes := []rune(line)
	if len(runes) > max {
		return string(runes[:max])
	}

	return line
}
