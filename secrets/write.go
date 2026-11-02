package secrets

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/vanclief/ez"
)

// Path is the file the store was read from.
func (s *Store) Path() string {
	return s.path
}

// Has reports whether the secret called name is set.
func (s *Store) Has(name string) bool {
	_, ok := s.values[name]
	return ok
}

// Set writes NAME=value to the file, replacing the lines that set name
// and keeping every other line as it was, and creates the file (0600, its
// directory 0700) when missing. The value is quoted as Load unquotes it,
// in single quotes, or double quotes when it holds a single quote; a value
// holding both, a control character, or surrounding whitespace is
// refused. Errors name the secret, never the value.
func (s *Store) Set(name string, value string) error {
	if !nameRe.MatchString(name) {
		msg := fmt.Sprintf("secret name %q must match [A-Z][A-Z0-9_]*", name)
		return ez.New(ez.EINVALID, msg, nil)
	}

	quoted, err := quote(name, value)
	if err != nil {
		return ez.Wrap(err)
	}

	lines, err := s.otherLines(name)
	if err != nil {
		return ez.Wrap(err)
	}

	lines = append(lines, name+"="+quoted)

	err = s.write(lines)
	if err != nil {
		return ez.Wrap(err)
	}

	s.values[name] = value

	return nil
}

// Remove drops the lines that set name from the file, keeping every other
// line as it was. Removing a name that is not set changes nothing.
func (s *Store) Remove(name string) error {
	_, ok := s.values[name]
	if !ok {
		return nil
	}

	lines, err := s.otherLines(name)
	if err != nil {
		return ez.Wrap(err)
	}

	err = s.write(lines)
	if err != nil {
		return ez.Wrap(err)
	}

	delete(s.values, name)

	return nil
}

// quote wraps value in quotes Load strips.
func quote(name string, value string) (string, error) {
	if value == "" {
		return "", ez.New(ez.EINVALID, "secret "+name+" needs a value", nil)
	}

	if strings.TrimSpace(value) != value {
		return "", ez.New(ez.EINVALID, "secret "+name+": the value cannot start or end with whitespace", nil)
	}

	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", ez.New(ez.EINVALID, "secret "+name+": the value cannot hold a control character or a line break", nil)
	}

	if !strings.Contains(value, "'") {
		return "'" + value + "'", nil
	}

	if !strings.Contains(value, `"`) {
		return `"` + value + `"`, nil
	}

	return "", ez.New(ez.EINVALID, "secret "+name+": the value cannot hold both a single and a double quote", nil)
}

// otherLines returns the file's lines without those that set name.
func (s *Store) otherLines(name string) ([]string, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, ez.New(ez.EINTERNAL, "cannot read secrets file "+s.path, err)
	}

	lines := []string{}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if setsName(line, name) {
			continue
		}
		lines = append(lines, line)
	}

	if len(lines) == 1 && lines[0] == "" {
		return []string{}, nil
	}

	return lines, nil
}

// setsName reports whether line is NAME=... (or export NAME=...) for name.
func setsName(line string, name string) bool {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))
	left, _, found := strings.Cut(trimmed, "=")

	return found && strings.TrimSpace(left) == name
}

// write replaces the file with lines atomically: a 0600 temp file in the
// same directory, renamed over it.
func (s *Store) write(lines []string) error {
	dir := filepath.Dir(s.path)

	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot create the directory of secrets file "+s.path, err)
	}

	tmp, err := os.CreateTemp(dir, ".secrets-*.env")
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot write secrets file "+s.path, err)
	}

	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	err = tmp.Chmod(0o600)
	if err != nil {
		_ = tmp.Close()
		return ez.New(ez.EINTERNAL, "cannot write secrets file "+s.path, err)
	}

	content := ""
	if len(lines) > 0 {
		content = strings.Join(lines, "\n") + "\n"
	}

	_, err = tmp.WriteString(content)
	if err != nil {
		_ = tmp.Close()
		return ez.New(ez.EINTERNAL, "cannot write secrets file "+s.path, err)
	}

	err = tmp.Sync()
	if err != nil {
		_ = tmp.Close()
		return ez.New(ez.EINTERNAL, "cannot write secrets file "+s.path, err)
	}

	err = tmp.Close()
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot write secrets file "+s.path, err)
	}

	err = os.Rename(tmpPath, s.path)
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot write secrets file "+s.path, err)
	}

	return nil
}
