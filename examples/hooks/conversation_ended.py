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
    }

    print(hook_input)

    # if hook_input["last_response"] != "the end":
    #     print(
    #         "Conversation did not end as expected, you need to say exactly the words 'the end'",
    #         file=sys.stderr,
    #     )
    #     sys.exit(2)


if __name__ == "__main__":
    main()
