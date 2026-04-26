# Agent Composer

**WARNING:** Early alpha. No sandboxing yet. If you enable shell access to an agent, assume the LLM has full file and network access.

Agent Composer is a vendor agnostic framework for building LLM agents.

(Currently only supports OpenAI models, more vendors coming soon)

## Documentation:

- **Rest Server**: https://vanclief.github.io/agent-composer

## Requirements

- **PostgreSQL** (requires previous installation)

- **Codex\*** (installed by the script)

\*GPT models often call an `apply-patch` tool that isn’t a native shell tool, but it’s bundled in Codex. So the installer adds Codex as a dependency.

## Installation

**Step 1: Install the binary**

```bash
curl -fsSL https://raw.githubusercontent.com/vanclief/agent-composer/master/install.sh | bash
```

**Step 2: Reload your shell or open a new terminal**

The installer adds `~/.agent_composer/bin` to your PATH.

Reload your shell or open a new terminal so it takes effect:

```
exec $SHELL -l
# or:
source ~/.zshrc   # zsh
source ~/.bashrc  # bash
```

You should then be able to run `agc`:

```
which agc
agc -h || agc --help
```

**Step 3: Create PostgreSQL user & DB**

```sql
CREATE ROLE agent_composer LOGIN;
CREATE DATABASE agent_composer OWNER agent_composer;
```

**Step 3: Set environment**

Create a `.env` file in the directory where you’ll run the server:

```dotenv
OPENAI_API_KEY="sk-xxxx" # Optional. Only needed for OpenAI-backed features.
```

`ENVIRONMENT` and `POSTGRES_PASSWORD` are currently hardcoded in the app for local development.

Load it:

```bash
set -a; source .env; set +a
```

**Step 4 (optional): Config file**

You can place a config file at:

```
$HOME/.agent_composer/config
```

See `core/config/local.config.json` for an example.

## Usage

**Terminal UI**

```bash
agc
```

**REST server**

```bash
agc rest
```

**Run a workflow**

```bash
agc workflow run \
  --id binary_vote_round \
  --input-json '{"question":"Are cats blue?"}'
```

Workflow commands live under `agc workflow`, with `agc wf` and `agc w` as aliases.

For workflows with exactly one top-level `string` input, you can use:

```bash
agc workflow run \
  --id binary_vote_round \
  --input-string "Are cats blue?"
```

**Compile a workflow without running it**

```bash
agc workflow compile --file ./examples/binary_vote_round.yaml
```

**List installed workflows**

```bash
agc workflow list
```

**Show a workflow's raw YAML**

```bash
agc workflow show --id binary_vote_round
```

**Import a workflow into the registry**

```bash
agc workflow import --file ./examples/binary_vote_round.yaml
```

**Export a workflow from the registry**

```bash
agc workflow export --id binary_vote_round --file ./binary_vote_round.yaml
```

**Delete a workflow from the registry**

```bash
agc workflow delete --id binary_vote_round
```

**Start the MCP server**

```bash
agc mcp
```

The AGC MCP server exposes:

- `agc_workflow_list`
- `agc_workflow_start`
- `agc_workflow_get`

`agc_workflow_start` returns immediately with an execution id. Use `agc_workflow_get` to poll for `running`, `succeeded`, or `failed`.

Imported workflows are copied into the registry one file at a time. If a workflow composes other workflows by `workflow_id`, those dependencies are not imported automatically.

## Updating

Re-run the install command from Installation.

## Troubleshooting

- **`agc: command not found`**
  Ensure `~/.agent_composer/bin` is in your `PATH`.

- **PostgreSQL connection errors**
  Confirm the role/DB exist and your `.env` values match your setup.

- **`apply_patch` not found**
  Re-run the installer so Codex is installed and on `PATH`.
