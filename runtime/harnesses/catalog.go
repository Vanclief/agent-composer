package harnesses

import (
	"bytes"
	"context"
	"encoding/json"
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

// Model knowledge is per harness: pi (`pi --list-models`) and codex
// (`codex debug models`) ship queryable catalogs; claude exposes no
// listing command, so a curated list of its common models stands in.
// Any model id typed by hand still works — these are suggestions,
// not a gate.

// Fallback when `codex debug models` fails — its catalog per 2026-08.
var codexModels = []string{
	"gpt-5.6-sol",
	"gpt-5.6-terra",
	"gpt-5.6-luna",
	"gpt-5.5",
	"gpt-5.4",
	"gpt-5.4-mini",
	"gpt-5.3-codex-spark",
}

var claudeModels = []string{
	"claude-fable-5",
	"claude-opus-5",
	"claude-sonnet-5",
	"claude-haiku-4-5",
	"claude-opus-4-8",
	"claude-opus-4-7",
	"claude-opus-4-5",
	"claude-sonnet-4-5",
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

	codex := HarnessInfo{
		ID:        agent.HarnessCodexCLI,
		Binary:    "codex",
		Available: binaryAvailable("codex"),
		Models:    codexModels,
	}
	if codex.Available {
		if models := listCodexModels(ctx); len(models) > 0 {
			codex.Models = models
		}
	}

	infos := []HarnessInfo{
		codex,
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

// listCodexModels reads codex's own model catalog. The command is
// local and fast (~150ms, cached by codex itself); "hide" entries are
// internal models not meant for the picker.
func listCodexModels(ctx context.Context) []string {
	cmd := exec.CommandContext(ctx, "codex", "debug", "models")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return nil
	}

	var catalog struct {
		Models []struct {
			Slug       string `json:"slug"`
			Visibility string `json:"visibility"`
		} `json:"models"`
	}
	err = json.Unmarshal(stdout.Bytes(), &catalog)
	if err != nil {
		return nil
	}

	models := []string{}
	for _, model := range catalog.Models {
		if model.Slug == "" || model.Visibility == "hide" {
			continue
		}
		models = append(models, model.Slug)
	}

	return models
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
