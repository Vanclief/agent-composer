package messaging

import "strings"

// Prefixes of agc's posts in #<task> (core/resources/tasks) that may be a
// turn for an agent.
const (
	DeliveredPrefix = "Delivered: "
	AcceptedPrefix  = "Accepted by "
	RejectedPrefix  = "Rejected by "
)

// FromSystem reports whether agc itself posted m.
func FromSystem(m *Message) bool {
	return m.From == SystemName && !m.Human && m.Task == "" && m.Agent == ""
}

// FromAgent reports whether an agent posted m: a task's sandbox or a
// persistent agent's home, neither a person nor agc.
func FromAgent(m *Message) bool {
	return !m.Human && (m.Task != "" || m.Agent != "")
}

// Names reports whether body names @name as a whole word, so @rev does
// not name reviewer-1 and @reviewer-1 does not name reviewer-10.
func Names(body string, name string) bool {
	if name == "" {
		return false
	}

	mention := "@" + name
	rest := body
	for {
		i := strings.Index(rest, mention)
		if i < 0 {
			return false
		}

		end := i + len(mention)
		if end == len(rest) || !nameByte(rest[end]) {
			return true
		}

		rest = rest[end:]
	}
}

func nameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// TurnFor reports whether m is a turn for the agent me (ADR 0016 item 6),
// the one rule behind the wake hook, the home loop and the task loop:
//
//   - its own posts never are (same participant, name or home);
//   - a person's post always is;
//   - another agent's post is only when it is a question or names @<me>,
//     so two agents never answer each other's plain posts in a loop;
//   - agc's post is only when it is a delivery, a rejection, an accept by
//     someone else, or a question naming @<me>. Callers narrow this: the
//     wake hook wakes only the task's manager and only on a delivery or
//     an accept, and the task loop takes only a rejection.
//
// It looks only at the message; a reply in a thread the agent started is
// a turn in any channel, by AnswerFor.
func TurnFor(m *Message, me *Participant) bool {
	if Own(m, me) {
		return false
	}

	if FromSystem(m) {
		if strings.HasPrefix(m.Body, RejectedPrefix) || strings.HasPrefix(m.Body, DeliveredPrefix) {
			return true
		}

		if strings.HasPrefix(m.Body, AcceptedPrefix) {
			return m.Body != AcceptedPrefix+me.Name+"."
		}

		return m.Kind == KindQuestion && Names(m.Body, me.Name)
	}

	if FromAgent(m) {
		return m.Kind == KindQuestion || Names(m.Body, me.Name)
	}

	return true
}

// Own reports whether me posted m: same participant, name or home.
func Own(m *Message, me *Participant) bool {
	if m.ParticipantID != "" && m.ParticipantID == me.ID {
		return true
	}
	if m.From == me.Name {
		return true
	}

	return m.Agent != "" && m.Agent == me.Agent
}

// AnswerFor reports whether m is a reply in a thread me started, root
// being the thread's first message: a turn for me in any channel, read or
// not (ADR 0016 item 6), so the asker always hears the answer. A person's
// or another agent's reply is; its own replies and agc's are not.
func AnswerFor(m *Message, root *Message, me *Participant) bool {
	if m.ReplyTo == nil || root == nil || root.ID == m.ID {
		return false
	}

	if Own(m, me) || FromSystem(m) {
		return false
	}

	return Own(root, me)
}
