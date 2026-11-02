# Agent Composer

**WARNING:** Early alpha. No sandboxing yet. If you enable shell access to an agent, assume the LLM has full file and network access.

Agent Composer (agc) runs multi-agent workflows defined as YAML specs: reviewers in parallel, summarize-critique-revise chains, loops, conditionals — executed by real coding-agent harnesses (Codex, Claude Code) with every run recorded.

## Documentation:

- **Rest Server**: https://vanclief.github.io/agent-composer

## Requirements

None. agc stores everything in a local SQLite database by default — no external services.

Workflows that use a coding-agent harness need that harness installed and authenticated (the installer bundles **Codex**; GPT models often call an `apply-patch` tool that isn't a native shell tool, but it's bundled in Codex).

## Installation

**Step 1: Install the binary**

```bash
curl -fsSL https://raw.githubusercontent.com/vanclief/agent-composer/master/install.sh | bash
```

The installer downloads the latest [release](https://github.com/vanclief/agent-composer/releases/latest) binary for your platform (macOS/Linux, amd64/arm64).

**Step 2: Reload your shell or open a new terminal**

The installer adds `~/.agent_composer/bin` to your PATH.

```
exec $SHELL -l
# or:
source ~/.zshrc   # zsh
source ~/.bashrc  # bash
```

You should then be able to run `agc`:

```
which agc
agc --help
```

That's it. The first command creates `~/.agent_composer/agc.db` with the current schema.

## Concepts

- **Workflow** — the unit you install and run. It has a **slug** (human handle, e.g. `parallel_pr_review`), a permanent **id** (uuid), an integer **version**, and full version history.
- **Spec** — the YAML document defining what a workflow does. Import a spec file to install a workflow; export one to get the YAML back. Every change to an installed workflow bumps its version and keeps the old spec in history.
- **Project** — the directory a run executes in. Defaults to wherever you invoke `agc`; pass `--project <dir>` to point elsewhere. Optionally a run can execute in a git **worktree** of the project (`--worktree <branch>`), isolating file changes from your checkout.
- **Run** — one execution of a workflow. Recorded in the database with the compiled spec snapshot, per-node results, and the resolved `project_dir`.

## Usage

**Run a workflow from a spec file**

```bash
agc workflow run \
  --file examples/article_summary.yaml \
  --input-string "Text of the article to summarize…"
```

The run executes in your current directory, prints one progress line per node, and blocks until it finishes with the result JSON. `--input-string` works for workflows with exactly one top-level `string` input; otherwise pass `--input-json '{"…":…}'` or `--input-file inputs.json`.

**Install a workflow, then run it by slug**

```bash
agc workflow import --file examples/parallel_pr_review.yaml
agc workflow run --slug parallel_pr_review --input-string master
```

Exactly one of `--slug` (installed workflow) or `--file` (spec on disk) is required. Workflow commands live under `agc workflow`, with `agc wf` and `agc w` as aliases.

**Run in a git worktree**

```bash
agc workflow run --slug my_fixer --input-string "…" --worktree feature-x --base main
```

The run executes in the worktree for `feature-x` (created from `main` on demand), leaving your checkout untouched.

**Inspect the registry**

```bash
agc workflow list
agc workflow show --slug parallel_pr_review
agc workflow versions --slug parallel_pr_review
agc workflow restore --slug parallel_pr_review --version 3
agc workflow export --slug parallel_pr_review --file ./parallel_pr_review.yaml
agc workflow delete --slug parallel_pr_review
```

`versions` lists the full history; `restore` re-installs a past version as a new head (history is never rewritten). `delete` removes the workflow but keeps its version history and run history.

**Compile without running**

```bash
agc workflow compile --file examples/parallel_pr_review.yaml
```

**Show the effective configuration**

```bash
agc config
```

Prints which database agc uses and *why* (config file found or not, postgres opt-in or sqlite default), plus the paths involved.

**Web UI + REST server**

```bash
agc rest
```

Workflow monitor, canvas, and composer at `http://localhost:1202`. For live reload during Go development: `air`.

```bash
curl http://localhost:1202/api/workflows
curl http://localhost:1202/api/workflows/parallel_pr_review

curl -X POST http://localhost:1202/api/workflow/executions \
  -H 'Content-Type: application/json' \
  -d '{"workflow_slug":"parallel_pr_review","input":{"branch":"master"}}'

curl http://localhost:1202/api/workflow/executions/<execution_id>
```

Creating an execution through REST returns an execution id immediately; poll the get endpoint for `running`, `succeeded`, or `failed`. CLI runs and server runs share the same database, so everything shows up in the UI either way.

**Using the CLI against a server**

`agc task …` and `agc sandbox …` normally open the local database and boot sandboxes in-process. Set `AGC_SERVER` (or pass `--server <url>`, before or after the subcommand) and they call that agc server's REST API instead, so a task can be created from any machine:

```bash
export AGC_SERVER=http://127.0.0.1:8788

agc task create -f task.yaml          # returns at once: task open, sandbox starting
agc task create -f task.yaml --wait   # polls until the sandbox is running or failed
agc task list
agc task get fix-login
agc task deliver --task fix-login "Fixed 2FA; branch agc/fix-login" --report report.json  # or --report - for stdin
agc task reject fix-login --why "the login test still fails"
agc task accept fix-login             # done; its sandboxes are stopped, not deleted
agc task delete fix-login --yes

agc sandbox allocate fix-login -f sandbox.yaml [--wait]
agc sandbox list
agc sandbox get fix-login
agc sandbox stop fix-login
agc sandbox start fix-login
agc sandbox delete fix-login --yes

agc task spec ls                      # saved specs are the server's
agc --server http://rig:8788 task ls  # one-off, without the env var
```

`task.yaml` is the ADR 0008 shape: `name`, `title`, `brief`, `assignee`, `for`, and `sandbox:` holding the environment block plus `command`. `agc task create` against a server prints the task and its page, `<server>/panel/tasks/<name>`. Every command prints the same lines in both modes; add `--json` for the raw view the API returns. A server error prints the server's message as is and exits 1. `agc task deliver --report` takes the delivery report of ADR 0010 ("Delivery report"): a JSON object with `criteria` (one `{met: yes|no|partly, note}` per accept criterion, in order), `changes` (`branch`, `sha`, `files`), `checks` (`name`, `passed`, `note`), `left_out` (`what`, `why`) and `needs`; agc stores it on the task and renders it under "Delivered:" in `#<task>`, and a bad report is refused with the server's error. Without `AGC_SERVER` or `--server` everything runs locally as before, and local `create`/`allocate` always wait for the boot, since it runs in that process.

**Scheduled tasks**

A schedule creates a task from a template every interval, or, with a `poll:` command, one task per new item the command lists as JSON lines (ADR 0012). `agc rest` runs the scheduler; `agc schedule` works locally or against `AGC_SERVER` like `agc task`, and locally a schedule runs only with `agc schedule run`. See [docs/schedules/pr-reviews.yaml](docs/schedules/pr-reviews.yaml):

```bash
agc schedule create -f docs/schedules/pr-reviews.yaml
agc schedule list
agc schedule show pr-reviews          # template, last error and the items seen
agc schedule run pr-reviews           # tick now; prints the tasks it created
agc schedule disable pr-reviews
agc schedule enable pr-reviews
agc schedule delete pr-reviews        # its tasks are kept
```

**MCP server**

```bash
agc mcp
```

Exposes `agc_workflow_list`, `agc_workflow_start` (by `slug` or `file`, returns an execution id immediately), and `agc_workflow_get` for polling.

**Logging**

One-shot commands only print their result; pass `--verbose` to see boot and database logs. Servers always log fully.

## Configuration

agc needs no configuration. To opt into PostgreSQL instead of SQLite, create `~/.agent_composer/config/local.config.json`:

```json
{
  "postgres": {
    "host": "localhost:5432",
    "username": "agent_composer",
    "database": "agent_composer"
  }
}
```

A `postgres` section with a host or database set selects PostgreSQL; without one (or without a config file) agc uses `~/.agent_composer/agc.db`. `agc config` always tells you which is active and why. `POSTGRES_PASSWORD` can be passed as an environment variable.

**Knowledge and agents directories**

agc reads two directories under its home, `~/.agent_composer`, on every request, so edits show at once:

- `knowledge/` holds the evidence, beliefs, decisions, rules and actions of ADR 0006 (`AGC_KNOWLEDGE_DIR` overrides it).
- `agents/` holds the persistent agents' role templates, `<role>.yaml` (`AGC_AGENTS_DIR` overrides it; ADR 0016). A template sets `harness`, `model`, `sandbox` (a task sandbox spec, `disk` included), `idle` (default `15m`), `reads` (channel patterns the agent watches besides its DM and its tasks' channels; `*` is every channel) and `instructions` (the agent's AGENT.md). The role is the file name, case-insensitive: `agc agent create Reviewer` reads `reviewer.yaml`. The repo ships `agents/reviewer.yaml` and `agents/main.yaml`; copy them into `~/.agent_composer/agents/`.

## Updating

Re-run the install command from Installation — it always fetches the latest release. Check what you have with `agc --version`.

## Releasing

Maintainers cut releases from the Actions tab: run the **Release** workflow on `master` and pick `patch`, `minor`, or `major`. It runs the test suite, builds binaries for all platforms, tags the next semver version, and publishes a GitHub release that the installer picks up automatically.

## Troubleshooting

- **`agc: command not found`**
  Ensure `~/.agent_composer/bin` is in your `PATH`.

- **`agc config` shows a database you didn't expect**
  The `source` field explains the choice — usually a forgotten `postgres` section in `~/.agent_composer/config/local.config.json`.

- **`apply_patch` not found**
  Re-run the installer so Codex is installed and on `PATH`.
