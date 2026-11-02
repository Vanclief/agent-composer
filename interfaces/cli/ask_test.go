package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/vanclief/agent-composer/messaging/httpapi"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// Inside a sandbox, agc msg ask posts a question over the messaging socket
// (AGC_MSG_URL stands in for vsock here) as the joined participant, and
// agc msg post --reply-to answers one.
func TestMsgAskInsideASandboxUsesTheSocket(t *testing.T) {
	posts := []httpapi.PostRequest{}
	paths := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)

		var req httpapi.PostRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			t.Error(err)
		}
		posts = append(posts, req)

		kind := req.Kind
		if kind == "" {
			kind = msgmodels.KindMessage
		}

		httpapi.WriteJSON(w, http.StatusOK, msgmodels.Message{ID: int64(len(posts)), Channel: req.Channel, Body: req.Body, Kind: kind, ReplyTo: req.ReplyTo})
	}))
	defer server.Close()

	t.Setenv("AGC_MSG_URL", server.URL)
	t.Setenv("AGC_MSG_STATE", filepath.Join(t.TempDir(), "msg.json"))

	err := saveMsgState(msgState{ParticipantID: "p-agent", Name: "fix-login"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = runAgc(t, "msg", "ask", "fix-login", "Square", "or", "round?")
	if err != nil {
		t.Fatal(err)
	}

	_, err = runAgc(t, "msg", "post", "#fix-login", "Square.", "--reply-to", "1")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 2 || paths[0] != "POST /post" || paths[1] != "POST /post" {
		t.Fatalf("paths %q", paths)
	}

	asked := posts[0]
	if asked.ParticipantID != "p-agent" || asked.Channel != "#fix-login" || asked.Body != "Square or round?" || asked.Kind != msgmodels.KindQuestion || asked.ReplyTo != nil {
		t.Fatalf("ask sent %+v", asked)
	}

	answered := posts[1]
	if answered.Kind != "" || answered.ReplyTo == nil || *answered.ReplyTo != 1 || answered.Body != "Square." {
		t.Fatalf("post sent %+v", answered)
	}

	_, err = runAgc(t, "msg", "ask", "#fix-login")
	if err == nil {
		t.Fatal("ask without text succeeded")
	}
}
