package messaging

import (
	"context"
	"testing"
	"time"

	"github.com/vanclief/ez"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

func ask(t *testing.T, svc *Service, caller Caller, p *msgmodels.Participant, channel, body string) *msgmodels.Message {
	t.Helper()

	m, err := svc.Post(context.Background(), caller, p.ID, channel, body, nil, msgmodels.KindQuestion)
	if err != nil {
		t.Fatalf("ask in %s: %v", channel, err)
	}

	return m
}

func reply(t *testing.T, svc *Service, caller Caller, p *msgmodels.Participant, to *msgmodels.Message, body string) *msgmodels.Message {
	t.Helper()

	m, err := svc.Post(context.Background(), caller, p.ID, to.Channel, body, &to.ID, "")
	if err != nil {
		t.Fatalf("reply to %d: %v", to.ID, err)
	}

	return m
}

func questions(t *testing.T, svc *Service, caller Caller, filter QuestionsFilter) []Question {
	t.Helper()

	got, err := svc.Questions(context.Background(), caller, filter)
	if err != nil {
		t.Fatalf("questions %+v: %v", filter, err)
	}

	return got
}

func TestPostKind(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	agent := join(t, svc, review, "agent")

	m := post(t, svc, review, agent, "#review-42", "plain")
	if m.Kind != msgmodels.KindMessage {
		t.Fatalf("default kind = %q", m.Kind)
	}

	q := ask(t, svc, review, agent, "#review-42", "which icon?")
	if q.Kind != msgmodels.KindQuestion {
		t.Fatalf("kind = %q", q.Kind)
	}

	_, err := svc.Post(ctx, review, agent.ID, "#review-42", "hm", nil, "poll")
	wantCode(t, err, ez.EINVALID)

	read, err := svc.Read(ctx, review, "#review-42", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(read) != 2 || read[0].Kind != msgmodels.KindMessage || read[1].Kind != msgmodels.KindQuestion {
		t.Fatalf("read = %+v", read)
	}
}

func TestReplyAnswersQuestion(t *testing.T) {
	svc := newTestService(t)

	agent := join(t, svc, review, "agent")
	franco := join(t, svc, human, "franco")
	q := ask(t, svc, review, agent, "#review-42", "which icon?")

	got := questions(t, svc, human, QuestionsFilter{})
	if len(got) != 1 || got[0].ID != q.ID || got[0].AnsweredBy != nil || got[0].AnsweredAt != nil || got[0].AnswerID != nil {
		t.Fatalf("unanswered = %+v", got)
	}

	// The asker adding to their own question does not answer it.
	reply(t, svc, review, agent, q, "the square or the round one")
	if len(questions(t, svc, human, QuestionsFilter{Open: true})) != 1 {
		t.Fatal("a reply by the asker answered the question")
	}

	answer := reply(t, svc, human, franco, q, "the square one")
	reply(t, svc, human, franco, q, "and make it red")

	got = questions(t, svc, human, QuestionsFilter{})
	if len(got) != 1 || got[0].AnsweredBy == nil || *got[0].AnsweredBy != "franco" || got[0].AnswerID == nil || *got[0].AnswerID != answer.ID {
		t.Fatalf("answered = %+v", got)
	}
	if got[0].AnsweredAt == nil || got[0].AnsweredAt.Sub(answer.CreatedAt).Abs() > time.Millisecond {
		t.Fatalf("answered_at = %v, want %v", got[0].AnsweredAt, answer.CreatedAt)
	}

	if len(questions(t, svc, human, QuestionsFilter{Open: true})) != 0 {
		t.Fatal("open lists an answered question")
	}
}

func TestReplyByAskerUnderNewParticipantDoesNotAnswer(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	agent := join(t, svc, review, "agent")
	q := ask(t, svc, review, agent, "#review-42", "which icon?")

	err := svc.Leave(ctx, review, agent.ID)
	if err != nil {
		t.Fatal(err)
	}

	again := join(t, svc, review, "agent")
	reply(t, svc, review, again, q, "still wondering")

	if len(questions(t, svc, human, QuestionsFilter{Open: true})) != 1 {
		t.Fatal("the asker's own reply answered the question")
	}
}

func TestQuestionsFilters(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	svc.SetChannelsFor(func(ctx context.Context, name string) ([]string, error) {
		if name == "franco" {
			return []string{"#review-42"}, nil
		}

		return nil, nil
	})

	agent := join(t, svc, review, "agent")
	builder := join(t, svc, other, "builder")
	franco := join(t, svc, human, "franco")

	first := ask(t, svc, review, agent, "#review-42", "first?")
	second := ask(t, svc, review, agent, "#review-42", "second?")
	inbox := ask(t, svc, review, agent, "#releases", "release?")
	build := ask(t, svc, other, builder, "#build-7", "build?")
	direct := ask(t, svc, human, franco, "#franco", "note to self?")
	post(t, svc, review, agent, "#review-42", "not a question")
	reply(t, svc, human, franco, first, "yes")

	ids := func(got []Question) []int64 {
		out := []int64{}
		for _, q := range got {
			out = append(out, q.ID)
		}
		return out
	}
	want := func(name string, got []Question, expected ...int64) {
		t.Helper()
		gotIDs := ids(got)
		if len(gotIDs) != len(expected) {
			t.Fatalf("%s = %v, want %v", name, gotIDs, expected)
		}
		for i := range expected {
			if gotIDs[i] != expected[i] {
				t.Fatalf("%s = %v, want %v", name, gotIDs, expected)
			}
		}
	}

	want("all", questions(t, svc, human, QuestionsFilter{}), direct.ID, build.ID, inbox.ID, second.ID, first.ID)
	want("open", questions(t, svc, human, QuestionsFilter{Open: true}), direct.ID, build.ID, inbox.ID, second.ID)
	want("channel", questions(t, svc, human, QuestionsFilter{Channel: "#review-42"}), second.ID, first.ID)
	want("for franco", questions(t, svc, human, QuestionsFilter{For: "franco"}), direct.ID, second.ID, first.ID)
	want("open for franco", questions(t, svc, human, QuestionsFilter{For: "franco", Open: true}), direct.ID, second.ID)
	want("for nobody", questions(t, svc, human, QuestionsFilter{For: "nobody"}))
	want("task sees its channels", questions(t, svc, review, QuestionsFilter{}), inbox.ID, second.ID, first.ID)

	_, err := svc.Questions(ctx, review, QuestionsFilter{Channel: "#build-7"})
	wantCode(t, err, ez.ENOTAUTHORIZED)

	_, err = svc.Questions(ctx, human, QuestionsFilter{For: "Not A Name"})
	wantCode(t, err, ez.EINVALID)
}
