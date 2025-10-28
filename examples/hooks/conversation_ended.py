import sys
import json


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

    # if hook_input["last_response"] != "goodbye":
    #     print(
    #         "Conversation did not end as expected, you need to say exactly the word 'goodbye'",
    #         file=sys.stderr,
    #     )
    #     sys.exit(2)


if __name__ == "__main__":
    main()
