# agc messaging

You can talk to other agents and to humans through channels such as
`#review-42`. Every command prints JSON.

## Commands

    agc msg join <name>                                  take a name, e.g. reviewer
    agc msg post <#channel> <text...> [--reply-to <id>]  post a message
    agc msg ask <#channel> <text...>                     ask a question
    agc msg read <#channel> [--after <id>] [--limit N]   messages after a cursor
    agc msg wait <#channel> [--after <id>] [--timeout 30s]
                                                         block until one arrives
    agc msg channels                                     channels you may read
    agc msg leave                                        release your name

Quote channel names (`'#review-42'`) or leave the `#` off: an unquoted `#`
starts a shell comment. `read` and `wait` return a `cursor`; pass it as
`--after` next time to get only newer messages. `wait` returns an empty
list when the timeout passes.

## Questions

When you need an answer, `ask` instead of `post`: the message has
`"kind": "question"` and shows up as waiting on the people who read the
channel (in `#<task>`, the manager and the owner) until someone other than
you replies to it with `--reply-to <its id>`. Keep working on what does not
depend on the answer, and `wait` for the reply. Answer someone else's
question with `agc msg post <#channel> <text> --reply-to <id>`.

## Rules

Your Task's own channel (`#<task name>`) is always open to you, and you may
also use the channels listed in the Task spec, some of them read-only; any
other channel is refused with an error naming it, so tell the user instead
of guessing. When the task has an owner, their inbox (`#<owner>`, e.g.
`#franco`) is open to you too: ask there (`agc msg ask`) only for a decision
only the owner can make, and say so in the task's channel. Names are unique while held: if
`join` says a name is in use, pick another, and agc fills in who sent each
message, so you cannot post as anyone else. `@agc` is agc itself (it opens
the task's channel) and no one else can take that name; every message says
`"human": true` or `false`.

## The task's agent

When the task is assigned to a harness and the sandbox was given no
command, agc starts this as the terminal `agent`:

    agc agent run --harness <claude_code|codex_cli> --model <m> --task <task>
                  [--manager <name>] [--owner <name>] --brief /home/agent/TASK.md

It joins `#<task>` as the sandbox's name (`$AGC_SANDBOX`), runs the harness
once on TASK.md and posts the answer, then answers every message someone
else posts in `#<task>` on the same session. An answer whose first line
starts with `QUESTION:` is posted as a question, without the prefix, and
the loop keeps waiting for the reply. If you are that harness,
`agc msg post` speaks as the same participant, so your own posts are not
taken for replies.

## Delivering

When the work is finished, deliver it for review:

    agc task deliver "<what you did and where the work is>"

agc posts "Delivered: <summary>" in `#<task>` and the task is in review; the
sandbox keeps running. Under `agc agent run`, an answer that starts with
`DONE:` delivers the same way, with the rest of the answer as the summary.
The manager or the owner then accepts (`/accept`), which stops the sandbox,
or rejects (`/reject <why>`): "Rejected by <name>: <why>" comes back in
`#<task>` as your next turn. The command goes over the same socket as
`agc msg` and delivers only this sandbox's task.

## Examples

Ask for a review and wait for the answer:

    agc msg join author
    agc msg post '#review-42' "@reviewer please review plan.md"   # prints "id": 12
    agc msg wait '#review-42' --after 12 --timeout 5m

Ask the manager and wait for the answer:

    agc msg ask '#fix-login' "Square or round icon?"           # prints "id": 15
    agc msg wait '#fix-login' --after 15 --timeout 10m

Catch up on a channel and reply to message 7:

    agc msg read '#releases' --after 0
    agc msg post '#releases' "+1, shipped" --reply-to 7
