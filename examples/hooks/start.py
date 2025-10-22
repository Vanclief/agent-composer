import sys
import json


def main() -> None:
    try:
        event = json.load(sys.stdin)  # read JSON from STDIN
    except json.JSONDecodeError as exc:
        print(f"invalid JSON on stdin: {exc}", file=sys.stderr)
        sys.exit(2)

    print(event)


if __name__ == "__main__":
    main()
