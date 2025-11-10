# agent-composer-sdk

Typed Python client for the Agent Composer REST API defined in `docs/openapi.yaml`. The code is generated with `openapi-python-client`, so it stays in sync with the service surface.

## Installation

```bash
pip install agent-composer-sdk           # once published to PyPI
pip install -e sdks/python/agent-composer-sdk  # from this repo
```

The SDK currently targets the unauthenticated API exposed at `http://localhost:8080/api` (see `servers` in the OpenAPI spec). Use the plain `Client` class—`AuthenticatedClient` is present for future-secured endpoints but is not required right now.

## Getting started

```python
from agent_composer_sdk import Client

client = Client(
    base_url="http://localhost:8080/api",
    raise_on_unexpected_status=True,
)
```

Every REST operation lives under `agent_composer_sdk.api.<tag>.<operation>`. Each operation exposes `sync`, `sync_detailed`, `asyncio`, and `asyncio_detailed` helpers that accept the shared client plus strongly typed request bodies or query params.

### Example: list agent specs

```python
from agent_composer_sdk import Client
from agent_composer_sdk.api.agent_specs import list_agent_specs

client = Client(base_url="http://localhost:8080/api")

specs = list_agent_specs.sync(client=client, limit=5)
if specs and specs.agent_specs:
    first = specs.agent_specs[0]
    print(f"{first.name} ({first.id}) uses {first.provider.value}/{first.model}")
```

The response type is `AgentSpecListResponse`, giving you pagination info plus strongly typed `AgentSpec` objects with fields such as `instructions`, `reasoning_effort`, and structured-output metadata.

### Example: start a conversation

```python
from agent_composer_sdk import Client
from agent_composer_sdk.api.agent_specs import list_agent_specs
from agent_composer_sdk.api.conversations import create_conversation
from agent_composer_sdk.models.create_conversation_request import CreateConversationRequest

client = Client(base_url="http://localhost:8080/api")

specs = list_agent_specs.sync(client=client)
spec_id = specs.agent_specs[0].id  # pick any spec you want to run

payload = CreateConversationRequest(
    agent_spec_id=spec_id,
    prompt="Summarize the latest build output.",
    parallel_conversations=1,
)

result = create_conversation.sync(client=client, body=payload)
conversation_id = result.conversations[0].id
print(f"Launched conversation {conversation_id}")
```

`CreateConversationRequest` mirrors the API schema—set the agent spec to run, the user prompt, and optional knobs such as `session_id`. The response includes the IDs of the conversations that were launched.

### Async usage

```python
import asyncio

from agent_composer_sdk import Client
from agent_composer_sdk.api.conversations import list_conversations

client = Client(base_url="http://localhost:8080/api")

async def main() -> None:
    async with client:
        conversations = await list_conversations.asyncio(client=client, limit=20)
        if conversations:
            for convo in conversations.conversations:
                print(convo.id, convo.status.value)

asyncio.run(main())
```

Entering the client as a context manager ensures the shared `httpx` session is opened/closed correctly for both sync and async paths.

## Working with other endpoints

- `agent_composer_sdk.api.agent_specs`: CRUD helpers for agent specs plus strong models such as `AgentSpec`, `CreateAgentSpecRequest`, and `UpdateAgentSpecRequest`.
- `agent_composer_sdk.api.conversations`: Start, resume, fork, list, and delete conversations. Models cover payloads like `CreateConversationRequest` and responses such as `ConversationListResponse`.
- `agent_composer_sdk.api.hooks`: Manage runtime hooks via `Hook`, `CreateHookRequest`, `UpdateHookRequest`, etc.
- `agent_composer_sdk.models`: All schema objects re-exported for direct construction/import.

Each operation returns either the documented model type or an `ErrorResponse`. Enable `raise_on_unexpected_status` on the client to surface 4xx/5xx responses as exceptions automatically.

## Advanced customization

The underlying transport is `httpx`. Pass `cookies`, `headers`, timeouts, TLS settings, or raw `httpx` keyword overrides via the `Client` constructor:

```python
from agent_composer_sdk import Client

client = Client(
    base_url="http://localhost:8080/api",
    headers={"x-agent-composer-experiment": "beta"},
    httpx_args={"event_hooks": {"response": [lambda r: print(r.status_code)]}},
)
```

You can also inject a fully constructed `httpx.Client`/`AsyncClient` via `set_httpx_client` or `set_async_httpx_client` when you need custom retry logic, proxies, or tracing.

## Building / publishing

The project uses [Poetry](https://python-poetry.org/) for packaging.

1. Update metadata in `pyproject.toml` (version, description, authors, etc.).
2. (Optional) Configure private indexes:
   ```bash
   poetry config repositories.<repo> <url>
   poetry config http-basic.<repo> <username> <password>
   ```
3. Build and publish:
   ```bash
   cd sdks/python/agent-composer-sdk
   poetry publish --build            # or --build -r <repo>
   ```

For local development installs without publishing:

```bash
poetry build -f wheel
pip install dist/agent_composer_sdk-<version>-py3-none-any.whl
# or: poetry add ../path/to/sdks/python/agent-composer-sdk
```
