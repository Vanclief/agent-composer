package controller

import (
	"testing"
	"time"
)

func TestResolveWatchdog(t *testing.T) {
	t.Setenv("AGC_WATCHDOG_SILENCE", "")
	t.Setenv("AGC_WATCHDOG_REVIEW", "")

	got := ResolveWatchdog(WatchdogSettings{})
	want := Watchdog{Silence: 15 * time.Minute, Review: 2 * time.Hour}
	if got != want {
		t.Fatalf("built-in watchdog = %+v, want %+v", got, want)
	}

	got = ResolveWatchdog(WatchdogSettings{Silence: "30m", Review: "0"})
	want = Watchdog{Silence: 30 * time.Minute, Review: 0}
	if got != want {
		t.Fatalf("configured watchdog = %+v, want %+v", got, want)
	}

	t.Setenv("AGC_WATCHDOG_SILENCE", "5m")
	got = ResolveWatchdog(WatchdogSettings{Silence: "30m", Review: "soon"})
	want = Watchdog{Silence: 5 * time.Minute, Review: 2 * time.Hour}
	if got != want {
		t.Fatalf("env watchdog = %+v, want %+v", got, want)
	}
}
