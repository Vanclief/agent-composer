package cli

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/vanclief/ez"

	msgclient "github.com/vanclief/agent-composer/messaging/client"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// fakeImages serves the images the fakes hold, keyed <message id>/<name>.
var fakeImages = map[string][]byte{"1/shot.png": []byte("png bytes"), "4/diff.png": []byte("diff bytes")}

func fakeImage(messageID int64, name string) ([]byte, error) {
	data, ok := fakeImages[strconv.FormatInt(messageID, 10)+"/"+name]
	if !ok {
		return nil, ez.New(ez.ENOTFOUND, "message "+strconv.FormatInt(messageID, 10)+" has no image "+name, nil)
	}

	return data, nil
}

func (m *fakeHomeMessenger) Image(ctx context.Context, messageID int64, name string) ([]byte, error) {
	return fakeImage(messageID, name)
}

func (m *fakeMessenger) Image(ctx context.Context, messageID int64, name string) ([]byte, error) {
	return fakeImage(messageID, name)
}

// withImages adds images to m, with the URLs agc gives them.
func withImages(m msgmodels.Message, names ...string) msgmodels.Message {
	for _, name := range names {
		m.Images = append(m.Images, msgmodels.Image{Name: name, Type: "image/png", Size: 9})
	}
	m.FillImageURLs()

	return m
}

// ADR 0005 "Images": the home loop downloads a turn's images into the
// sandbox and lists each path under its message's line.
func TestHomeLoopTurnListsImagePaths(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := &fakeHomeMessenger{
		cancel: cancel,
		agent:  msgclient.AgentResponse{Agent: msgclient.AgentView{Name: "reviewer-1", Role: "reviewer"}, Channels: []string{"#dm"}, Idle: "15m"},
		script: [][]msgmodels.Message{{withImages(homeMsg(1, "#dm", "franco", 0, "look at this"), "shot.png", "gone.png")}},
	}
	harness := &homeHarness{}
	loop, _ := newHomeLoop(t, msg, harness)
	loop.images = t.TempDir()

	err := loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(loop.images, "1", "shot.png")
	want := "@franco in #dm [message] (1): look at this\n" +
		"  image: " + path + "\n" +
		"  image: /api/msg/images/1/gone.png (not fetched: message 1 has no image gone.png)\n"
	if len(harness.turns) != 1 || !strings.Contains(harness.turns[0].prompt, want) {
		t.Fatalf("turns = %#v", harness.turns)
	}

	data, err := os.ReadFile(path)
	if err != nil || string(data) != "png bytes" {
		t.Fatalf("image file = %q, %v", data, err)
	}
}

// The task loop does the same for a message in #<task>; a text-only
// message's prompt is as before.
func TestAgentLoopTurnListsImagePaths(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := &fakeMessenger{
		cancel:  cancel,
		nextID:  2,
		lastIDs: map[string]int64{"#fix-login": 2},
		script: [][]msgmodels.Message{
			{withImages(message(4, "orchestrator", "p-orch", "Match this."), "diff.png"), message(5, "orchestrator", "p-orch", "Thanks.")},
		},
	}
	harness := &fakeHarness{}
	dir := t.TempDir()

	loop := &agentLoop{harness: harness, msg: msg, task: "fix-login", name: "fix-login", brief: "# Fix login\n", images: dir}

	err := loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "4", "diff.png")
	want := []string{
		"|# Fix login\n",
		"s1|@orchestrator in #fix-login:\n\nMatch this.\n\nImages (read each file to see it):\n  image: " + path + "\n",
		"s1|@orchestrator in #fix-login:\n\nThanks.",
	}
	if strings.Join(harness.turns, "\n---\n") != strings.Join(want, "\n---\n") {
		t.Fatalf("turns:\n%s", strings.Join(harness.turns, "\n---\n"))
	}

	data, err := os.ReadFile(path)
	if err != nil || string(data) != "diff bytes" {
		t.Fatalf("image file = %q, %v", data, err)
	}
}
