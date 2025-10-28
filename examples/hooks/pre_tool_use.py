import sys
import json
from pathlib import Path


def main() -> None:
    try:
        event = json.load(sys.stdin)  # read JSON from STDIN
    except json.JSONDecodeError as exc:
        print(f"invalid JSON on stdin: {exc}", file=sys.stderr)
        sys.exit(2)

    hook_input = {
        "id": event["id"],
        "conversation_id": event["conversation_id"],
        "event_type": event["event_type"],
        "agent_name": event["agent_name"],
        "last_response": event.get("last_response"),
        "tool_name": event.get("tool_name"),
        "tool_arguments": event.get("tool_arguments"),
    }

    print(hook_input)

    # tool_arguments = event.get("tool_arguments", "")
    # if "git" in tool_arguments.lower():
    #     print(
    #         "You are not allowed to use git",
    #         file=sys.stderr,
    #     )
    #     sys.exit(2)


if __name__ == "__main__":
    main()
