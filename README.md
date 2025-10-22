# Agent Composer

A vendor agnostic framework for building LLM agents.

## Requirements

- **PostgreSQL** (running locally)

## Installation

**Step 1: Install the binary**

```bash
curl -L https://raw.githubusercontent.com/vanclief/agent-composer/master/install.sh | bash
```

**Step 2: Create PostgreSQL User & DB**

```sql
CREATE ROLE agent_composer LOGIN;

CREATE DATABASE agent_composer OWNER agent_composer;
```

**Step 3: Setup the Env vars**

Create `.env` file where server will be run

```dotenv
ENVIRONMENT="LOCAL"
POSTGRES_PASSWORD=""     # Replace if different
OPENAI_API_KEY="sk-xxxx" # Your OpenAI key
```

Then export the environment:

`eval $(cat .env | sed "s/^/export /")`

**Step 4: Run the binary**

```bash
./agc
```
