package controller

import (
	"testing"
	"time"
)

func TestResolveKeepDone(t *testing.T) {
	t.Setenv("AGC_SANDBOX_KEEP_DONE", "")

	cases := []struct {
		env        string
		configured string
		want       time.Duration
		bad        bool
	}{
		{"", "", 24 * time.Hour, false},
		{"", "48h", 48 * time.Hour, false},
		{"", "0", 0, false},
		{"90m", "48h", 90 * time.Minute, false},
		{"0", "48h", 0, false},
		{"", "soon", 0, true},
		{"-1h", "", 0, true},
	}
	for _, c := range cases {
		t.Setenv("AGC_SANDBOX_KEEP_DONE", c.env)

		got, err := ResolveKeepDone(SandboxesSettings{KeepDone: c.configured})
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
