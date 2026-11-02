package cli

import (
	"encoding/json"
	"os"
	"testing"
)

// readJSONFile is the helper sessions_test.go uses; agc/agents-loop
// defines it in agent_home_test.go, so whoever merges that branch second
// deletes this file, as sessions_write.go.
func readJSONFile(t *testing.T, path string, out any) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(data, out)
	if err != nil {
		t.Fatal(err)
	}
}
