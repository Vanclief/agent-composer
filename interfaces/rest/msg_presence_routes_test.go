package rest

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/vanclief/agent-composer/messaging/httpapi"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// Participants carry computed presence: agc is never online, a human is
// once it reads or posts.
func TestMsgParticipantPresenceRoutes(t *testing.T) {
	routes := newTaskRoutes(t, &fakeRuntime{sb: &fakeSandbox{}}, "")

	status, body := routes.do(http.MethodPost, "/api/tasks", `{"title":"Fix login","brief":"b","manager":"orchestrator"}`)
	if status != http.StatusOK {
		t.Fatalf("create: status %d, body %s", status, body)
	}
	routes.server.SandboxesAPI.Wait()

	status, body = routes.do(http.MethodPost, "/api/msg/join", `{"name":"franco"}`)
	if status != http.StatusOK {
		t.Fatalf("join: status %d, body %s", status, body)
	}

	var franco msgmodels.Participant
	err := json.Unmarshal([]byte(body), &franco)
	if err != nil {
		t.Fatal(err)
	}

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/msg/participants/" + franco.ID, "", http.StatusOK, `"online":false,"last_seen_at":null`},
		{http.MethodGet, "/api/msg/read?channel=%23fix-login&participant=" + franco.ID, "", http.StatusOK, `"messages"`},
		{http.MethodGet, "/api/msg/participants/" + franco.ID, "", http.StatusOK, `"online":true,"last_seen_at":"20`},
		{http.MethodPost, "/api/msg/post", `{"participant_id":"` + franco.ID + `","channel":"#fix-login","body":"hi"}`, http.StatusOK, `"human":true`},
		{http.MethodGet, "/api/msg/participants/nope", "", http.StatusNotFound, "participant nope not found"},
	})

	status, body = routes.do(http.MethodGet, "/api/msg/participants?channel=%23fix-login", "")
	if status != http.StatusOK {
		t.Fatalf("participants: status %d, body %s", status, body)
	}

	var list httpapi.ParticipantsResponse
	err = json.Unmarshal([]byte(body), &list)
	if err != nil {
		t.Fatal(err)
	}

	online := map[string]bool{}
	for _, p := range list.Participants {
		online[p.Name] = p.Online
		if p.LastSeenAt == nil {
			t.Fatalf("%s posted, so it has a last_seen_at: %s", p.Name, body)
		}
	}

	if len(online) != 2 || online["agc"] || !online["franco"] {
		t.Fatalf("participants = %s", body)
	}
}
