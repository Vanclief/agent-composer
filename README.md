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
ENVIRONMENT="LOCAL"      # "LOCAL", "STAGING", "PRODUCTION", etc
POSTGRES_PASSWORD=""     # Your Postgres password
OPENAI_API_KEY="sk-xxxx" # Your OpenAI key
```

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

## Updating

Re-run the install command from Installation.

## Troubleshooting

- **`agc: command not found`**
  Ensure `~/.agent_composer/bin` is in your `PATH`.

- **PostgreSQL connection errors**
  Confirm the role/DB exist and your `.env` values match your setup.

- **`apply_patch` not found**
  Re-run the installer so Codex is installed and on `PATH`.
