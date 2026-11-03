package triggers

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/vanclief/ez"
)

// Cursor is what agc remembers about one source between polls: the time
// the last successful poll started (the next one asks for everything
// since then, less Overlap), the keys it already posted inside that
// overlap, and how the last poll went.
type Cursor struct {
	Source     string               `json:"source"`
	Cursor     time.Time            `json:"cursor"`
	LastPolled *time.Time           `json:"last_polled,omitempty"`
	LastError  string               `json:"last_error,omitempty"`
	Posted     int                  `json:"posted"`
	Seen       map[string]time.Time `json:"seen,omitempty"`
}

// state is the cursor file: one cursor per source.
type state struct {
	Sources map[string]*Cursor `json:"sources"`
}

// Store keeps the cursors in one JSON file under the agc home, written
// whole through a temporary file and a rename after every change, so a
// restart reads what was last posted and nothing posts twice.
type Store struct {
	path string

	mu    sync.Mutex
	state *state
}

// NewStore returns the store at path; the file is read on first use.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Path is the cursor file's path.
func (s *Store) Path() string {
	return s.path
}

func (s *Store) load() error {
	if s.state != nil {
		return nil
	}

	loaded := &state{Sources: map[string]*Cursor{}}

	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		s.state = loaded
		return nil
	}
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot read trigger cursors "+s.path, err)
	}

	err = json.Unmarshal(data, loaded)
	if err != nil {
		return ez.New(ez.EINTERNAL, "trigger cursors "+s.path+" is not valid JSON", err)
	}

	if loaded.Sources == nil {
		loaded.Sources = map[string]*Cursor{}
	}

	s.state = loaded
	return nil
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return ez.Wrap(err)
	}

	err = os.MkdirAll(filepath.Dir(s.path), 0o700)
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot create "+filepath.Dir(s.path), err)
	}

	tmp := s.path + ".tmp"
	err = os.WriteFile(tmp, data, 0o600)
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot write trigger cursors "+tmp, err)
	}

	err = os.Rename(tmp, s.path)
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot write trigger cursors "+s.path, err)
	}

	return nil
}

// Get returns a copy of the source's cursor, nil when it never polled.
func (s *Store) Get(source string) (*Cursor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.load()
	if err != nil {
		return nil, err
	}

	cursor, ok := s.state.Sources[source]
	if !ok {
		return nil, nil
	}

	return cursor.copy(), nil
}

// Update applies fn to the source's cursor (a new one when absent) and
// writes the file.
func (s *Store) Update(source string, fn func(cursor *Cursor)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.load()
	if err != nil {
		return err
	}

	cursor, ok := s.state.Sources[source]
	if !ok {
		cursor = &Cursor{Source: source}
		s.state.Sources[source] = cursor
	}

	if cursor.Seen == nil {
		cursor.Seen = map[string]time.Time{}
	}

	fn(cursor)

	return s.save()
}

// All returns a copy of every cursor, sorted by source.
func (s *Store) All() ([]Cursor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.load()
	if err != nil {
		return nil, err
	}

	cursors := make([]Cursor, 0, len(s.state.Sources))
	for _, cursor := range s.state.Sources {
		cursors = append(cursors, *cursor.copy())
	}

	sort.Slice(cursors, func(i, j int) bool {
		return cursors[i].Source < cursors[j].Source
	})

	return cursors, nil
}

func (c *Cursor) copy() *Cursor {
	out := *c
	out.Seen = make(map[string]time.Time, len(c.Seen))
	for key, at := range c.Seen {
		out.Seen[key] = at
	}

	return &out
}
