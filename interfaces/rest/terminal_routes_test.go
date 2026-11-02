package rest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/executor/sandbox"
	"github.com/vanclief/agent-composer/interfaces/rest/handler"
	restserver "github.com/vanclief/agent-composer/interfaces/rest/server"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// fakeRuntime connects every task to one fakeSandbox.
type fakeRuntime struct {
	sb *fakeSandbox
}

func (f *fakeRuntime) Create(ctx context.Context, spec sandbox.Spec) (sandboxes.Sandbox, error) {
	return f.sb, nil
}

func (f *fakeRuntime) Connect(ctx context.Context, name string) (sandboxes.Sandbox, error) {
	return f.sb, nil
}

func (f *fakeRuntime) Start(ctx context.Context, name string, secrets []sandbox.Secret) error {
	return nil
}

func (f *fakeRuntime) Stop(ctx context.Context, name string) error {
	return nil
}

func (f *fakeRuntime) Remove(ctx context.Context, name string) error {
	return nil
}

func (f *fakeRuntime) Status(ctx context.Context, name string) (string, error) {
	return "stopped", nil
}

// fakeSandbox implements Run, WriteFile, the terminal methods and Detach.
// Any other method panics on the nil embedded interface.
type fakeSandbox struct {
	executor.TerminalExecutor

	mu       sync.Mutex
	started  []executor.Command
	detaches int
}

// Run succeeds, so a boot can write the agent config.
func (s *fakeSandbox) Run(ctx context.Context, cmd executor.Command) (*executor.Result, error) {
	return &executor.Result{}, nil
}

// WriteFile succeeds, so a boot can write TASK.md.
func (s *fakeSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	return nil
}

func (s *fakeSandbox) StartTerminal(ctx context.Context, cmd executor.Command) (*executor.Terminal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.started = append(s.started, cmd)
	return &executor.Terminal{ID: "job_2", Command: append([]string{cmd.Name}, cmd.Args...), Running: true}, nil
}

func (s *fakeSandbox) ListTerminals(ctx context.Context) ([]executor.Terminal, error) {
	return []executor.Terminal{{ID: "job_1", Command: []string{"bash"}, Running: true}}, nil
}

func (s *fakeSandbox) Detach(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.detaches++
	return nil
}

func TestTerminalRoutes(t *testing.T) {
	sb := &fakeSandbox{}
	routes := newTaskRoutes(t, &fakeRuntime{sb: sb}, "")
	e := routes.e

	owner := &taskmodels.Task{Title: "Review 42", Name: "review-42", Brief: "b"}
	err := owner.Insert(context.Background(), routes.db)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	record := &taskmodels.Sandbox{TaskID: owner.ID, Name: "review-42", Spec: []byte(`{}`), BootedAt: &now}
	err = record.Insert(context.Background(), routes.db)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{http.MethodGet, "/api/sandboxes/review-42/terminals", "", http.StatusOK, `{"terminals":[{"id":"job_1","command":["bash"],"running":true}]}`},
		{http.MethodGet, "/api/sandboxes/" + record.ID.String() + "/terminals", "", http.StatusOK, `"id":"job_1"`},
		{http.MethodGet, "/api/sandboxes/missing/terminals", "", http.StatusNotFound, "sandbox missing not found"},
		{http.MethodPost, "/api/sandboxes/review-42/terminals", `{"command":"claude --resume","dir":"/home/agent/x"}`, http.StatusOK, `{"id":"job_2"}`},
		{http.MethodPost, "/api/sandboxes/review-42/terminals", `{"command":"  "}`, http.StatusBadRequest, "command is required"},
	}

	for _, tc := range cases {
		request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		e.ServeHTTP(recorder, request)

		if recorder.Code != tc.status {
			t.Fatalf("%s %s: status %d, body %s", tc.method, tc.path, recorder.Code, recorder.Body.String())
		}

		if !strings.Contains(recorder.Body.String(), tc.want) {
			t.Fatalf("%s %s: body %s does not contain %s", tc.method, tc.path, recorder.Body.String(), tc.want)
		}
	}

	want := []executor.Command{{Name: "claude", Args: []string{"--resume"}, Dir: "/home/agent/x"}}
	if !reflect.DeepEqual(sb.started, want) {
		t.Fatalf("started %+v, want %+v", sb.started, want)
	}

	// Every route that connected also released its handle.
	if sb.detaches != 3 {
		t.Fatalf("detaches = %d, want 3", sb.detaches)
	}
}

// fakeStream emits two chunks, then waits for a resize and reports the
// program as exited.
type fakeStream struct {
	chunks [][]byte
	input  chan []byte
	sizes  chan [2]uint16
	exited chan struct{}
	closed chan struct{}
}

func (s *fakeStream) Recv(ctx context.Context) ([]byte, error) {
	if len(s.chunks) > 0 {
		chunk := s.chunks[0]
		s.chunks = s.chunks[1:]
		return chunk, nil
	}

	select {
	case <-s.exited:
		return nil, io.EOF
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *fakeStream) Write(ctx context.Context, data []byte) error {
	s.input <- data
	return nil
}

func (s *fakeStream) Resize(ctx context.Context, rows uint16, cols uint16) error {
	s.sizes <- [2]uint16{rows, cols}
	close(s.exited)
	return nil
}

func (s *fakeStream) Close() error {
	close(s.closed)
	return nil
}

type fakeTerminal struct {
	executor.TerminalExecutor
	stream *fakeStream
}

func (f *fakeTerminal) AttachTerminal(ctx context.Context, id string, replayBytes int) (executor.TerminalStream, error) {
	if id != "job_1" {
		return nil, ez.New(ez.ENOTFOUND, "cannot find job "+id, nil)
	}
	if replayBytes != 64*1024 {
		return nil, ez.New(ez.EINVALID, "unexpected replay size", nil)
	}

	return f.stream, nil
}

func TestTerminalSocket(t *testing.T) {
	stream := &fakeStream{
		chunks: [][]byte{[]byte("hello "), []byte("world")},
		input:  make(chan []byte, 1),
		sizes:  make(chan [2]uint16, 1),
		exited: make(chan struct{}),
		closed: make(chan struct{}),
	}
	released := make(chan string, 1)

	h := handler.NewHandler(&restserver.Server{})
	h.ConnectTerminal = func(ctx context.Context, ref string) (executor.TerminalExecutor, func(), error) {
		return &fakeTerminal{stream: stream}, func() { released <- ref }, nil
	}
	e := echo.New()
	addAPIRoutes(e, h)
	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/sandboxes/review-42/terminals/job_1/ws"
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()

	for _, want := range []string{"hello ", "world"} {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if typ != websocket.MessageBinary || string(data) != want {
			t.Fatalf("got %v %q, want binary %q", typ, data, want)
		}
	}

	err = conn.Write(ctx, websocket.MessageBinary, []byte("ls\n"))
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"resize","rows":40,"cols":120}`))
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = conn.Read(ctx)
	var closeErr websocket.CloseError
	if !errors.As(err, &closeErr) {
		t.Fatalf("read after exit: %v, want a close frame", err)
	}
	if closeErr.Code != websocket.StatusNormalClosure || closeErr.Reason != "job completed" {
		t.Fatalf("close = %d %q, want normal closure \"job completed\"", closeErr.Code, closeErr.Reason)
	}

	if string(<-stream.input) != "ls\n" {
		t.Fatal("input did not reach the terminal")
	}

	size := <-stream.sizes
	if size != [2]uint16{40, 120} {
		t.Fatalf("resized to %v, want 40x120", size)
	}

	select {
	case ref := <-released:
		if ref != "review-42" {
			t.Fatalf("released %q, want review-42", ref)
		}
	case <-ctx.Done():
		t.Fatal("handle was not released")
	}

	select {
	case <-stream.closed:
	case <-ctx.Done():
		t.Fatal("stream was not closed")
	}
}
