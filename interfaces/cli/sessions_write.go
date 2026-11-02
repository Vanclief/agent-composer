package cli

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/vanclief/ez"
)

// writeJSONFile replaces path with value as JSON, atomically. It is the
// helper sessions.go uses; agc/agents-loop defines the same function in
// agent_home.go, so whoever merges that branch second deletes this file.
func writeJSONFile(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return ez.Wrap(err)
	}

	err = os.MkdirAll(filepath.Dir(path), 0o700)
	if err != nil {
		return ez.Wrap(err)
	}

	tmp := path + ".tmp"

	err = os.WriteFile(tmp, encoded, 0o600)
	if err != nil {
		return ez.Wrap(err)
	}

	err = os.Rename(tmp, path)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
