package controller

import "testing"

func TestResolveDefaults(t *testing.T) {
	t.Setenv("AGC_DEFAULT_HARNESS", "")
	t.Setenv("AGC_DEFAULT_MODEL", "")
	t.Setenv("AGC_DEFAULT_OWNER", "")

	got := ResolveDefaults(DefaultsSettings{})
	want := DefaultsSettings{Harness: "claude_code", Model: "claude-opus-5-5"}
	if got != want {
		t.Fatalf("built-in defaults = %+v, want %+v", got, want)
	}

	got = ResolveDefaults(DefaultsSettings{Harness: "codex_cli", Model: "gpt-5.5", Owner: "franco"})
	want = DefaultsSettings{Harness: "codex_cli", Model: "gpt-5.5", Owner: "franco"}
	if got != want {
		t.Fatalf("configured defaults = %+v, want %+v", got, want)
	}

	t.Setenv("AGC_DEFAULT_OWNER", "ana")
	t.Setenv("AGC_DEFAULT_MODEL", "claude-sonnet-5")

	got = ResolveDefaults(DefaultsSettings{Harness: "codex_cli", Model: "gpt-5.5", Owner: "franco"})
	want = DefaultsSettings{Harness: "codex_cli", Model: "claude-sonnet-5", Owner: "ana"}
	if got != want {
		t.Fatalf("env defaults = %+v, want %+v", got, want)
	}
}
