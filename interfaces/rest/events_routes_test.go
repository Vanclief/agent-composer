package rest

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/messaging"
)

// sseEvent is one server-sent event as the browser sees it.
type sseEvent struct {
	ID   string
	Type string
	Data string
}

// eventStream is an open GET /api/events.
type eventStream struct {
	events  <-chan sseEvent
	cancel  context.CancelFunc
	backlog []sseEvent
}

// openEvents opens GET /api/events on a real listener and parses frames
// until the stream ends. It returns once the stream is subscribed.
func openEvents(t *testing.T, routes *taskRoutes) *eventStream {
	t.Helper()

	srv := httptest.NewServer(routes.e)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		srv.Close()
	})

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}

	response, err := srv.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status %d", response.StatusCode)
	}
	if response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content type %q", response.Header.Get("Content-Type"))
	}
	if response.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("cache control %q", response.Header.Get("Cache-Control"))
	}

	reader := bufio.NewReader(response.Body)

	// The first frame is a comment, written after the subscription.
	line, err := reader.ReadString('\n')
	if err != nil || !strings.HasPrefix(line, ":") {
		t.Fatalf("first line %q, err %v", line, err)
	}

	out := make(chan sseEvent, 64)
	go func() {
		defer close(out)
		defer response.Body.Close()

		var event sseEvent
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}

			line = strings.TrimRight(line, "\n")
			switch {
			case line == "":
				if event.Type != "" {
					out <- event
				}
				event = sseEvent{}
			case strings.HasPrefix(line, "id: "):
				event.ID = strings.TrimPrefix(line, "id: ")
			case strings.HasPrefix(line, "event: "):
				event.Type = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				event.Data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()

	return &eventStream{events: out, cancel: cancel}
}

// next returns the next event of type typ, in order within its type.
// Events of other types read meanwhile are kept for later calls, since a
// background boot may publish before or after the request's own events.
func (s *eventStream) next(t *testing.T, typ string, into any) sseEvent {
	t.Helper()

	event, found := s.take(typ)
	timeout := time.After(5 * time.Second)
	for !found {
		select {
		case read, ok := <-s.events:
			if !ok {
				t.Fatalf("stream ended before %s", typ)
			}
			if read.Type == typ {
				event = read
				found = true
				continue
			}
			s.backlog = append(s.backlog, read)
		case <-timeout:
			t.Fatalf("no %s event", typ)
		}
	}

	if event.ID == "" {
		t.Fatalf("%s has no id", typ)
	}

	err := json.Unmarshal([]byte(event.Data), into)
	if err != nil {
		t.Fatalf("%s data %s: %v", typ, event.Data, err)
	}

	return event
}

// take removes and returns the first kept event of type typ.
func (s *eventStream) take(typ string) (sseEvent, bool) {
	for i, event := range s.backlog {
		if event.Type == typ {
			s.backlog = append(s.backlog[:i], s.backlog[i+1:]...)
			return event, true
		}
	}

	return sseEvent{}, false
}

func TestEventsTaskCreated(t *testing.T) {
	routes := newTaskRoutes(t, sandboxes.SandboxRuntime{}, "")
	stream := openEvents(t, routes)

	status, body := routes.do(http.MethodPost, "/api/tasks", `{"title":"Fix login","brief":"Make 2FA work.","manager":"orchestrator"}`)
	if status != http.StatusOK {
		t.Fatalf("status %d, body %s", status, body)
	}

	// agc's opening message comes first, then the task.
	var posted messaging.PostedEvent
	stream.next(t, "message.posted", &posted)
	if posted.Message.Channel != "#fix-login" || posted.Message.From != "agc" {
		t.Fatalf("message.posted %+v", posted.Message)
	}

	var created tasks.CreatedEvent
	stream.next(t, "task.created", &created)
	if created.Task.Name != "fix-login" || created.Task.Manager != "orchestrator" || !created.Task.Notify {
		t.Fatalf("task.created %+v", created.Task)
	}
	if created.Task.Sandboxes == nil {
		t.Fatal("task.created has null sandboxes")
	}

	// The stream ends, and unsubscribes, when the client goes away.
	stream.cancel()
	deadline := time.Now().Add(5 * time.Second)
	for routes.server.Events.Subscribers() != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%d subscribers after the client left", routes.server.Events.Subscribers())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestEventsSandboxStatusAndTaskDone(t *testing.T) {
	routes := newTaskRoutes(t, &fakeRuntime{sb: &fakeSandbox{}}, "")
	stream := openEvents(t, routes)

	status, body := routes.do(http.MethodPost, "/api/tasks", `{"title":"Fix login","manager":"orchestrator","notify":false,"sandbox":{"command":"true"}}`)
	if status != http.StatusOK {
		t.Fatalf("status %d, body %s", status, body)
	}

	var starting sandboxes.StatusEvent
	stream.next(t, "sandbox.status", &starting)
	if starting.Sandbox.Name != "fix-login" || starting.Sandbox.Status != "starting" {
		t.Fatalf("sandbox.status %+v", starting.Sandbox)
	}

	var created tasks.CreatedEvent
	stream.next(t, "task.created", &created)
	if created.Task.Notify || len(created.Task.Sandboxes) != 1 {
		t.Fatalf("task.created %+v", created.Task)
	}
	routes.check(t, []routeCase{
		{http.MethodGet, "/api/tasks/fix-login", "", http.StatusOK, `"notify":false`},
	})

	// The fake runtime says stopped once booted.
	var booted sandboxes.StatusEvent
	stream.next(t, "sandbox.status", &booted)
	if booted.Sandbox.Status != "stopped" || booted.Sandbox.BootedAt == nil {
		t.Fatalf("sandbox.status after boot %+v", booted.Sandbox)
	}

	// The agent's closing words, longer than the summary keeps, then a
	// human's reply: the summary is the agent's, cut to 500 characters.
	msg := routes.server.Messaging
	ctx := context.Background()
	agentCaller := messaging.Caller{Task: "fix-login"}
	agent, err := msg.Join(ctx, agentCaller, "fixer")
	if err != nil {
		t.Fatal(err)
	}
	closing := strings.Repeat("é", 600)
	_, err = msg.Post(ctx, agentCaller, agent.ID, "#fix-login", closing, nil)
	if err != nil {
		t.Fatal(err)
	}
	humanCaller := messaging.Caller{Human: true}
	franco, err := msg.Join(ctx, humanCaller, "franco")
	if err != nil {
		t.Fatal(err)
	}
	_, err = msg.Post(ctx, humanCaller, franco.ID, "#fix-login", "thanks", nil)
	if err != nil {
		t.Fatal(err)
	}

	status, body = routes.do(http.MethodPost, "/api/tasks/fix-login/done", "")
	if status != http.StatusOK {
		t.Fatalf("status %d, body %s", status, body)
	}

	var done tasks.DoneEvent
	stream.next(t, "task.done", &done)
	if done.Task.Status != "done" || done.Summary != strings.Repeat("é", 500) {
		t.Fatalf("task.done %s, summary of %d runes", done.Task.Status, len([]rune(done.Summary)))
	}

	status, body = routes.do(http.MethodPost, "/api/sandboxes/fix-login/stop", "")
	if status != http.StatusOK {
		t.Fatalf("status %d, body %s", status, body)
	}

	var stopped sandboxes.StatusEvent
	stream.next(t, "sandbox.status", &stopped)
	if stopped.Sandbox.StoppedAt == nil {
		t.Fatalf("sandbox.status after stop %+v", stopped.Sandbox)
	}

	status, body = routes.do(http.MethodDelete, "/api/sandboxes/fix-login", "")
	if status != http.StatusOK {
		t.Fatalf("status %d, body %s", status, body)
	}

	var deleted sandboxes.StatusEvent
	stream.next(t, "sandbox.status", &deleted)
	if deleted.Sandbox.Status != "deleted" || deleted.Sandbox.Name != "fix-login" {
		t.Fatalf("sandbox.status after delete %+v", deleted.Sandbox)
	}
}

func TestEventsDoneSummaryEmptyWithoutAgent(t *testing.T) {
	routes := newTaskRoutes(t, sandboxes.SandboxRuntime{}, "")
	stream := openEvents(t, routes)

	status, body := routes.do(http.MethodPost, "/api/tasks", `{"title":"Fix login","brief":"b","manager":"orchestrator"}`)
	if status != http.StatusOK {
		t.Fatalf("status %d, body %s", status, body)
	}

	// The manager, an agent, closes it with /done: a command, not a
	// summary, and agc's own messages never count.
	ctx := context.Background()
	managerCaller := messaging.Caller{Task: "lead", Grants: []messaging.Grant{{Channel: "#fix-login"}}}
	manager, err := routes.server.Messaging.Join(ctx, managerCaller, "orchestrator")
	if err != nil {
		t.Fatal(err)
	}
	_, err = routes.server.Messaging.Post(ctx, managerCaller, manager.ID, "#fix-login", "/done", nil)
	if err != nil {
		t.Fatal(err)
	}

	var done tasks.DoneEvent
	stream.next(t, "task.done", &done)
	if done.Task.Name != "fix-login" || done.Task.DoneAt == nil || done.Summary != "" {
		t.Fatalf("task.done %+v, summary %q", done.Task, done.Summary)
	}

	// Done again publishes nothing: the next task.done would be a new one.
	status, _ = routes.do(http.MethodPost, "/api/tasks/fix-login/done", "")
	if status != http.StatusOK {
		t.Fatalf("status %d", status)
	}
	timeout := time.After(200 * time.Millisecond)
	waiting := true
	for waiting {
		select {
		case event := <-stream.events:
			stream.backlog = append(stream.backlog, event)
		case <-timeout:
			waiting = false
		}
	}
	_, again := stream.take("task.done")
	if again {
		t.Fatal("a second task.done for a task already done")
	}
}
