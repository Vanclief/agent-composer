package client

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vanclief/ez"
)

// stallingClient is a client whose connection takes the first limit
// bytes, as the vsock forwarder takes 256 KiB, and then nothing, and never
// answers.
func stallingClient(t *testing.T, limit int64) *Client {
	t.Helper()

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			ours, theirs := net.Pipe()
			go func() {
				_, _ = io.CopyN(io.Discard, theirs, limit)
				<-ctx.Done()
			}()
			t.Cleanup(func() {
				_ = theirs.Close()
			})
			return ours, nil
		},
	}

	return &Client{BaseURL: "http://vsock", HTTP: &http.Client{Transport: transport}, Stall: 200 * time.Millisecond}
}

func TestDeliverFailsWhenTheSocketStopsTakingTheBody(t *testing.T) {
	c := stallingClient(t, 256<<10)

	start := time.Now()
	err := c.Deliver(context.Background(), "with shots", json.RawMessage(`{}`), map[string][]byte{"before": make([]byte, 1200*1000)}, &struct{}{})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("delivered through a stalled socket")
	}
	if ez.ErrorCode(err) != ez.EUNAVAILABLE {
		t.Fatalf("code %s: %v", ez.ErrorCode(err), err)
	}
	message := ez.ErrorMessage(err)
	if !strings.Contains(message, "sending 1.6 MB to the sandbox socket: no progress for 200ms") || !strings.Contains(message, "POST /task/deliver") {
		t.Fatalf("message = %q", message)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("failed after %s", elapsed)
	}
}

func TestSendShotFailsWhenNoAnswerComes(t *testing.T) {
	// The connection takes the whole piece and never answers.
	c := stallingClient(t, 1<<20)

	err := c.SendShot(context.Background(), "before", make([]byte, 100*1000))
	if err == nil {
		t.Fatal("sent a shot to a socket that never answers")
	}
	message := ez.ErrorMessage(err)
	if !strings.Contains(message, "sending shot before (100.0 KB, piece at 0 B) to the sandbox socket: no answer for 200ms") {
		t.Fatalf("message = %q", message)
	}
}

// A long poll is not a stall: a request with no body is never cut short.
func TestWaitIsNotCutShort(t *testing.T) {
	c := stallingClient(t, 1<<20)

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()

	_, err := c.Wait(ctx, "#x", 0, time.Minute)
	if err == nil {
		t.Fatal("expected the context's deadline")
	}
	if strings.Contains(ez.ErrorMessage(err), "no progress") {
		t.Fatalf("a long poll was cut short: %v", err)
	}
}

func TestSize(t *testing.T) {
	cases := map[int]string{512: "512 B", 196608: "196.6 KB", 1600000: "1.6 MB"}
	for n, want := range cases {
		if Size(n) != want {
			t.Errorf("Size(%d) = %q, want %q", n, Size(n), want)
		}
	}
}
