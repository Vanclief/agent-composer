// Package client calls the agc REST API for the task and sandbox commands
// of the CLI, so `agc task …` and `agc sandbox …` work against a remote
// server (AGC_SERVER or --server). Request and response types are the ones
// in core/resources/tasks and core/resources/sandboxes.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/task"
)

// DefaultPollInterval is how often WaitSandbox asks for the sandbox status.
const DefaultPollInterval = 2 * time.Second

// Client talks to one agc server.
type Client struct {
	// BaseURL is the server root, such as http://127.0.0.1:8788. The API
	// lives under /api and the panel under /panel.
	BaseURL string
	HTTP    *http.Client
	// PollInterval is the pause between status reads in WaitSandbox;
	// zero means DefaultPollInterval.
	PollInterval time.Duration
}

// New returns a client for the server at baseURL.
func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		HTTP:    &http.Client{Timeout: 5 * time.Minute},
	}
}

// TaskURL is the panel page of the task called name.
func (c *Client) TaskURL(name string) string {
	return c.BaseURL + "/panel/tasks/" + url.PathEscape(name)
}

// CreateTask creates a task and, when request.Sandbox is set, allocates its
// sandbox. The server answers at once with the sandbox starting.
func (c *Client) CreateTask(ctx context.Context, request *tasks.CreateRequest) (*tasks.TaskView, error) {
	out := &tasks.TaskView{}
	err := c.do(ctx, http.MethodPost, "/api/tasks", request, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// ListTasks lists every task with its sandboxes.
func (c *Client) ListTasks(ctx context.Context) (*tasks.ListResponse, error) {
	out := &tasks.ListResponse{}
	err := c.do(ctx, http.MethodGet, "/api/tasks", nil, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// GetTask reads the task called ref (a name or an id).
func (c *Client) GetTask(ctx context.Context, ref string) (*tasks.TaskView, error) {
	out := &tasks.TaskView{}
	err := c.do(ctx, http.MethodGet, "/api/tasks/"+url.PathEscape(ref), nil, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// DoneTask marks the task done.
func (c *Client) DoneTask(ctx context.Context, ref string) (*tasks.TaskView, error) {
	out := &tasks.TaskView{}
	err := c.do(ctx, http.MethodPost, "/api/tasks/"+url.PathEscape(ref)+"/done", nil, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// DeleteTask deletes the task and its sandboxes.
func (c *Client) DeleteTask(ctx context.Context, ref string) (*tasks.DeleteResponse, error) {
	out := &tasks.DeleteResponse{}
	err := c.do(ctx, http.MethodDelete, "/api/tasks/"+url.PathEscape(ref), nil, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// Allocate allocates a sandbox to the task; it comes back starting.
func (c *Client) Allocate(ctx context.Context, taskRef string, sandbox task.Sandbox) (*sandboxes.SandboxView, error) {
	out := &sandboxes.SandboxView{}
	err := c.do(ctx, http.MethodPost, "/api/tasks/"+url.PathEscape(taskRef)+"/sandboxes", sandboxes.AllocateRequest{Sandbox: sandbox}, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// ListSandboxes lists every sandbox with its live status.
func (c *Client) ListSandboxes(ctx context.Context) (*sandboxes.ListResponse, error) {
	out := &sandboxes.ListResponse{}
	err := c.do(ctx, http.MethodGet, "/api/sandboxes", nil, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// GetSandbox reads the sandbox called ref (a name or an id).
func (c *Client) GetSandbox(ctx context.Context, ref string) (*sandboxes.SandboxView, error) {
	return c.sandboxCall(ctx, http.MethodGet, ref, "")
}

// StopSandbox stops the sandbox and keeps its disk.
func (c *Client) StopSandbox(ctx context.Context, ref string) (*sandboxes.SandboxView, error) {
	return c.sandboxCall(ctx, http.MethodPost, ref, "/stop")
}

// StartSandbox boots a stopped sandbox from its disk.
func (c *Client) StartSandbox(ctx context.Context, ref string) (*sandboxes.SandboxView, error) {
	return c.sandboxCall(ctx, http.MethodPost, ref, "/start")
}

// DeleteSandbox deletes the sandbox and its disk.
func (c *Client) DeleteSandbox(ctx context.Context, ref string) (*sandboxes.DeleteResponse, error) {
	out := &sandboxes.DeleteResponse{}
	err := c.do(ctx, http.MethodDelete, "/api/sandboxes/"+url.PathEscape(ref), nil, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (c *Client) sandboxCall(ctx context.Context, method string, ref string, suffix string) (*sandboxes.SandboxView, error) {
	out := &sandboxes.SandboxView{}
	err := c.do(ctx, method, "/api/sandboxes/"+url.PathEscape(ref)+suffix, nil, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// ListSpecs lists the server's saved sandbox specs.
func (c *Client) ListSpecs(ctx context.Context) (*tasks.ListSpecsResponse, error) {
	out := &tasks.ListSpecsResponse{}
	err := c.do(ctx, http.MethodGet, "/api/specs", nil, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// GetSpec reads the saved spec called name.
func (c *Client) GetSpec(ctx context.Context, name string) (*task.Saved, error) {
	return c.specCall(ctx, http.MethodGet, name, nil)
}

// SaveSpec saves a sandbox spec on the server, replacing one of the same name.
func (c *Client) SaveSpec(ctx context.Context, name string, yaml string) (*task.Saved, error) {
	return c.specCall(ctx, http.MethodPut, name, tasks.SaveSpecRequest{YAML: yaml})
}

// DeleteSpec deletes the saved spec called name.
func (c *Client) DeleteSpec(ctx context.Context, name string) (*task.Saved, error) {
	return c.specCall(ctx, http.MethodDelete, name, nil)
}

func (c *Client) specCall(ctx context.Context, method string, name string, in any) (*task.Saved, error) {
	out := &task.Saved{}
	err := c.do(ctx, method, "/api/specs/"+url.PathEscape(name), in, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// WaitSandbox polls the sandbox until it is running or failed, calling
// onStatus with each status it has not reported before (the first one
// included). It returns the last view read; a failed sandbox is returned
// with its status_error, not as an error.
func (c *Client) WaitSandbox(ctx context.Context, ref string, onStatus func(view *sandboxes.SandboxView)) (*sandboxes.SandboxView, error) {
	interval := c.PollInterval
	if interval <= 0 {
		interval = DefaultPollInterval
	}

	last := ""
	for {
		view, err := c.GetSandbox(ctx, ref)
		if err != nil {
			return nil, err
		}

		if view.Status != last && onStatus != nil {
			onStatus(view)
		}
		last = view.Status

		if view.Status == sandboxes.StatusRunning || view.Status == sandboxes.StatusFailed {
			return view, nil
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ez.Wrap(ctx.Err())
		case <-timer.C:
		}
	}
}

// errorBody is the server's error shape (openapi ErrorResponse). Echo's
// own errors, such as an unknown route, carry only a top-level message.
type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Message string `json:"message"`
}

// do sends one request and decodes the JSON answer into out. A non-2xx
// answer becomes an ez error carrying the server's error.message verbatim.
func (c *Client) do(ctx context.Context, method string, path string, in any, out any) error {
	var body io.Reader
	if in != nil {
		encoded, err := json.Marshal(in)
		if err != nil {
			return ez.Wrap(err)
		}
		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return ez.New(ez.EINVALID, "bad server URL "+c.BaseURL, err)
	}
	request.Header.Set("Accept", "application/json")
	if in != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return ez.New(ez.EUNAVAILABLE, "cannot reach the agc server at "+c.BaseURL+": "+err.Error(), err)
	}
	defer response.Body.Close() // nolint:errcheck // Close errors are not actionable here.

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return ez.New(ez.EUNAVAILABLE, "cannot read the answer from "+c.BaseURL, err)
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return ez.New(ez.HTTPStatusToError(response.StatusCode), errorMessage(response, data), nil)
	}

	if out == nil {
		return nil
	}

	err = json.Unmarshal(data, out)
	if err != nil {
		return ez.New(ez.EINTERNAL, "the server at "+c.BaseURL+" answered "+method+" "+path+" with something other than JSON", err)
	}

	return nil
}

func errorMessage(response *http.Response, data []byte) string {
	failure := errorBody{}
	err := json.Unmarshal(data, &failure)
	if err == nil && failure.Error.Message != "" {
		return failure.Error.Message
	}

	if err == nil && failure.Message != "" {
		return failure.Message
	}

	return response.Status
}
