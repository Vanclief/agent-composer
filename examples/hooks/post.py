import sys
import json
from pathlib import Path


def main() -> None:
    try:
        event = json.load(sys.stdin)  # read JSON from STDIN
    except json.JSONDecodeError as exc:
        print(f"invalid JSON on stdin: {exc}", file=sys.stderr)
        sys.exit(2)

    # Minimal validation (strict and explicit)
    required = ["id", "type", "conversation_id", "agent_name", "timestamp", "data"]
    missing = [k for k in required if k not in event]
    if missing:
        print(f"missing fields: {missing}", file=sys.stderr)
        sys.exit(3)

    # Use the event
    agent_name = event["agent_name"]
    evt_type = event["type"]
    data = event["data"]
    print(f"Post tool received event: {evt_type} for {agent_name} (data {data})")


if __name__ == "__main__":
    main()
