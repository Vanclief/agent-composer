# agc messaging

You can talk to other agents and to humans through channels such as
`#review-42`. Every command prints JSON.

## Commands

    agc msg join <name>                                  take a name, e.g. reviewer
    agc msg post <#channel> <text...> [--reply-to <id>]  post a message
    agc msg read <#channel> [--after <id>] [--limit N]   messages after a cursor
    agc msg wait <#channel> [--after <id>] [--timeout 30s]
                                                         block until one arrives
    agc msg channels                                     channels you may read
    agc msg leave                                        release your name

Quote channel names (`'#review-42'`) or leave the `#` off: an unquoted `#`
starts a shell comment. `read` and `wait` return a `cursor`; pass it as
`--after` next time to get only newer messages. `wait` returns an empty
list when the timeout passes.

## Rules

Your Task's own channel (`#<task name>`) is always open to you, and you may
also use the channels listed in the Task spec, some of them read-only; any
other channel is refused with an error naming it, so tell the user instead
of guessing. Names are unique while held: if `join` says a name is in use,
pick another, and agc fills in who sent each message, so you cannot post as
anyone else.

## Examples

Ask for a review and wait for the answer:

    agc msg join author
    agc msg post '#review-42' "@reviewer please review plan.md"   # prints "id": 12
    agc msg wait '#review-42' --after 12 --timeout 5m

Catch up on a channel and reply to message 7:

    agc msg read '#releases' --after 0
    agc msg post '#releases' "+1, shipped" --reply-to 7
