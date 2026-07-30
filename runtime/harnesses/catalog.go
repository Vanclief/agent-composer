package harnesses

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/vanclief/agent-composer/models/agent"
)

// HarnessInfo describes one harness: whether its binary is installed
// and which models it can run.
type HarnessInfo struct {
	ID        agent.Harness `json:"id"`
	Binary    string        `json:"binary"`
	Available bool          `json:"available"`
	Models    []string      `json:"models"`
}

// Model knowledge is per harness: pi ships a queryable catalog
// (`pi --list-models`); codex and claude expose none, so a curated
// list of their common models stands in.
var codexModels = []string{
	"gpt-5.5",
	"gpt-5.5-codex",
	"gpt-5-codex",
	"gpt-5",
	"gpt-5-mini",
}

var claudeModels = []string{
	"claude-opus-5",
	"claude-sonnet-5",
	"claude-opus-4-5",
	"claude-sonnet-4-5",
	"claude-haiku-4-5",
}

const catalogTTL = 5 * time.Minute

var catalogCache struct {
	sync.Mutex
	infos     []HarnessInfo
	refreshed time.Time
}

// ListHarnessInfo reports every known harness with availability and
// models. Results are cached briefly — pi's catalog only changes on
// `pi update`.
func ListHarnessInfo(ctx context.Context) []HarnessInfo {
	catalogCache.Lock()
	defer catalogCache.Unlock()

	if catalogCache.infos != nil &&
		time.Since(catalogCache.refreshed) < catalogTTL {
		return catalogCache.infos
	}

	infos := []HarnessInfo{
		{
			ID:        agent.HarnessCodexCLI,
			Binary:    "codex",
			Available: binaryAvailable("codex"),
			Models:    codexModels,
		},
		{
			ID:        agent.HarnessClaudeCode,
			Binary:    "claude",
			Available: binaryAvailable("claude"),
			Models:    claudeModels,
		},
	}

	pi := HarnessInfo{
		ID:        agent.HarnessPi,
		Binary:    "pi",
		Available: binaryAvailable("pi"),
	}
	if pi.Available {
		pi.Models = listPiModels(ctx)
	}
	infos = append(infos, pi)

	catalogCache.infos = infos
	catalogCache.refreshed = time.Now()

	return infos
}

func binaryAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func listPiModels(ctx context.Context) []string {
	// --offline keeps this fast: no network refresh, local catalog only.
	cmd := exec.CommandContext(ctx, "pi", "--list-models", "--offline")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return nil
	}

	models := []string{}
	for index, line := range strings.Split(stdout.String(), "\n") {
		if index == 0 {
			// Header row: provider  model  context  …
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		models = append(models, fields[0]+"/"+fields[1])
	}

	return models
}
