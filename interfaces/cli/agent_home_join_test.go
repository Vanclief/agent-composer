package cli

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/vanclief/ez"
	_ "modernc.org/sqlite"

	"github.com/vanclief/agent-composer/messaging"
	msgclient "github.com/vanclief/agent-composer/messaging/client"
	"github.com/vanclief/agent-composer/messaging/httpapi"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// homePresence says which sandboxes run, for messaging's presence.
type homePresence map[string]bool

func (p homePresence) SandboxRunning(_ context.Context, name string) (bool, bool) {
	running, found := p[name]
	return running, found
}

func (p homePresence) TaskRunning(_ context.Context, task string) bool {
	return false
}

// joiningMessenger is the fake home socket with Join and Leave served by
// the real messaging service, as the home's socket serves them.
type joiningMessenger struct {
	*fakeHomeMessenger
	client *msgclient.Client
}

func (m *joiningMessenger) Join(ctx context.Context, name string) (*msgmodels.Participant, error) {
	return m.client.Join(ctx, name)
}

func (m *joiningMessenger) Leave(ctx context.Context, participantID string) (*httpapi.LeaveResponse, error) {
	return m.client.Leave(ctx, participantID)
}

// newHomeMessaging is a messaging service on an empty database, with
// presence, and the socket of the home sandbox called home.
func newHomeMessaging(t *testing.T, presence homePresence, home string) (*messaging.Service, *msgclient.Client) {
	t.Helper()

	sqldb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	sqldb.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqldb.Close()
	})

	db := bun.NewDB(sqldb, sqlitedialect.New())
	for _, table := range []interface{}{(*msgmodels.Participant)(nil), (*msgmodels.Message)(nil), (*msgmodels.ChannelArchive)(nil)} {
		_, err = db.NewCreateTable().Model(table).Exec(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	}

	svc := messaging.NewService(db)
	svc.SetPresence(presence)

	caller := func(*http.Request) (messaging.Caller, error) {
		return messaging.Caller{Agent: home}, nil
	}
	server := httptest.NewServer(httpapi.New(svc, caller))
	t.Cleanup(server.Close)

	return svc, &msgclient.Client{BaseURL: server.URL + "/"}
}

// runHomeJoin runs the home loop of reviewer-1 in its home reviewer-1-2
// until it sleeps, and returns whom it joined as.
func runHomeJoin(t *testing.T, client *msgclient.Client) (*msgmodels.Participant, error) {
	t.Helper()

	fake := &fakeHomeMessenger{
		idle:  true,
		agent: msgclient.AgentResponse{Agent: msgclient.AgentView{Name: "reviewer-1", Role: "reviewer"}, Channels: []string{"#dm"}, Idle: "1m"},
	}
	loop, _ := newHomeLoop(t, fake, &homeHarness{})
	loop.msg = &joiningMessenger{fakeHomeMessenger: fake, client: client}

	var joined *msgmodels.Participant
	loop.joined = func(p *msgmodels.Participant) error {
		joined = p
		return nil
	}

	err := loop.run(context.Background())

	return joined, err
}

func TestHomeLoopJoinsAsTheAgentsName(t *testing.T) {
	_, client := newHomeMessaging(t, homePresence{"reviewer-1-2": true}, "reviewer-1-2")

	p, err := runHomeJoin(t, client)
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.Name != "reviewer-1" || p.Agent != "reviewer-1-2" {
		t.Fatalf("joined = %+v", p)
	}
}

// The first home reviewer-1 was deleted and left its participant
// reviewer-1 joined; presence says it is absent, so the new home takes
// the name over instead of joining as reviewer-1-2.
func TestHomeLoopTakesOverAStaleParticipantOfTheAgentsName(t *testing.T) {
	svc, client := newHomeMessaging(t, homePresence{"reviewer-1-2": true}, "reviewer-1-2")

	stale, err := svc.Join(context.Background(), messaging.Caller{Agent: "reviewer-1"}, "reviewer-1")
	if err != nil {
		t.Fatal(err)
	}

	p, err := runHomeJoin(t, client)
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.Name != "reviewer-1" || p.Agent != "reviewer-1-2" || p.ID == stale.ID {
		t.Fatalf("joined = %+v, stale %s", p, stale.ID)
	}

	views, err := svc.AllParticipants(context.Background(), messaging.Caller{Human: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range views {
		if view.ID == stale.ID && view.LeftAt == nil {
			t.Fatalf("stale participant still joined: %+v", view)
		}
	}
}

// Messaging refuses a name a running sandbox's participant holds (409),
// and the home loop fails rather than joining as reviewer-1-2.
func TestHomeLoopFailsWhenALiveParticipantHoldsTheAgentsName(t *testing.T) {
	svc, client := newHomeMessaging(t, homePresence{"reviewer-1": true, "reviewer-1-2": true}, "reviewer-1-2")

	_, err := svc.Join(context.Background(), messaging.Caller{Agent: "reviewer-1"}, "reviewer-1")
	if err != nil {
		t.Fatal(err)
	}

	p, err := runHomeJoin(t, client)
	if ez.ErrorCode(err) != ez.ECONFLICT || p != nil {
		t.Fatalf("joined = %+v, err = %v", p, err)
	}
}

// A human's name is never taken over, even with no sandbox behind it.
func TestHomeLoopFailsWhenAHumanHoldsTheAgentsName(t *testing.T) {
	svc, client := newHomeMessaging(t, homePresence{"reviewer-1-2": true}, "reviewer-1-2")

	_, err := svc.Join(context.Background(), messaging.Caller{Human: true}, "reviewer-1")
	if err != nil {
		t.Fatal(err)
	}

	p, err := runHomeJoin(t, client)
	if ez.ErrorCode(err) != ez.ECONFLICT || p != nil {
		t.Fatalf("joined = %+v, err = %v", p, err)
	}
}
