package agents

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/models/agent"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// An agent with no task and reads: [] still reads its inbox #<name>: GET
// /agent lists it, the home is granted it, and a person's post there
// wakes the agent when asleep (ADR 0016 item 6).
func TestAnAgentWithNoTaskReadsItsInbox(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	env.create(t, "designer")

	record, err := agent.GetAgentByName(ctx, env.db, "designer-1")
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := env.api.template(record.Role)
	if err != nil {
		t.Fatal(err)
	}

	reads, err := env.api.Reads(ctx, record, tmpl)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reads, []string{"#designer-1"}) {
		t.Fatalf("reads = %q, want the inbox only", reads)
	}
	err = msgmodels.ValidateChannel(reads[0])
	if err != nil {
		t.Fatalf("messaging rejects the inbox: %v", err)
	}

	view := env.wake(t, "designer-1")
	if view.Home == nil {
		t.Fatal("the woken agent has no home")
	}
	homeName := *view.Home

	got, err := homeClient(env, homeName).Agent(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Channels, []string{"#designer-1"}) {
		t.Fatalf("GET /agent channels = %q, want the inbox", got.Channels)
	}

	caller, err := homes{api: env.api}.Caller(ctx, homeName)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(caller.Grants, messaging.Grant{Channel: "#designer-1"}) {
		t.Fatalf("grants = %+v, want the inbox", caller.Grants)
	}

	// The agent posts a note in its own inbox through its socket.
	home := homeClient(env, homeName)
	me, err := home.Join(ctx, "designer-1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = home.Post(ctx, me.ID, "#designer-1", "Looking at the agents page.", nil)
	if err != nil {
		t.Fatalf("the agent's note in its own inbox: %v", err)
	}

	_, err = env.api.Sleep(ctx, nil, &SleepRequest{Name: "designer-1"})
	if err != nil {
		t.Fatal(err)
	}
	env.sbx.Wait()

	// Sleep posted "Asleep." in the inbox, which wakes nobody.
	asleep, err := env.msg.Read(ctx, messaging.Caller{Human: true}, "#designer-1", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(asleep) == 0 || asleep[len(asleep)-1].Body != "Asleep." {
		t.Fatalf("inbox = %+v, want Asleep. last", asleep)
	}
	env.api.Wait()
	before := len(env.rt.Calls())

	env.postAs(t, messaging.Caller{Human: true}, "franco", "#designer-1", "Can you look at the sidebar?")

	calls := env.rt.Calls()[before:]
	if !slices.Contains(calls, "start "+homeName) {
		t.Fatalf("calls after franco's post = %q, want %s started", calls, homeName)
	}
}
