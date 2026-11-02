package task

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vanclief/ez"

	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// Store keeps saved specs: reusable sandbox specs (the sandbox: block of a
// task.yaml, at top level), one per spec, in Dir as <name>.yaml. The YAML is
// kept verbatim so comments survive.
type Store struct {
	Dir string
}

// Saved is a saved spec.
type Saved struct {
	Name string `json:"name"`
	YAML string `json:"yaml"`
}

// List returns the saved specs sorted by name. A missing Dir has none.
func (s Store) List() ([]Saved, error) {
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Saved{}, nil
	}
	if err != nil {
		return nil, ez.New(ez.EINTERNAL, "cannot read saved specs in "+s.Dir, err)
	}

	saved := []Saved{}
	for _, entry := range entries {
		name, found := strings.CutSuffix(entry.Name(), ".yaml")
		if !found || entry.IsDir() || taskmodels.ValidateName(name) != nil {
			continue
		}

		spec, err := s.Get(name)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		saved = append(saved, spec)
	}

	sort.Slice(saved, func(i, j int) bool {
		return saved[i].Name < saved[j].Name
	})

	return saved, nil
}

// Get reads the saved spec called name.
func (s Store) Get(name string) (Saved, error) {
	err := taskmodels.ValidateName(name)
	if err != nil {
		return Saved{}, ez.Wrap(err)
	}

	data, err := os.ReadFile(s.path(name))
	if errors.Is(err, fs.ErrNotExist) {
		return Saved{}, ez.New(ez.ENOTFOUND, "saved spec "+name+" not found", err)
	}
	if err != nil {
		return Saved{}, ez.New(ez.EINTERNAL, "cannot read saved spec "+name, err)
	}

	return Saved{Name: name, YAML: string(data)}, nil
}

// Save validates yamlText as a sandbox spec and writes it as name,
// replacing any saved spec of that name.
func (s Store) Save(name string, yamlText string) (Saved, error) {
	err := taskmodels.ValidateName(name)
	if err != nil {
		return Saved{}, ez.Wrap(err)
	}

	parsed, err := ParseSandbox([]byte(yamlText))
	if err != nil {
		return Saved{}, ez.Wrap(err)
	}

	err = parsed.Validate()
	if err != nil {
		return Saved{}, ez.Wrap(err)
	}

	err = os.MkdirAll(s.Dir, 0o755)
	if err != nil {
		return Saved{}, ez.New(ez.EINTERNAL, "cannot create "+s.Dir, err)
	}

	// Write then rename, so a reader never sees half a file.
	tmp, err := os.CreateTemp(s.Dir, "."+name+".*.tmp")
	if err != nil {
		return Saved{}, ez.New(ez.EINTERNAL, "cannot save spec "+name, err)
	}
	defer os.Remove(tmp.Name()) // nolint:errcheck // Gone after the rename.

	_, err = tmp.WriteString(yamlText)
	if err != nil {
		_ = tmp.Close()
		return Saved{}, ez.New(ez.EINTERNAL, "cannot save spec "+name, err)
	}

	err = tmp.Close()
	if err != nil {
		return Saved{}, ez.New(ez.EINTERNAL, "cannot save spec "+name, err)
	}

	err = os.Chmod(tmp.Name(), 0o644)
	if err != nil {
		return Saved{}, ez.New(ez.EINTERNAL, "cannot save spec "+name, err)
	}

	err = os.Rename(tmp.Name(), s.path(name))
	if err != nil {
		return Saved{}, ez.New(ez.EINTERNAL, "cannot save spec "+name, err)
	}

	return Saved{Name: name, YAML: yamlText}, nil
}

// Delete removes the saved spec called name.
func (s Store) Delete(name string) error {
	err := taskmodels.ValidateName(name)
	if err != nil {
		return ez.Wrap(err)
	}

	err = os.Remove(s.path(name))
	if errors.Is(err, fs.ErrNotExist) {
		return ez.New(ez.ENOTFOUND, "saved spec "+name+" not found", err)
	}
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot delete saved spec "+name, err)
	}

	return nil
}

func (s Store) path(name string) string {
	return filepath.Join(s.Dir, name+".yaml")
}
