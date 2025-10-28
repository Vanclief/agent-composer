package shell

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	mcpproto "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vanclief/ez"
)

type shellRunArgs struct {
	Command string `json:"command"        jsonschema:"required" jsonschema_description:"Full shell command to execute using bash -lc"`
	Workdir string `json:"workdir"        jsonschema_description:"Optional working directory"`
}

type ShellRunResult struct {
	ExitCode     int    `json:"exit_code"`
	DurationMS   int64  `json:"duration_ms"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	TimedOut     bool   `json:"timed_out"`
	EffectiveDir string `json:"effective_dir"`
	CommandEcho  string `json:"command_echo"`
}

// NewServer constructs an in process MCP server exposing a single shell_run tool
func NewServer(rootDir string, allowedWorkdirs []string, defaultWorkdir string, maxTimeout time.Duration) (*server.MCPServer, error) {
	const op = "mcp.shell.NewServer"

	if maxTimeout <= 0 {
		maxTimeout = 3 * time.Minute
	}

	if rootDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
		rootDir = cwd
	}

	absoluteRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	resolver, err := newWorkdirResolver(absoluteRoot, allowedWorkdirs, defaultWorkdir)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	srv := server.NewMCPServer("Shell MCP", "0.1.0")

	shellTool := mcpproto.NewTool(
		"shell",
		mcpproto.WithDescription("Execute a bash command"),
		mcpproto.WithInputSchema[shellRunArgs](),
		mcpproto.WithOutputSchema[ShellRunResult](),
	)

	srv.AddTool(shellTool, mcpproto.NewStructuredToolHandler(func(
		ctx context.Context,
		_ mcpproto.CallToolRequest,
		args shellRunArgs,
	) (ShellRunResult, error) {
		// 1) Resolve workdir (this defines `workdir`)
		workdir, err := resolver.resolve(args.Workdir)
		if err != nil {
			return ShellRunResult{}, ez.Wrap(op, err)
		}

		// 2) Compute effective timeout (this defines `effectiveTimeout`)
		execCtx, cancel := context.WithTimeout(ctx, maxTimeout)
		defer cancel()

		start := time.Now()
		outcome, runErr := runBashIsolated(execCtx, workdir, args.Command)
		duration := time.Since(start)

		result := ShellRunResult{
			ExitCode:     outcome.ExitCode,
			DurationMS:   duration.Milliseconds(),
			Stdout:       outcome.Stdout,
			Stderr:       outcome.Stderr,
			TimedOut:     outcome.TimedOut,
			EffectiveDir: workdir,
			CommandEcho:  args.Command,
		}

		switch {
		case outcome.TimedOut:
			// Preserve your ez taxonomy
			return result, ez.New(op, ez.ERESOURCEEXHAUSTED, "command timed out", runErr)

		case runErr != nil:
			// runErr already contains the exit code message; keep your wrapping
			return result, ez.Wrap(op, runErr)

		default:
			return result, nil
		}
	}))

	return srv, nil
}

type workdirResolver struct {
	rootDir       string
	allowedAbs    []string
	allowAllUnder bool
	defaultAbs    string
}

func newWorkdirResolver(rootDir string, allowed []string, defaultWorkdir string) (*workdirResolver, error) {
	const op = "mcp.shell.newWorkdirResolver"

	resolver := &workdirResolver{rootDir: rootDir}

	if len(allowed) == 0 {
		resolver.allowAllUnder = true
	} else {
		resolver.allowedAbs = make([]string, 0, len(allowed))
		for _, entry := range allowed {
			clean := filepath.Clean(entry)
			if clean == "." || clean == "" {
				resolver.allowAllUnder = true
				resolver.allowedAbs = nil
				break
			}
			joined := filepath.Join(rootDir, clean)
			abs, err := filepath.Abs(joined)
			if err != nil {
				return nil, ez.Wrap(op, err)
			}
			rel, err := filepath.Rel(rootDir, abs)
			if err != nil || strings.HasPrefix(rel, "..") {
				return nil, ez.New(op, ez.ENOTAUTHORIZED, "allowed workdir escapes rootDir", nil)
			}
			resolver.allowedAbs = append(resolver.allowedAbs, abs)
		}
	}

	defaultAbs, err := resolver.normalize(defaultWorkdir)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}
	if !resolver.allowAllUnder {
		allowed := false
		for _, candidate := range resolver.allowedAbs {
			if defaultAbs == candidate || strings.HasPrefix(defaultAbs, candidate+string(os.PathSeparator)) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, ez.New(op, ez.ENOTAUTHORIZED, "default workdir not allowed", nil)
		}
	}
	checked, err := ensureDir(defaultAbs)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}
	resolver.defaultAbs = checked
	return resolver, nil
}

func (r *workdirResolver) resolve(requested string) (string, error) {
	const op = "mcp.shell.workdirResolver.resolve"

	target := r.defaultAbs
	if strings.TrimSpace(requested) != "" {
		abs, err := r.normalize(requested)
		if err != nil {
			return "", ez.Wrap(op, err)
		}
		target = abs
	}

	abs := target

	if !r.allowAllUnder {
		for _, allowed := range r.allowedAbs {
			if abs == allowed || strings.HasPrefix(abs, allowed+string(os.PathSeparator)) {
				return ensureDir(abs)
			}
		}
		return "", ez.New(op, ez.ENOTAUTHORIZED, "workdir not allowed", nil)
	}

	return ensureDir(abs)
}

func (r *workdirResolver) normalize(path string) (string, error) {
	const op = "mcp.shell.workdirResolver.normalize"

	if strings.TrimSpace(path) == "" {
		return r.rootDir, nil
	}

	clean := filepath.Clean(path)
	var candidate string
	if filepath.IsAbs(clean) {
		candidate = clean
	} else {
		candidate = filepath.Join(r.rootDir, clean)
	}

	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", ez.Wrap(op, err)
	}

	rel, err := filepath.Rel(r.rootDir, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", ez.New(op, ez.ENOTAUTHORIZED, "workdir escapes rootDir", nil)
	}

	return abs, nil
}

func ensureDir(path string) (string, error) {
	const op = "mcp.shell.ensureDir"

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ez.New(op, ez.ENOTFOUND, "workdir does not exist", err)
		}
		return "", ez.Wrap(op, err)
	}
	if !info.IsDir() {
		return "", ez.New(op, ez.EINVALID, "workdir must be a directory", nil)
	}
	return path, nil
}
