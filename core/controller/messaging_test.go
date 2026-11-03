package controller

import (
	"testing"
	"time"
)

func TestResolveArchiveDone(t *testing.T) {
	cases := []struct {
		env        string
		configured string
		want       time.Duration
		bad        bool
	}{
		{"", "", 168 * time.Hour, false},
		{"", "48h", 48 * time.Hour, false},
		{"", "0", 0, false},
		{"90m", "48h", 90 * time.Minute, false},
		{"0", "48h", 0, false},
		{"", "soon", 0, true},
		{"-1h", "", 0, true},
	}
	for _, c := range cases {
		t.Setenv("AGC_MESSAGING_ARCHIVE_DONE", c.env)

		got, err := ResolveArchiveDone(MessagingSettings{ArchiveDone: c.configured})
		if c.bad {
			if err == nil {
				t.Fatalf("env %q config %q: no error", c.env, c.configured)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Fatalf("env %q config %q = %v, %v, want %v", c.env, c.configured, got, err, c.want)
		}
	}
}
