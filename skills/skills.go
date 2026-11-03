// Package skills is agc's skills repository: one directory per skill,
// <dir>/<name>/SKILL.md plus any files beside it, the Claude Code skill
// layout. agc writes the skills a sandbox spec names into the sandbox on
// every prepare; the copy in a sandbox is never the source
// (docs/decisions/0008-tasks-allocate-compute.md "Skills"). Everything is
// read from disk on every call, so an edit shows at once.
package skills

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/vanclief/ez"
	"gopkg.in/yaml.v3"
)

// FileName is the file every skill has.
const FileName = "SKILL.md"

// nameRe is a skill name: a lowercase slug.
var nameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateName checks name is a slug such as rtsdesign or code-review.
func ValidateName(name string) error {
	if !nameRe.MatchString(name) {
		msg := fmt.Sprintf("skill %q is not a valid name: use lowercase letters and digits separated by single dashes", name)
		return ez.New(ez.EINVALID, msg, nil)
	}

	return nil
}

// Skill is a skill in the repository, with the description from its
// SKILL.md frontmatter, empty when it has none.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// File is a file of a skill: Path is relative to the skill's directory,
// with forward slashes.
type File struct {
	Path       string
	Data       []byte
	Executable bool
}

// Repo is the skills repository at Dir. An empty Dir, or one that does not
// exist, holds no skills.
type Repo struct {
	Dir string
}

// Names lists the skills in the repository, sorted: the directories whose
// name is a valid skill name.
func (r Repo) Names() ([]string, error) {
	names := []string{}
	if r.Dir == "" {
		return names, nil
	}

	entries, err := os.ReadDir(r.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return nil, ez.New(ez.EINTERNAL, "cannot read the skills directory "+r.Dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() && nameRe.MatchString(entry.Name()) {
			names = append(names, entry.Name())
		}
	}

	sort.Strings(names)

	return names, nil
}

// List lists the skills with their descriptions, sorted by name.
func (r Repo) List() ([]Skill, error) {
	names, err := r.Names()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	list := []Skill{}
	for _, name := range names {
		description := ""

		text, err := r.Text(name)
		if err == nil {
			description = Description(text)
		}

		list = append(list, Skill{Name: name, Description: description})
	}

	return list, nil
}

// Text is the skill's SKILL.md, ENOTFOUND when the skill or its file is
// absent.
func (r Repo) Text(name string) (string, error) {
	err := ValidateName(name)
	if err != nil {
		return "", ez.New(ez.ENOTFOUND, "skill "+name+" not found", err)
	}

	if r.Dir == "" {
		return "", ez.New(ez.ENOTFOUND, "skill "+name+" not found: no skills directory is set", nil)
	}

	data, err := os.ReadFile(filepath.Join(r.Dir, name, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return "", ez.New(ez.ENOTFOUND, "skill "+name+" not found", err)
	}
	if err != nil {
		return "", ez.New(ez.EINTERNAL, "cannot read skill "+name, err)
	}

	return string(data), nil
}

// Check refuses a name that is not a skill in the repository, naming it
// and the known skills.
func (r Repo) Check(names []string) error {
	if len(names) == 0 {
		return nil
	}

	known, err := r.Names()
	if err != nil {
		return ez.Wrap(err)
	}

	for _, name := range names {
		found := false
		for _, candidate := range known {
			if candidate == name {
				found = true
			}
		}

		if found {
			continue
		}

		list := "none"
		if len(known) > 0 {
			list = strings.Join(known, ", ")
		}

		msg := fmt.Sprintf("skill %q is not in the skills directory %s (known skills: %s)", name, r.Dir, list)
		return ez.New(ez.EINVALID, msg, nil)
	}

	return nil
}

// Files reads every regular file of the skill, SKILL.md included, sorted
// by path. Symbolic links and other special files are skipped.
func (r Repo) Files(name string) ([]File, error) {
	err := r.Check([]string{name})
	if err != nil {
		return nil, ez.Wrap(err)
	}

	root := filepath.Join(r.Dir, name)
	files := []File{}

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.Type().IsRegular() {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		files = append(files, File{Path: filepath.ToSlash(rel), Data: data, Executable: info.Mode()&0o111 != 0})

		return nil
	})
	if err != nil {
		return nil, ez.New(ez.EINTERNAL, "cannot read skill "+name, err)
	}

	return files, nil
}

// Description is the description in a SKILL.md's YAML frontmatter, the
// block between a first line --- and the next ---, empty when there is
// none or it does not parse.
func Description(text string) string {
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")

	if !strings.HasPrefix(text, "---\n") {
		return ""
	}

	rest := text[len("---\n"):]

	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}

	front := struct {
		Description string `yaml:"description"`
	}{}

	err := yaml.NewDecoder(bytes.NewReader([]byte(rest[:end+1]))).Decode(&front)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(front.Description)
}
