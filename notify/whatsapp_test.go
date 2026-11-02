package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vanclief/ez"
)

func TestWhatsAppRequestShape(t *testing.T) {
	var gotPath, gotAuth, gotType string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.Method + " " + r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotType = r.Header.Get("Content-Type")

		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}

		err = json.Unmarshal(raw, &gotBody)
		if err != nil {
			t.Error(err)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.1"}]}`))
	}))
	defer server.Close()

	sender := &WhatsApp{
		BaseURL:       server.URL + "/",
		Version:       "v21.0",
		PhoneNumberID: "12345",
		Numbers:       map[string]string{"franco": "+34600111222"},
		Token:         func() (string, error) { return "tok", nil },
	}

	err := sender.Notify(context.Background(), Notice{Human: "franco", Sender: "agent", Task: "yc-pitch", Text: "ready?"})
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "POST /v21.0/12345/messages" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotType != "application/json" {
		t.Fatalf("content type = %q", gotType)
	}

	text, ok := gotBody["text"].(map[string]any)
	if !ok {
		t.Fatalf("no text object in %v", gotBody)
	}
	if gotBody["messaging_product"] != "whatsapp" || gotBody["type"] != "text" || gotBody["to"] != "34600111222" {
		t.Fatalf("body = %v", gotBody)
	}
	if text["body"] != "agent in yc-pitch:\nready?" {
		t.Fatalf("text = %v", text)
	}
}

func TestWhatsAppErrors(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid OAuth access token"}}`))
	}))
	defer server.Close()

	sender := &WhatsApp{
		BaseURL:       server.URL,
		PhoneNumberID: "1",
		Numbers:       map[string]string{"franco": "34600111222"},
		Token:         func() (string, error) { return "bad", nil },
	}

	err := sender.Notify(context.Background(), Notice{Human: "franco", Sender: "a", Task: "t", Text: "x"})
	if err == nil || !strings.Contains(ez.ErrorMessage(err), "401") {
		t.Fatalf("want a 401 error, got %v", err)
	}

	// A human without a number is skipped, not an error, and no call is made.
	before := calls
	err = sender.Notify(context.Background(), Notice{Human: "ana", Sender: "a", Task: "t", Text: "x"})
	if err != nil || calls != before {
		t.Fatalf("unknown human: err=%v calls=%d", err, calls-before)
	}

	// A missing token is an error and nothing is sent.
	sender.Token = func() (string, error) { return "", ez.New(ez.ENOTFOUND, "no WHATSAPP_TOKEN", nil) }
	err = sender.Notify(context.Background(), Notice{Human: "franco", Sender: "a", Task: "t", Text: "x"})
	if err == nil || calls != before {
		t.Fatalf("missing token: err=%v calls=%d", err, calls-before)
	}
}
