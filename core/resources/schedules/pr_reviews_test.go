package schedules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	schedulemodels "github.com/vanclief/agent-composer/models/schedule"
)

// TestPRReviewsScheduleRenders creates the production review schedule,
// docs/schedules/pr-reviews.yaml, and checks the task it makes for one
// pull request.
func TestPRReviewsScheduleRenders(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	path := filepath.Join(root, "docs", "schedules", "pr-reviews.yaml")
	_, err := os.Stat(path)
	if err != nil {
		t.Skipf("no %s from here: %v", path, err)
	}

	request, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	env := newTestEnv(t)
	// The repository's skills, so the template's pr-review is found.
	env.sandboxes.SetSkillsDir(filepath.Join(root, "skills"))

	// The real poll needs gh and the network; a script prints one item.
	url := "https://github.com/Vanclief/compose/pull/7"
	poll := env.script(t, "poll.sh", `echo '{"key":"`+url+`@abc123","url":"`+url+`","title":"Add exports"}'`)
	request.Poll = &schedulemodels.Poll{Command: poll, Secrets: request.Poll.Secrets}

	env.create(t, request)

	response := env.run(t, request.Name)
	if response.Error != "" || len(response.Created) != 1 {
		t.Fatalf("response = %+v", response)
	}

	made := response.Created[0]
	if made.Manager != "reviewer-1" || made.Owner != "franco" || made.Title != "Review Add exports" {
		t.Fatalf("task = %+v", made)
	}
	if !strings.Contains(made.Brief, url) || strings.Contains(made.Brief, "{{") {
		t.Fatalf("brief = %q", made.Brief)
	}
	if len(made.Accept) == 0 {
		t.Fatal("no accept entries")
	}
	for _, criterion := range made.Accept {
		if !strings.Contains(criterion, url) {
			t.Fatalf("accept entry without the URL: %q", criterion)
		}
	}

	if len(made.Sandboxes) != 1 {
		t.Fatalf("sandboxes = %+v", made.Sandboxes)
	}
	spec := made.Sandboxes[0].Spec
	if spec.Repo != "github.com/Vanclief/compose" || len(spec.Skills) != 1 || spec.Skills[0] != "pr-review" {
		t.Fatalf("spec = %+v", spec)
	}
}
