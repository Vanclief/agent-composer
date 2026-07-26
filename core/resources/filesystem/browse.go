// Package filesystem lets the UI browse the server's directories —
// the browser sandbox cannot reveal absolute paths, but this process
// runs on the same machine the workflows do.
package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vanclief/ez"
)

type API struct{}

func NewAPI() *API {
	return &API{}
}

type BrowseRequest struct {
	// Path to list; empty means the user's home directory.
	Path string `json:"path,omitempty"`
}

func (r *BrowseRequest) Validate() error {
	return nil
}

type DirectoryEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// HasGit reports a .git entry — the UI badges repositories.
	HasGit bool `json:"has_git"`
}

type BrowseResponse struct {
	Path        string           `json:"path"`
	Parent      string           `json:"parent,omitempty"`
	Directories []DirectoryEntry `json:"directories"`
}

func (api *API) Browse(ctx context.Context, requester interface{}, request *BrowseRequest) (*BrowseResponse, error) {
	const op = "filesystem.API.Browse"

	path := strings.TrimSpace(request.Path)
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
		path = home
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return nil, ez.New(op, ez.ENOTFOUND, path+" is not a directory", err)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, ez.New(op, ez.EINVALID, "Cannot read "+path, err)
	}

	directories := []DirectoryEntry{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		full := filepath.Join(path, entry.Name())
		_, gitErr := os.Stat(filepath.Join(full, ".git"))
		directories = append(directories, DirectoryEntry{
			Name:   entry.Name(),
			Path:   full,
			HasGit: gitErr == nil,
		})
	}
	sort.Slice(directories, func(i, j int) bool {
		return directories[i].Name < directories[j].Name
	})

	parent := filepath.Dir(path)
	if parent == path {
		parent = ""
	}

	return &BrowseResponse{
		Path:        path,
		Parent:      parent,
		Directories: directories,
	}, nil
}
