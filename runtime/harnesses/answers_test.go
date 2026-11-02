package harnesses

import "testing"

func TestClaudeCodeAnswer(t *testing.T) {
	answer, session, err := ClaudeCodeAnswer(`{"type":"result","subtype":"success","result":"Done.","session_id":"s1"}`)
	if err != nil || answer != "Done." || session != "s1" {
		t.Fatalf("answer %q, session %q, err %v", answer, session, err)
	}

	_, session, err = ClaudeCodeAnswer(`{"type":"result","is_error":true,"result":"Not logged in","session_id":"s2"}`)
	if err == nil || session != "s2" {
		t.Fatalf("session %q, err %v", session, err)
	}
}

func TestCodexSessionRef(t *testing.T) {
	session, failure := CodexSessionRef("{\"type\":\"thread.started\",\"thread_id\":\"t1\"}\n{\"type\":\"error\",\"message\":\"quota\"}\n")
	if session != "t1" || failure != "quota" {
		t.Fatalf("session %q, failure %q", session, failure)
	}
}
