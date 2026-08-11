package harnesses

import (
	"strings"

	"github.com/vanclief/ez"
)

// Permissions is the harness-agnostic access tier declared in the DSL under
// config.harness.permissions. It is the single knob that controls what an agent
// may do to the working tree and the host; both the Claude Code and Codex
// harnesses translate the same tier into their own backend flags via the
// resolvers below, so the mapping for both lives in one reviewable place.
type Permissions string

const (
	// PermissionsReadOnly is the default. The agent may read and inspect the
	// workspace but may not modify it.
	//
	// Enforcement differs by backend and is intentionally documented here:
	//   - Claude Code has no filesystem sandbox, so we gate by tool: the write
	//     and shell tools are hard-disallowed, leaving only read tools. That
	//     means a read_only Claude agent cannot run shell commands at all.
	//   - Codex read-only sandbox blocks filesystem writes and network but still
	//     lets commands run, so a read_only Codex agent CAN run read-only probes
	//     (grep, and builds/tests that don't need to write). Both honor the core
	//     guarantee "cannot modify the workspace or reach the network".
	PermissionsReadOnly Permissions = "read_only"

	// PermissionsExec lets the agent modify the workspace and run shell commands
	// scoped to it (tests, builds, formatters).
	//
	// Codex enforces this as a real sandbox: workspace writes and shell, no
	// network, no writes outside the workspace. Claude Code has no network
	// sandbox, so once shell is allowed it can also reach the network — on
	// Claude, exec and dangerously-exec are therefore nearly identical in
	// practice. The tier means "as locked-down as this backend can express".
	PermissionsExec Permissions = "exec"

	// PermissionsDangerouslyExec removes all guardrails: network access, writes
	// anywhere, and no approvals. Use only for trusted, disposable environments.
	PermissionsDangerouslyExec Permissions = "dangerously-exec"
)

// defaultPermissions is applied when config.harness.permissions is omitted.
const defaultPermissions = PermissionsReadOnly

// ParsePermissions normalizes and validates a raw permissions string. An empty
// value resolves to the safe default (read_only).
func ParsePermissions(raw string) (Permissions, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultPermissions, nil
	}

	p := Permissions(trimmed)
	switch p {
	case PermissionsReadOnly, PermissionsExec, PermissionsDangerouslyExec:
		return p, nil
	default:
		return "", ez.New(ez.EINVALID, "invalid permissions: must be one of read_only, exec, dangerously-exec", nil)
	}
}

// claudePermissionFlags is the resolved Claude Code representation of a tier.
type claudePermissionFlags struct {
	// PermissionMode is passed to --permission-mode. Empty when the tier relies
	// on --dangerously-skip-permissions instead.
	PermissionMode string
	// DisallowedTools are merged into --disallowedTools to hard-block writes and
	// shell for read_only.
	DisallowedTools []string
	// DangerouslySkip toggles --dangerously-skip-permissions.
	DangerouslySkip bool
}

// resolveClaudePermissions maps a tier onto Claude Code's approval model.
func resolveClaudePermissions(p Permissions) claudePermissionFlags {
	switch p {
	case PermissionsExec:
		return claudePermissionFlags{PermissionMode: "bypassPermissions"}
	case PermissionsDangerouslyExec:
		return claudePermissionFlags{DangerouslySkip: true}
	default: // PermissionsReadOnly
		return claudePermissionFlags{
			PermissionMode:  "default",
			DisallowedTools: []string{"Edit", "Write", "NotebookEdit", "Bash"},
		}
	}
}

// codexPermissionFlags is the resolved Codex representation of a tier.
type codexPermissionFlags struct {
	// Sandbox is passed to --sandbox. Empty when the tier relies on
	// --dangerously-bypass-approvals-and-sandbox instead.
	Sandbox string
	// DangerousBypass toggles --dangerously-bypass-approvals-and-sandbox.
	DangerousBypass bool
}

// resolveCodexPermissions maps a tier onto Codex's sandbox model.
func resolveCodexPermissions(p Permissions) codexPermissionFlags {
	switch p {
	case PermissionsExec:
		return codexPermissionFlags{Sandbox: "workspace-write"}
	case PermissionsDangerouslyExec:
		return codexPermissionFlags{DangerousBypass: true}
	default: // PermissionsReadOnly
		return codexPermissionFlags{Sandbox: "read-only"}
	}
}

// rejectLegacyPermissionKeys returns a migration error if a harness config still
// carries one of the pre-`permissions` raw permission fields, so old workflows
// fail loud instead of silently ignoring the setting.
func rejectLegacyPermissionKeys(raw map[string]any, legacyKeys []string) error {
	for _, key := range legacyKeys {
		_, found := raw[key]
		if found {
			return ez.New(ez.EINVALID, "'"+key+"' is no longer supported; use config.harness.permissions (read_only, exec, dangerously-exec)", nil)
		}
	}

	return nil
}
