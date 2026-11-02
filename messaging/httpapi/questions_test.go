package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

func TestPostKind(t *testing.T) {
	server := newTestServer(t)

	var p msgmodels.Participant
	call(t, http.MethodPost, server.URL+"/join", `{"name":"agent"}`, &p)

	var m msgmodels.Message
	status := call(t, http.MethodPost, server.URL+"/post", fmt.Sprintf(`{"participant_id":%q,"channel":"#review-42","body":"which icon?","kind":"question"}`, p.ID), &m)
	if status != http.StatusOK || m.Kind != msgmodels.KindQuestion {
		t.Fatalf("status %d, message %+v", status, m)
	}

	status = call(t, http.MethodPost, server.URL+"/post", fmt.Sprintf(`{"participant_id":%q,"channel":"#review-42","body":"hi"}`, p.ID), &m)
	if status != http.StatusOK || m.Kind != msgmodels.KindMessage {
		t.Fatalf("status %d, message %+v", status, m)
	}

	var failure ErrorResponse
	status = call(t, http.MethodPost, server.URL+"/post", fmt.Sprintf(`{"participant_id":%q,"channel":"#review-42","body":"hi","kind":"poll"}`, p.ID), &failure)
	if status != http.StatusBadRequest || failure.Error == "" {
		t.Fatalf("status %d, error %+v", status, failure)
	}

	var read MessagesResponse
	call(t, http.MethodGet, server.URL+"/read?channel=%23review-42", "", &read)
	if len(read.Messages) != 2 || read.Messages[0].Kind != msgmodels.KindQuestion || read.Messages[1].Kind != msgmodels.KindMessage {
		t.Fatalf("read = %+v", read)
	}
}

func TestQuestionsRoute(t *testing.T) {
	humanCaller := func(*http.Request) (messaging.Caller, error) {
		return messaging.Caller{Human: true}, nil
	}

	server, svc := newServerAs(t, humanCaller)
	svc.SetChannelsFor(func(ctx context.Context, name string) ([]string, error) {
		if name == "franco" {
			return []string{"#stop-icon"}, nil
		}

		return nil, nil
	})

	join := func(name string) msgmodels.Participant {
		var p msgmodels.Participant
		status := call(t, http.MethodPost, server.URL+"/join", fmt.Sprintf(`{"name":%q}`, name), &p)
		if status != http.StatusOK {
			t.Fatalf("join %s: %d", name, status)
		}
		return p
	}
	post := func(p msgmodels.Participant, channel, body, kind string, replyTo int64) msgmodels.Message {
		var m msgmodels.Message
		reply := ""
		if replyTo != 0 {
			reply = fmt.Sprintf(`,"reply_to":%d`, replyTo)
		}
		status := call(t, http.MethodPost, server.URL+"/post", fmt.Sprintf(`{"participant_id":%q,"channel":%q,"body":%q,"kind":%q%s}`, p.ID, channel, body, kind, reply), &m)
		if status != http.StatusOK {
			t.Fatalf("post %q: %d", body, status)
		}
		return m
	}
	list := func(query string) []int64 {
		var res QuestionsResponse
		status := call(t, http.MethodGet, server.URL+"/questions?"+query, "", &res)
		if status != http.StatusOK {
			t.Fatalf("questions?%s: %d", query, status)
		}
		ids := []int64{}
		for _, q := range res.Questions {
			ids = append(ids, q.ID)
		}
		return ids
	}
	want := func(query string, expected ...int64) {
		t.Helper()
		got := list(query)
		if fmt.Sprint(got) != fmt.Sprint(expected) {
			t.Fatalf("questions?%s = %v, want %v", query, got, expected)
		}
	}

	agent := join("agent")
	franco := join("franco")

	answered := post(agent, "#stop-icon", "square?", msgmodels.KindQuestion, 0)
	open := post(agent, "#stop-icon", "red?", msgmodels.KindQuestion, 0)
	inbox := post(agent, "#franco", "budget?", msgmodels.KindQuestion, 0)
	elsewhere := post(agent, "#other-task", "who?", msgmodels.KindQuestion, 0)
	post(agent, "#stop-icon", "just saying", msgmodels.KindMessage, 0)
	answer := post(franco, "#stop-icon", "yes", "", answered.ID)

	want("", elsewhere.ID, inbox.ID, open.ID, answered.ID)
	want("open=1", elsewhere.ID, inbox.ID, open.ID)
	want("for=franco", inbox.ID, open.ID, answered.ID)
	want("open=1&for=franco", inbox.ID, open.ID)
	want("channel="+url.QueryEscape("#stop-icon")+"&open=1", open.ID)

	var res QuestionsResponse
	call(t, http.MethodGet, server.URL+"/questions?channel=%23stop-icon", "", &res)
	last := res.Questions[len(res.Questions)-1]
	if last.AnsweredBy == nil || *last.AnsweredBy != "franco" || last.AnswerID == nil || *last.AnswerID != answer.ID || last.AnsweredAt == nil {
		t.Fatalf("answered question = %+v", last)
	}
	if res.Questions[0].AnsweredBy != nil || res.Questions[0].AnsweredAt != nil {
		t.Fatalf("open question = %+v", res.Questions[0])
	}

	var failure ErrorResponse
	status := call(t, http.MethodGet, server.URL+"/questions?open=maybe", "", &failure)
	if status != http.StatusBadRequest {
		t.Fatalf("open=maybe: %d", status)
	}
	status = call(t, http.MethodGet, server.URL+"/questions?channel=nohash", "", &failure)
	if status != http.StatusBadRequest {
		t.Fatalf("channel=nohash: %d", status)
	}
}

func TestQuestionsRouteForTask(t *testing.T) {
	server := newTestServer(t)

	var p msgmodels.Participant
	call(t, http.MethodPost, server.URL+"/join", `{"name":"agent"}`, &p)

	var m msgmodels.Message
	call(t, http.MethodPost, server.URL+"/post", fmt.Sprintf(`{"participant_id":%q,"channel":"#review-42","body":"which?","kind":"question"}`, p.ID), &m)

	var res QuestionsResponse
	status := call(t, http.MethodGet, server.URL+"/questions", "", &res)
	if status != http.StatusOK || len(res.Questions) != 1 || res.Questions[0].ID != m.ID {
		t.Fatalf("status %d, questions %+v", status, res)
	}

	var failure ErrorResponse
	status = call(t, http.MethodGet, server.URL+"/questions?channel=%23other", "", &failure)
	if status != http.StatusUnauthorized && status != http.StatusForbidden {
		t.Fatalf("other channel: %d", status)
	}
}
