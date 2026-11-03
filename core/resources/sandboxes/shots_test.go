package sandboxes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging/httpapi"
)

// A shot larger than vsock lets one body be goes in pieces, and the
// delivery that names it takes it (ADR 0010, "Delivery report").
func TestListenerTakesShotsInPieces(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	insertTask(t, env.db, "review-42")

	_, err := env.api.Allocate(ctx, nil, &AllocateRequest{TaskName: "review-42"})
	if err != nil {
		t.Fatal(err)
	}
	env.api.Wait()

	var got map[string][]byte
	env.api.OnDeliver(func(ctx context.Context, taskName string, summary string, report json.RawMessage, files map[string][]byte) (any, error) {
		got = files
		return map[string]string{"task": taskName}, nil
	})

	client := socketClient(env.api.socketPath("review-42"))

	before := bytes.Repeat([]byte("b"), 1500*1000)
	after := bytes.Repeat([]byte("a"), 100*1000)

	err = client.SendShot(ctx, "before", before)
	if err != nil {
		t.Fatal(err)
	}
	err = client.SendShot(ctx, "after", after)
	if err != nil {
		t.Fatal(err)
	}
	// Sending a shot again starts it over.
	err = client.SendShot(ctx, "after", after)
	if err != nil {
		t.Fatal(err)
	}
	// A shot the report does not name stays out of the delivery.
	err = client.SendShot(ctx, "stray", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}

	report := `{"shots":[{"label":"before","file":"/tmp/before.png"},{"label":"after","file":"/tmp/after.png"}]}`
	out := map[string]string{}
	err = client.Deliver(ctx, "with shots", json.RawMessage(report), nil, &out)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !bytes.Equal(got["before"], before) || !bytes.Equal(got["after"], after) {
		t.Fatalf("delivered %d files: before %d bytes, after %d bytes", len(got), len(got["before"]), len(got["after"]))
	}

	// A delivery takes the shots once.
	err = client.Deliver(ctx, "again", json.RawMessage(report), nil, &out)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("second delivery got %d files", len(got))
	}
}

func TestShotPiecesRules(t *testing.T) {
	pieces := newShotPieces()

	_, err := pieces.add("before", 10, []byte("x"))
	if ez.ErrorCode(err) != ez.ECONFLICT {
		t.Fatalf("a piece past the end: %v, want ECONFLICT", err)
	}

	size, err := pieces.add("before", 0, make([]byte, 2<<20))
	if err != nil || size != 2<<20 {
		t.Fatalf("size %d, err %v", size, err)
	}

	_, err = pieces.add("before", 2<<20, []byte("x"))
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("a shot over 2 MB: %v, want EINVALID", err)
	}

	for i := range 7 {
		_, err = pieces.add(fmt.Sprint("s", i), 0, []byte("x"))
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err = pieces.add("ninth", 0, []byte("x"))
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("a ninth shot: %v, want EINVALID", err)
	}
}

// A delivery whose body trickles in is logged while it is read and once
// it ends.
func TestListenerLogsASlowBodyRead(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	insertTask(t, env.db, "review-42")

	_, err := env.api.Allocate(ctx, nil, &AllocateRequest{TaskName: "review-42"})
	if err != nil {
		t.Fatal(err)
	}
	env.api.Wait()

	env.api.OnDeliver(func(ctx context.Context, taskName string, summary string, report json.RawMessage, files map[string][]byte) (any, error) {
		return map[string]string{"task": taskName}, nil
	})

	var mu sync.Mutex
	var logged []string

	oldSlow := slowBodyRead
	oldLog := logSlowBody
	slowBodyRead = 100 * time.Millisecond
	logSlowBody = func(sandboxName string, path string, read int64, length int64, elapsed time.Duration, done bool) {
		mu.Lock()
		defer mu.Unlock()
		logged = append(logged, fmt.Sprintf("%s %s %d/%d done=%t", sandboxName, path, read, length, done))
	}
	t.Cleanup(func() {
		slowBodyRead = oldSlow
		logSlowBody = oldLog
	})

	conn, err := net.Dial("unix", env.api.socketPath("review-42"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	body := `{"summary":"slow"}`
	head := fmt.Sprintf("POST %s HTTP/1.1\r\nHost: task\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n", httpapi.DeliverPath, len(body))

	_, err = conn.Write([]byte(head + body[:5]))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	_, err = conn.Write([]byte(body[5:]))
	if err != nil {
		t.Fatal(err)
	}

	answer := make([]byte, 512)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(answer)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(answer[:n]), "HTTP/1.1 200") {
		t.Fatalf("answer = %q", answer[:n])
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{
		"review-42 /task/deliver 5/18 done=false",
		"review-42 /task/deliver 18/18 done=true",
	}
	if strings.Join(logged, "\n") != strings.Join(want, "\n") {
		t.Fatalf("logged:\n%s\nwant:\n%s", strings.Join(logged, "\n"), strings.Join(want, "\n"))
	}
}
