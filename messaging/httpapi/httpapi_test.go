package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	_ "modernc.org/sqlite"

	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	task := func(*http.Request) (messaging.Caller, error) {
		return messaging.Caller{Task: "review-42"}, nil
	}

	server, _ := newServerAs(t, task)

	return server
}

// newServerAs serves a fresh service with caller resolving every request.
func newServerAs(t *testing.T, caller func(*http.Request) (messaging.Caller, error)) (*httptest.Server, *messaging.Service) {
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
	tables := []interface{}{
		(*msgmodels.Participant)(nil),
		(*msgmodels.Message)(nil),
	}
	for _, table := range tables {
		_, err = db.NewCreateTable().Model(table).IfNotExists().Exec(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	}

	svc := messaging.NewService(db)
	server := httptest.NewServer(New(svc, caller))
	t.Cleanup(server.Close)

	return server, svc
}

func call(t *testing.T, method, url, body string, out any) int {
	t.Helper()

	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(out)
	if err != nil {
		t.Fatal(err)
	}

	return res.StatusCode
}

func TestJoinPostRead(t *testing.T) {
	server := newTestServer(t)

	var p msgmodels.Participant
	status := call(t, http.MethodPost, server.URL+"/join", `{"name":"reviewer"}`, &p)
	if status != http.StatusOK || p.ID == "" || p.Task != "review-42" {
		t.Fatalf("join: status %d, participant %+v", status, p)
	}

	// The body tries to claim another sender, which the server ignores.
	var m msgmodels.Message
	body := `{"participant_id":"` + p.ID + `","channel":"#review-42","body":"hi","from":"franco","task":"other"}`
	status = call(t, http.MethodPost, server.URL+"/post", body, &m)
	if status != http.StatusOK || m.From != "reviewer" || m.Task != "review-42" {
		t.Fatalf("post: status %d, message %+v", status, m)
	}

	var read MessagesResponse
	status = call(t, http.MethodGet, server.URL+"/read?channel=%23review-42", "", &read)
	if status != http.StatusOK || len(read.Messages) != 1 || read.Cursor != m.ID {
		t.Fatalf("read: status %d, response %+v", status, read)
	}

	var waited MessagesResponse
	status = call(t, http.MethodGet, server.URL+"/wait?channel=%23review-42&after=1&timeout=10ms", "", &waited)
	if status != http.StatusOK || len(waited.Messages) != 0 || waited.Cursor != 1 {
		t.Fatalf("wait: status %d, response %+v", status, waited)
	}

	var channels ChannelsResponse
	status = call(t, http.MethodGet, server.URL+"/channels", "", &channels)
	if status != http.StatusOK || len(channels.Channels) != 1 || channels.Channels[0].Count != 1 {
		t.Fatalf("channels: status %d, response %+v", status, channels)
	}

	var participants ParticipantsResponse
	status = call(t, http.MethodGet, server.URL+"/participants?channel=%23review-42", "", &participants)
	if status != http.StatusOK || len(participants.Participants) != 1 || participants.Participants[0].ID != p.ID {
		t.Fatalf("participants: status %d, response %+v", status, participants)
	}
}

func TestErrors(t *testing.T) {
	server := newTestServer(t)

	var failure ErrorResponse
	status := call(t, http.MethodGet, server.URL+"/read?channel=%23build-7", "", &failure)
	if status != http.StatusForbidden || failure.Error != "channel #build-7 is not available to task review-42" {
		t.Fatalf("want 403 naming the channel, got %d %+v", status, failure)
	}

	status = call(t, http.MethodGet, server.URL+"/read?channel=%23review-42&after=x", "", &failure)
	if status != http.StatusBadRequest {
		t.Fatalf("want 400 for a bad cursor, got %d %+v", status, failure)
	}

	failure = ErrorResponse{}
	status = call(t, http.MethodGet, server.URL+"/participants", "", &failure)
	if status != http.StatusBadRequest || !strings.Contains(failure.Error, "is invalid") {
		t.Fatalf("want 400 for a missing channel, got %d %+v", status, failure)
	}

	failure = ErrorResponse{}
	status = call(t, http.MethodGet, server.URL+"/participants?channel=review-42", "", &failure)
	if status != http.StatusBadRequest || !strings.Contains(failure.Error, "is invalid") {
		t.Fatalf("want 400 for a malformed channel, got %d %+v", status, failure)
	}

	status = call(t, http.MethodPost, server.URL+"/leave", `{"participant_id":"missing"}`, &failure)
	if status != http.StatusNotFound {
		t.Fatalf("want 404 for an unknown participant, got %d %+v", status, failure)
	}

	var p msgmodels.Participant
	call(t, http.MethodPost, server.URL+"/join", `{"name":"reviewer"}`, &p)
	status = call(t, http.MethodPost, server.URL+"/join", `{"name":"reviewer"}`, &failure)
	if status != http.StatusConflict {
		t.Fatalf("want 409 for a taken name, got %d %+v", status, failure)
	}
}

func TestReadMarksHumanSeen(t *testing.T) {
	human := func(*http.Request) (messaging.Caller, error) {
		return messaging.Caller{Human: true}, nil
	}
	server, _ := newServerAs(t, human)

	var p msgmodels.Participant
	call(t, http.MethodPost, server.URL+"/join", `{"name":"franco"}`, &p)

	var view messaging.ParticipantView
	status := call(t, http.MethodGet, server.URL+"/participants/"+p.ID, "", &view)
	if status != http.StatusOK || view.Online || view.LastSeenAt != nil {
		t.Fatalf("before reading: status %d, view %+v", status, view)
	}

	// A read without participant marks no one.
	var read MessagesResponse
	call(t, http.MethodGet, server.URL+"/read?channel=%23review-42", "", &read)
	view = messaging.ParticipantView{}
	call(t, http.MethodGet, server.URL+"/participants/"+p.ID, "", &view)
	if view.Online {
		t.Fatalf("a read naming no participant should not mark one: %+v", view)
	}

	status = call(t, http.MethodGet, server.URL+"/read?channel=%23review-42&participant="+p.ID, "", &read)
	if status != http.StatusOK {
		t.Fatalf("read: status %d", status)
	}

	view = messaging.ParticipantView{}
	status = call(t, http.MethodGet, server.URL+"/participants/"+p.ID, "", &view)
	if status != http.StatusOK || !view.Online || view.LastSeenAt == nil {
		t.Fatalf("after reading: status %d, view %+v", status, view)
	}

	var failure ErrorResponse
	status = call(t, http.MethodGet, server.URL+"/participants/missing", "", &failure)
	if status != http.StatusNotFound {
		t.Fatalf("want 404 for an unknown participant, got %d %+v", status, failure)
	}
}

func TestWaitOnSeveralChannels(t *testing.T) {
	grants := func(*http.Request) (messaging.Caller, error) {
		return messaging.Caller{Task: "review-42", Grants: []messaging.Grant{{Channel: "#releases"}}}, nil
	}
	server, _ := newServerAs(t, grants)

	var p msgmodels.Participant
	call(t, http.MethodPost, server.URL+"/join", `{"name":"reviewer"}`, &p)

	for _, channel := range []string{"#releases", "#review-42", "#releases"} {
		var m msgmodels.Message
		status := call(t, http.MethodPost, server.URL+"/post", `{"participant_id":"`+p.ID+`","channel":"`+channel+`","body":"x"}`, &m)
		if status != http.StatusOK {
			t.Fatalf("post in %s: status %d", channel, status)
		}
	}

	var waited MessagesResponse
	status := call(t, http.MethodGet, server.URL+"/wait?channels=%23review-42,%23releases&after=0&timeout=1s", "", &waited)
	if status != http.StatusOK || len(waited.Messages) != 3 || waited.Cursor != waited.Messages[2].ID {
		t.Fatalf("wait: status %d, response %+v", status, waited)
	}
	for i, want := range []string{"#releases", "#review-42", "#releases"} {
		if waited.Messages[i].Channel != want || (i > 0 && waited.Messages[i].ID <= waited.Messages[i-1].ID) {
			t.Fatalf("message %d: %+v", i, waited.Messages[i])
		}
	}

	var read MessagesResponse
	status = call(t, http.MethodGet, server.URL+"/read?channel=%23review-42&channels=%23releases&after=1", "", &read)
	if status != http.StatusOK || len(read.Messages) != 2 {
		t.Fatalf("read: status %d, response %+v", status, read)
	}

	var failure ErrorResponse
	status = call(t, http.MethodGet, server.URL+"/wait?channels=%23review-42,%23secret&timeout=10ms", "", &failure)
	if status != http.StatusForbidden || !strings.Contains(failure.Error, "#secret") {
		t.Fatalf("refused channel: status %d, error %q", status, failure.Error)
	}
}

func TestParticipantsAll(t *testing.T) {
	humanCaller := func(*http.Request) (messaging.Caller, error) {
		return messaging.Caller{Human: true}, nil
	}
	server, _ := newServerAs(t, humanCaller)

	var p msgmodels.Participant
	status := call(t, http.MethodPost, server.URL+"/join", `{"name":"franco"}`, &p)
	if status != http.StatusOK {
		t.Fatalf("join: status %d, participant %+v", status, p)
	}

	// franco never posted, so no channel lists them, but ?all=1 does.
	var participants ParticipantsResponse
	status = call(t, http.MethodGet, server.URL+"/participants?all=1", "", &participants)
	if status != http.StatusOK || len(participants.Participants) != 1 || participants.Participants[0].Name != "franco" {
		t.Fatalf("participants?all=1: status %d, response %+v", status, participants)
	}

	task := newTestServer(t)
	var failure ErrorResponse
	status = call(t, http.MethodGet, task.URL+"/participants?all=1", "", &failure)
	if status != http.StatusForbidden {
		t.Fatalf("want 403 for a task listing everyone, got %d %+v", status, failure)
	}
}
