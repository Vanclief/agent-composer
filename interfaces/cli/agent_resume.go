package cli

import (
	"context"

	"github.com/vanclief/ez"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// resumePrompt is the first turn of a task agent whose saved session
// continues after its sandbox was restarted (ADR 0017).
const resumePrompt = "agc restarted your sandbox; continue where you were; the repo clone may need a fresh pull."

// firstTurn is the task loop's first turn. When the sessions file holds
// task:<task>, the agent resumes that session with resumePrompt; otherwise
// it runs on the brief. From then on every session the harness reports is
// saved under that key, so a restart resumes the latest one (ADR 0017).
func (l *agentLoop) firstTurn(ctx context.Context, p *msgmodels.Participant) string {
	if l.sessions == nil {
		return l.turn(ctx, p, l.brief, "")
	}

	key := taskSessionKey(l.task)
	l.harness = &savingHarness{agentHarness: l.harness, sessions: l.sessions, key: key, logf: l.logf}

	saved := l.sessions.Get(key)
	if saved == "" {
		return l.turn(ctx, p, l.brief, "")
	}

	l.logf("resuming session %s after a restart", saved)

	// A session the harness cannot resume starts over on the brief.
	inner := l.harness
	l.harness = &resumeOrBrief{agentHarness: inner, brief: l.brief, logf: l.logf}
	session := l.turn(ctx, p, resumePrompt, saved)
	l.harness = inner

	return session
}

// savingHarness saves every new session id under key in the sessions file.
type savingHarness struct {
	agentHarness
	sessions *sessionStore
	key      string
	logf     func(format string, args ...any)
}

func (h *savingHarness) Run(ctx context.Context, prompt string, session string) (string, string, error) {
	answer, next, err := h.agentHarness.Run(ctx, prompt, session)
	if next != "" && next != h.sessions.Get(h.key) {
		saveErr := h.sessions.Set(h.key, next)
		if saveErr != nil {
			h.logf("cannot save the session: %s", ez.ErrorMessage(saveErr))
		}
	}

	return answer, next, err
}

// resumeOrBrief runs a turn on a saved session and, when the harness
// fails it, a new session on the brief instead.
type resumeOrBrief struct {
	agentHarness
	brief string
	logf  func(format string, args ...any)
}

func (h *resumeOrBrief) Run(ctx context.Context, prompt string, session string) (string, string, error) {
	answer, next, err := h.agentHarness.Run(ctx, prompt, session)
	if err == nil {
		return answer, next, nil
	}

	h.logf("cannot resume session %s, starting over on the brief: %s", session, ez.ErrorMessage(err))

	return h.agentHarness.Run(ctx, h.brief, "")
}
