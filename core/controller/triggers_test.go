package controller

import (
	"testing"
	"time"
)

func TestGitHubTriggers(t *testing.T) {
	off := GitHubTriggers{Repos: []string{" ", ""}}
	if off.Enabled() {
		t.Fatal("blank repos enable triggers")
	}

	every, err := off.PollInterval()
	if err != nil || every != DefaultGitHubPoll {
		t.Fatalf("default poll = %s, %v, want %s", every, err, DefaultGitHubPoll)
	}

	on := GitHubTriggers{Repos: []string{" acme/app "}, Poll: "2m"}
	if !on.Enabled() || on.RepoList()[0] != "acme/app" {
		t.Fatalf("repos = %v, want [acme/app]", on.RepoList())
	}

	every, err = on.PollInterval()
	if err != nil || every != 2*time.Minute {
		t.Fatalf("poll = %s, %v, want 2m", every, err)
	}

	bad := []GitHubTriggers{
		{Repos: []string{"acme"}},
		{Repos: []string{"acme/app/x"}},
		{Repos: []string{"acme/app"}, Poll: "soon"},
		{Repos: []string{"acme/app"}, Poll: "1s"},
	}
	for _, settings := range bad {
		err = settings.Validate()
		if err == nil {
			t.Errorf("%+v validated", settings)
		}
	}
}
