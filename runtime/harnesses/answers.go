package harnesses

import (
	"strings"

	"github.com/vanclief/ez"
)

// ClaudeCodeAnswer parses what `claude -p --output-format json` prints and
// returns the final answer and the session id. A result claude marks as an
// error comes back as an error carrying its message.
func ClaudeCodeAnswer(rawOutput string) (string, string, error) {
	result, err := parseClaudeCodeResult(rawOutput)
	if err != nil {
		return "", "", ez.Wrap(err)
	}

	if result.IsError {
		return "", result.SessionID, ez.New(ez.EINTERNAL, result.ErrorMessage(), nil)
	}

	return result.Result, result.SessionID, nil
}

// CodexSessionRef returns the session (thread) id from the JSON lines
// `codex exec --json` prints, and the first error event's message, if any.
func CodexSessionRef(rawOutput string) (string, string) {
	summary := parseCodexRunSummary(rawOutput)

	return summary.SessionRef, strings.Join(summary.Errors, "; ")
}
