package cli

import (
	"context"
	"os"
	"path/filepath"
	"strconv"

	"github.com/vanclief/ez"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// defaultImagesDir is where the agent loops put a turn's images (ADR 0005
// "Images"): <dir>/<message id>/<name>, inside the sandbox, so the
// harness can read them by path.
const defaultImagesDir = "/home/agent/.agc/images"

// imageFetcher downloads a message's image over the sandbox's socket.
type imageFetcher interface {
	Image(ctx context.Context, messageID int64, name string) ([]byte, error)
}

// imagePaths downloads m's images under dir, skipping those already
// there, and answers a line per image for the prompt: its local path, or
// its URL and why it could not be fetched.
func imagePaths(ctx context.Context, fetcher imageFetcher, dir string, m msgmodels.Message) []string {
	if len(m.Images) == 0 {
		return nil
	}

	if dir == "" {
		dir = defaultImagesDir
	}

	lines := make([]string, 0, len(m.Images))
	for _, image := range m.Images {
		path, err := fetchImage(ctx, fetcher, dir, m.ID, image.Name)
		if err != nil {
			lines = append(lines, image.URL+" (not fetched: "+ez.ErrorMessage(err)+")")
			continue
		}

		lines = append(lines, path)
	}

	return lines
}

// fetchImage keeps one image at <dir>/<message id>/<name> and answers the
// path. The name is the one agc stored, already a plain file name; Base
// keeps a bad one from leaving the directory anyway.
func fetchImage(ctx context.Context, fetcher imageFetcher, dir string, messageID int64, name string) (string, error) {
	base := filepath.Base(name)
	if base == "." || base == ".." || base == string(filepath.Separator) {
		return "", ez.New(ez.EINVALID, "bad image name "+name, nil)
	}

	path := filepath.Join(dir, strconv.FormatInt(messageID, 10), base)

	_, err := os.Stat(path)
	if err == nil {
		return path, nil
	}

	data, err := fetcher.Image(ctx, messageID, name)
	if err != nil {
		return "", ez.Wrap(err)
	}

	err = os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return "", ez.Wrap(err)
	}

	err = os.WriteFile(path, data, 0o644)
	if err != nil {
		return "", ez.Wrap(err)
	}

	return path, nil
}

// imageLines renders a message's image lines under its line in a prompt.
func imageLines(paths []string) string {
	out := ""
	for _, path := range paths {
		out += "  image: " + path + "\n"
	}

	return out
}
