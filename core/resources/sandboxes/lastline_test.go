package sandboxes

import (
	"strings"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/executor"
)

func TestLastLine(t *testing.T) {
	cases := []struct {
		name string
		data string
		want string
	}{
		{"plain", "one\ntwo\n", "two"},
		{"blank tail", "one\ntwo\n\n  \n", "two"},
		{"colors", "\x1b[1;32mready\x1b[0m\n", "ready"},
		{"title", "\x1b]0;claude\x07> \n", ">"},
		{"redraw", "50%\r75%\r100%\r\n", "100%"},
		{"crlf", "done\r\n", "done"},
		{"empty", "", ""},
		{"long", strings.Repeat("x", 200), strings.Repeat("x", maxLineRunes) + "…"},
	}

	for _, tc := range cases {
		got := LastLine([]byte(tc.data))
		if got != tc.want {
			t.Errorf("%s: LastLine = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestNewestWithCommand(t *testing.T) {
	base := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	jobs := []executor.Job{
		{ID: "old", Command: []string{"agc", "agent", "run"}, CreatedAt: base},
		{ID: "shell", Command: []string{"bash"}, CreatedAt: base.Add(2 * time.Hour)},
		{ID: "new", Command: []string{"agc", "agent", "run"}, CreatedAt: base.Add(time.Hour)},
	}

	got := newestWithCommand(jobs, []string{"agc", "agent", "run"})
	if got == nil || got.ID != "new" {
		t.Fatalf("newestWithCommand = %+v, want job new", got)
	}

	if newestWithCommand(jobs, []string{"claude"}) != nil {
		t.Fatalf("newestWithCommand found a job for a command nobody runs")
	}
}
