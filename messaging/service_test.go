package messaging

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/vanclief/ez"
	_ "modernc.org/sqlite"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// newTestDB returns an in-memory SQLite database with the messaging
// tables created the way agc creates them at boot.
func newTestDB(t *testing.T) *bun.DB {
	t.Helper()

	sqldb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	// Every pooled connection would get its own :memory: database.
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

	return db
}

func newTestService(t *testing.T) *Service {
	t.Helper()

	return NewService(newTestDB(t))
}

var (
	human  = Caller{Human: true}
	review = Caller{Task: "review-42", Grants: []Grant{
		{Channel: "#releases"},
		{Channel: "#alerts", ReadOnly: true},
	}}
	other = Caller{Task: "build-7"}
)

func join(t *testing.T, svc *Service, caller Caller, name string) *msgmodels.Participant {
	t.Helper()

	p, err := svc.Join(context.Background(), caller, name)
	if err != nil {
		t.Fatalf("join %s: %v", name, err)
	}

	return p
}

func post(t *testing.T, svc *Service, caller Caller, p *msgmodels.Participant, channel, body string) *msgmodels.Message {
	t.Helper()

	m, err := svc.Post(context.Background(), caller, p.ID, channel, body, nil, "")
	if err != nil {
		t.Fatalf("post to %s: %v", channel, err)
	}

	return m
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()

	if err == nil {
		t.Fatalf("want %s error, got nil", code)
	}

	if ez.ErrorCode(err) != code {
		t.Fatalf("want %s error, got %s: %v", code, ez.ErrorCode(err), err)
	}
}

func TestJoinConflictAndRelease(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	first := join(t, svc, review, "reviewer")
	if len(first.ID) != 32 {
		t.Fatalf("want 32 hex chars id, got %q", first.ID)
	}

	_, err := svc.Join(ctx, other, "reviewer")
	wantCode(t, err, ez.ECONFLICT)
	if !strings.Contains(ez.ErrorMessage(err), "@reviewer") {
		t.Fatalf("conflict should name the name, got %q", ez.ErrorMessage(err))
	}

	_, err = svc.Join(ctx, review, "Bad Name")
	wantCode(t, err, ez.EINVALID)

	err = svc.Leave(ctx, other, first.ID)
	wantCode(t, err, ez.ENOTAUTHORIZED)

	err = svc.Leave(ctx, review, first.ID)
	if err != nil {
		t.Fatal(err)
	}

	second := join(t, svc, other, "reviewer")
	if second.ID == first.ID {
		t.Fatal("a later holder of a name must get a new id")
	}

	// A third holder needs the second to leave too: released names
	// are NULL and do not collide with each other.
	err = svc.Leave(ctx, human, second.ID)
	if err != nil {
		t.Fatal(err)
	}

	join(t, svc, human, "reviewer")
}

func TestAccessMatrix(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	agent := join(t, svc, review, "agent")
	franco := join(t, svc, human, "franco")

	// Own channel: read and post.
	post(t, svc, review, agent, "#review-42", "hello")
	_, err := svc.Read(ctx, review, "#review-42", 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Granted read-write channel.
	post(t, svc, review, agent, "#releases", "shipped")

	// Granted read-only channel: read yes, post no.
	post(t, svc, human, franco, "#alerts", "disk full")
	_, err = svc.Read(ctx, review, "#alerts", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Post(ctx, review, agent.ID, "#alerts", "ack", nil, "")
	wantCode(t, err, ez.ENOTAUTHORIZED)

	// Ungranted channel: no read, no wait, no post.
	_, err = svc.Read(ctx, review, "#build-7", 0, 0)
	wantCode(t, err, ez.ENOTAUTHORIZED)
	if ez.ErrorMessage(err) != "channel #build-7 is not available to task review-42" {
		t.Fatalf("unexpected message %q", ez.ErrorMessage(err))
	}
	_, err = svc.Wait(ctx, review, "#build-7", 0, time.Millisecond)
	wantCode(t, err, ez.ENOTAUTHORIZED)
	_, err = svc.Post(ctx, review, agent.ID, "#build-7", "hi", nil, "")
	wantCode(t, err, ez.ENOTAUTHORIZED)

	// Humans are unrestricted.
	post(t, svc, human, franco, "#build-7", "anyone there?")
	post(t, svc, human, franco, "#review-42", "how is it going?")
	_, err = svc.Read(ctx, human, "#releases", 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Channel names are validated before access.
	_, err = svc.Read(ctx, human, "releases", 0, 0)
	wantCode(t, err, ez.EINVALID)

	// A caller with no task and not human gets nothing.
	_, err = svc.Read(ctx, Caller{}, "#releases", 0, 0)
	wantCode(t, err, ez.ENOTAUTHORIZED)

	channels, err := svc.Channels(ctx, review)
	if err != nil {
		t.Fatal(err)
	}
	want := []ChannelInfo{
		{Channel: "#review-42", LastID: 5, Count: 2},
		{Channel: "#releases", LastID: 2, Count: 1},
		{Channel: "#alerts", ReadOnly: true, LastID: 3, Count: 1},
	}
	if len(channels) != len(want) {
		t.Fatalf("want %v, got %v", want, channels)
	}
	for i := range want {
		if channels[i] != want[i] {
			t.Fatalf("channel %d: want %+v, got %+v", i, want[i], channels[i])
		}
	}

	all, err := svc.Channels(ctx, human)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Fatalf("human should see all 4 channels, got %+v", all)
	}
}

func TestPostIdentityComesFromParticipant(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	agent := join(t, svc, review, "agent")

	m := post(t, svc, review, agent, "#review-42", "hi")
	if m.From != "agent" || m.ParticipantID != agent.ID || m.Task != "review-42" {
		t.Fatalf("sender fields not taken from participant: %+v", m)
	}

	// Another task cannot post as this participant, even in its own channel.
	_, err := svc.Post(ctx, other, agent.ID, "#build-7", "spoof", nil, "")
	wantCode(t, err, ez.ENOTAUTHORIZED)

	// Neither can a human.
	_, err = svc.Post(ctx, human, agent.ID, "#review-42", "spoof", nil, "")
	wantCode(t, err, ez.ENOTAUTHORIZED)

	_, err = svc.Post(ctx, review, "nope", "#review-42", "hi", nil, "")
	wantCode(t, err, ez.ENOTFOUND)

	_, err = svc.Post(ctx, review, agent.ID, "#review-42", "  ", nil, "")
	wantCode(t, err, ez.EINVALID)

	_, err = svc.Post(ctx, review, agent.ID, "#review-42", strings.Repeat("x", msgmodels.MaxBodyBytes+1), nil, "")
	wantCode(t, err, ez.EINVALID)

	err = svc.Leave(ctx, review, agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Post(ctx, review, agent.ID, "#review-42", "still here?", nil, "")
	wantCode(t, err, ez.EINVALID)
}

func TestReadOrderAndCursor(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	agent := join(t, svc, review, "agent")
	for _, body := range []string{"one", "two", "three"} {
		post(t, svc, review, agent, "#review-42", body)
		post(t, svc, review, agent, "#releases", "noise")
	}

	page, err := svc.Read(ctx, review, "#review-42", 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Body != "one" || page[1].Body != "two" {
		t.Fatalf("unexpected first page %+v", page)
	}

	rest, err := svc.Read(ctx, review, "#review-42", page[1].ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) != 1 || rest[0].Body != "three" {
		t.Fatalf("unexpected second page %+v", rest)
	}

	none, err := svc.Read(ctx, review, "#review-42", rest[0].ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if none == nil || len(none) != 0 {
		t.Fatalf("want empty non-nil slice, got %#v", none)
	}
}

func TestWaitWakesOnPostAndTimesOut(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	agent := join(t, svc, review, "agent")

	start := time.Now()
	empty, err := svc.Wait(ctx, review, "#review-42", 0, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("want empty slice on timeout, got %#v", empty)
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("wait returned before the timeout")
	}

	type result struct {
		messages []msgmodels.Message
		err      error
	}
	done := make(chan result, 1)
	go func() {
		messages, err := svc.Wait(ctx, review, "#review-42", 0, 10*time.Second)
		done <- result{messages, err}
	}()

	// A post elsewhere wakes the waiter but must not satisfy it.
	time.Sleep(20 * time.Millisecond)
	post(t, svc, review, agent, "#releases", "elsewhere")
	time.Sleep(20 * time.Millisecond)
	post(t, svc, review, agent, "#review-42", "here")

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatal(got.err)
		}
		if len(got.messages) != 1 || got.messages[0].Body != "here" {
			t.Fatalf("unexpected wait result %+v", got.messages)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wait did not wake on post")
	}

	// Already-present messages return immediately.
	ready, err := svc.Wait(ctx, review, "#review-42", 0, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 {
		t.Fatalf("want 1 ready message, got %+v", ready)
	}
}

func TestReplyToValidation(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	agent := join(t, svc, review, "agent")
	question := post(t, svc, review, agent, "#review-42", "question")
	elsewhere := post(t, svc, review, agent, "#releases", "elsewhere")

	answer, err := svc.Post(ctx, review, agent.ID, "#review-42", "answer", &question.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if answer.ReplyTo == nil || *answer.ReplyTo != question.ID {
		t.Fatalf("reply_to not stored: %+v", answer)
	}

	_, err = svc.Post(ctx, review, agent.ID, "#review-42", "answer", &elsewhere.ID, "")
	wantCode(t, err, ez.EINVALID)

	missing := int64(999)
	_, err = svc.Post(ctx, review, agent.ID, "#review-42", "answer", &missing, "")
	wantCode(t, err, ez.EINVALID)
}

func TestParticipantsActiveFirstThenDeparted(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	zed := join(t, svc, review, "zed")
	franco := join(t, svc, human, "franco")
	alice := join(t, svc, review, "alice")
	quiet := join(t, svc, review, "quiet")

	post(t, svc, review, zed, "#review-42", "on it")
	post(t, svc, review, zed, "#review-42", "still on it")
	post(t, svc, human, franco, "#review-42", "thanks")
	post(t, svc, review, alice, "#review-42", "done here")
	post(t, svc, review, quiet, "#releases", "elsewhere only")

	err := svc.Leave(ctx, review, alice.ID)
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.Participants(ctx, review, "#review-42")
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"franco", "zed", "alice"}
	if len(got) != len(want) {
		t.Fatalf("want %v, got %+v", want, got)
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Fatalf("participant %d: want %s, got %+v", i, name, got)
		}
	}

	if !got[0].Human || got[0].Task != "" || got[0].LeftAt != nil {
		t.Fatalf("franco should be an active human: %+v", got[0])
	}
	if got[1].Human || got[1].Task != "review-42" || got[1].LeftAt != nil {
		t.Fatalf("zed should be an active agent of review-42: %+v", got[1])
	}
	if got[2].LeftAt == nil {
		t.Fatalf("alice should have left: %+v", got[2])
	}

	empty, err := svc.Participants(ctx, human, "#nobody")
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("want empty non-nil slice, got %#v", empty)
	}

	_, err = svc.Participants(ctx, review, "#build-7")
	wantCode(t, err, ez.ENOTAUTHORIZED)

	_, err = svc.Participants(ctx, human, "")
	wantCode(t, err, ez.EINVALID)

	_, err = svc.Participants(ctx, human, "review-42")
	wantCode(t, err, ez.EINVALID)
}

func TestAllParticipantsListsEveryoneForHumansOnly(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	zed := join(t, svc, review, "zed")
	join(t, svc, human, "franco")
	quiet := join(t, svc, review, "quiet")
	post(t, svc, review, zed, "#review-42", "on it")

	err := svc.Leave(ctx, review, quiet.ID)
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.AllParticipants(ctx, human)
	if err != nil {
		t.Fatal(err)
	}

	// franco never posted and quiet left: both are still listed.
	want := []string{"franco", "zed", "quiet"}
	if len(got) != len(want) {
		t.Fatalf("want %v, got %+v", want, got)
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Fatalf("participant %d: want %s, got %+v", i, name, got)
		}
	}

	_, err = svc.AllParticipants(ctx, review)
	wantCode(t, err, ez.ENOTAUTHORIZED)
}
