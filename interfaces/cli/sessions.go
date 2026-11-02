package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vanclief/ez"
)

// The sessions file maps a key to a harness session id. It lives on the
// sandbox's kept volume (/home/agent/.harness), so sessions outlive the
// VM, and it is shared by every loop that runs in the sandbox:
//
//   - "<channel>:<root id>": a persistent agent's thread (ADR 0016 item 7)
//   - "main": the main role's one session (ADR 0016 item 7)
//   - "task:<task name>": the task-mode loop (ADR 0017)
const sessionsFile = "agc-sessions.json"

// mainSessionKey is main's session in the sessions file.
const mainSessionKey = "main"

// threadSessionKey is a persistent agent's session for one thread.
func threadSessionKey(channel string, root int64) string {
	return fmt.Sprintf("%s:%d", channel, root)
}

// taskSessionKey is the task-mode loop's session for task name.
func taskSessionKey(name string) string {
	return "task:" + name
}

// sessionStore is the sessions file: load it once, Get from memory, Set
// writes through. Every key is optional and nobody owns the whole file.
type sessionStore struct {
	path     string
	sessions map[string]string
}

// loadSessionStore reads the sessions file in dir. A missing file is an
// empty store; a corrupt one is an empty store too, with an error saying
// so for the caller to log: a lost session only costs a fresh one.
func loadSessionStore(dir string) (*sessionStore, error) {
	store := &sessionStore{path: filepath.Join(dir, sessionsFile), sessions: map[string]string{}}

	sessions, err := readSessions(store.path)
	if err != nil {
		return store, err
	}

	store.sessions = sessions

	return store, nil
}

func readSessions(path string) (map[string]string, error) {
	sessions := map[string]string{}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return sessions, nil
	}
	if err != nil {
		return sessions, ez.New(ez.EINTERNAL, "cannot read the sessions file "+path, err)
	}

	err = json.Unmarshal(data, &sessions)
	if err != nil || sessions == nil {
		return map[string]string{}, ez.New(ez.EINVALID, "the sessions file "+path+" is corrupt, starting new sessions", err)
	}

	return sessions, nil
}

// Get is the session saved under key, empty when there is none.
func (s *sessionStore) Get(key string) string {
	return s.sessions[key]
}

// Set saves id under key. It reads the file again first, so keys another
// loop wrote since the load are kept, then replaces it atomically.
func (s *sessionStore) Set(key string, id string) error {
	current, err := readSessions(s.path)
	if err == nil {
		for k, v := range current {
			_, mine := s.sessions[k]
			if !mine {
				s.sessions[k] = v
			}
		}
	}

	s.sessions[key] = id

	return s.Save()
}

// Save writes the store to its file.
func (s *sessionStore) Save() error {
	return writeJSONFile(s.path, s.sessions)
}
