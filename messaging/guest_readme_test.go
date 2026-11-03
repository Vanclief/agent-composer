package messaging

import (
	"os"
	"strings"
	"testing"
)

// The guest README (/etc/agc/README.md in the image) sends a task's agent
// to its manager for what only the owner can decide, never to #<owner>
// (ADR 0009 item 5).
func TestGuestReadmeSendsTheAgentToItsManager(t *testing.T) {
	data, err := os.ReadFile("guest-readme.md")
	if err != nil {
		t.Fatal(err)
	}

	text := string(data)

	if !strings.Contains(text, "ask your manager there and say it is\nfor the owner; your manager raises it.") {
		t.Fatalf("guest README does not send the agent to its manager:\n%s", text)
	}

	if strings.Contains(text, "`#<owner>`") {
		t.Fatalf("guest README still names #<owner> as a place to post:\n%s", text)
	}
}
