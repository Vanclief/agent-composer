# ADR 0015: The main agent routes, dispatches under gates and never does the work

**Status:** Proposed — 2026-11-02. Builds on [ADR 0005](0005-messaging-transport-and-identity.md), [ADR 0006](0006-rules-and-beliefs.md), [ADR 0007](0007-proposals-and-gates.md), [ADR 0009](0009-task-agents-and-managers.md), [ADR 0010](0010-task-acceptance.md) and [ADR 0016](0016-persistent-agents.md), which gives `main` its record, its home, its wake and its one long session.

## Context

ADR 0016 made `main` a role: an agent whose home wakes when a message lands
in any channel, keeps one harness session, and may read and post anywhere.
It left two things to this ADR: how the batch of new messages becomes a
prompt, and what `main` may do with it. Franco wants someone who reads
everything so he does not have to, sends work to the right task, makes new
tasks when work has no home, and asks him before it acts, until he says it
need not. Two of our beliefs set the shape: a human who cannot follow why
something was done stops saying no to it
(`authority-without-legibility-is-ritual`), and a digest nobody reads is how
a human quietly leaves the loop (`unread-digests-remove-the-human`). An
agent reading every channel is exactly the digest; the failure mode is
`main` doing things in Franco's name that he never sees.

## Decision

1. **The batch is inline and spill.** Every turn of `main` gets the messages
   the wake loop hands it (ADR 0016 item 6) as one section: per message its
   id, channel, sender (marked `(human)` when a person wrote it), kind, the
   message it replies to, and the first 300 characters of its body, with
   `agc msg read '<channel>' --after <id-1> --limit 1` for the rest
   (`BatchPrompt`, `interfaces/cli/agent_home.go`). A message `main` posted,
   as its loop's participant, under its name, or from its home through the
   harness's `agc msg post` (`agent` is its home), is never a turn. agc's
   own posts are turns, since triggers (ADR 0014) land as agc's messages in
   `#<task>` or `#main`, except the sleep notice (`Asleep.`). The loop posts
   nothing for `main`: its final answer stays in the transcript as the
   trace, and it says what it has to say with `agc msg`. A batch that needs
   nothing ends with no post, and that is the normal case.

2. **It never works; it only talks and dispatches.** The harness runs in an
   empty working directory (`/home/agent/empty`, fixed because Claude Code
   keys a session to its directory) with `--tools Bash`, `--allowedTools`
   holding only `agc msg *`, `agc task *` and `agc
   beliefs|decisions|rules|actions|proposals|gates show|why`,
   `--permission-mode dontAsk` so anything off the list is refused rather
   than prompted, every file, agent and web tool named in
   `--disallowedTools`, `--strict-mcp-config` and
   `--disable-slash-commands`; never `--dangerously-skip-permissions`
   (`mainClaude`, `interfaces/cli/agent_main.go`; the test is on the
   arguments and on the directory a fake `claude` runs in). The loop refuses
   to start a turn in a working directory that is not empty. Only Claude
   Code can be held to a list of commands, so `main` refuses any other
   harness at start. `agc task create -f -` reads the task yaml from stdin,
   so `main` creates a task from the exact yaml it asked about without a
   file.

3. **Its task acts are gated (ADR 0007).** Creating, accepting and rejecting
   a task are the proposal kind `task` (`knowledge.Proposable`, an act: one
   resting on nothing is illegible and decide). A gate may say `proposes:
   task, by: [main]`. Every turn the loop pulls the knowledge clone, runs
   `agc gates show --ids`, and computes the tier of `task by main` now
   (`Set.TierFor`: the gate's tier, pulled to decide by a rejection under
   it, nothing sampled since there is no proposal id), and the prompt says
   both. At **decide**, or with no gate, `main` asks in `#<owner>` (kind
   question, `agc msg ask`) with the exact task yaml or the reason, and acts
   only on a yes from a human replying to that question; at **observe** it
   acts and says what it did in `#<owner>`; at **delegated** it acts
   quietly. Agents never edit `gates/` (rule `agents-never-edit-gates`): the
   gates Franco may add are proposals in `proposals/main-agent.yaml`. Until
   he adds one, everything is decide, which is correct.

4. **Policies are rules, not prompt text.** What `main` should do is
   `rules/main.yaml`: stay quiet unless a message asks for something or a
   trigger concerns an open task; never do the work; one task per piece of
   work, none without a brief and accept criteria; act by the task gate;
   never answer for the owner; escalate in `#<owner>`. Each is a judged rule
   trusting `main` and `franco`, so `agc rules judge --as main` asks the
   role's harness (claude_code) and Franco judges the transcript. `main` is
   accepted wherever harness ids are accepted in `trusted`, and like a
   harness it is never a witness: evidence `who: main` is refused. The
   prompt assembled in Go (`MainPrompt`) is only: who it is and its tools
   (the first turn of a session, with AGENT.md), the rules it is trusted
   with (`agc rules show --trusted main --ids`), the gates and the tier, and the
   batch.

5. **The knowledge is a clone in the home.** `agents/main.yaml` clones the
   repo at `master` and sets `AGC_KNOWLEDGE_DIR` to the clone. The knowledge
   commands fall back to `$AGC_KNOWLEDGE_DIR` when the current directory
   holds none of `evidence/ beliefs/ decisions/ rules/ actions/ proposals/
   gates/`, so the harness's `agc gates show` works from the empty
   directory. The loop runs `git pull --ff-only` there before every turn: a
   gate merged to master counts at the next batch.

## Changes to ADR 0016's runner and hook

The smallest that could host the above, all named here:

- `homeLoop` gets a `main` mode (`mainMode`): for role `main`, `isMainTurn`
  instead of `isTurn`, `takeMain` instead of `take`, and the restricted
  harness instead of the template's plain one. Other roles are unchanged.
- `GET /agent`'s client view reads `home`, which the server already sent,
  so the loop knows the `agent` its harness's posts carry.
- The wake hook woke nobody on agc's own posts; it now wakes role `main` on
  them, never another role, and never on the sleep notice, which would
  otherwise wake `main` every time it fell asleep.
- `agc task create` passed no `accept:` from the file: fixed, since a task
  `main` creates must carry its criteria.

## Rationale

- Inline and spill because a batch across every channel can be large; 300
  characters is enough to route most messages, and the id and channel make
  the rest one command away, so the prompt stays small without hiding
  anything.
- No post by default because a router that answers every batch is the
  unread digest; silence is the cheap, correct answer to most batches, and
  the transcript keeps the reasoning anyway (ADR 0016 item 5).
- Tools cut to agc commands because "never does the work" is a property we
  can enforce, not only ask for: without files, a shell or the web there is
  no work to do, and nothing the agent reads in a channel can make it
  fetch, write or run anything else.
- The tier computed in Go as well as shown because the gates are what a
  human wrote; misreading them would put an act outside them, and one line
  saying the tier costs nothing.
- Rules not prompt text because a rule is reviewed in git, cited by id,
  judged, and changes without a code change; a prompt paragraph is none of
  these.

## Alternatives considered

- **Enforce the tier in agc**, refusing a home's `POST /tasks` at decide
  unless it cites a question a human answered yes. Stronger, and left open
  (`home-task-routes-enforce-the-gate`): it needs a way to tie an act to a
  question and a yes, which messaging does not model yet.
- **Codex for main** with `--sandbox read-only`: refused, since codex has no
  per-command allow list and `agc msg post` needs the socket.
- **One turn per thread for main**, as other roles: rejected by ADR 0016
  item 7; routing needs to see across channels.
- **Posting main's answer as a reply**, as other roles: rejected, see
  Rationale.

## Consequences

- New: `interfaces/cli/agent_main.go` and its tests, `knowledge` kind `task`
  and `Set.TierFor`, `main` as a trusted judge (`IsAgentRole`,
  `JudgeHarness`), `agc rules show --trusted`, the `AGC_KNOWLEDGE_DIR`
  fallback, `agc task create -f -`, `rules/main.yaml`,
  `decisions/main-agent.yaml`, `actions/main-agent.yaml`,
  `proposals/main-agent.yaml`, and the rewritten `agents/main.yaml`.
- Every agc post now wakes an asleep `main` (task openings, deliveries,
  acceptances, triggers): a boot and a quiet turn each. Cheap at today's
  volume; a filter belongs in the hook if it is not.
- Starting it on the rig: `agc agent create main` (once), then a message in
  any channel wakes it, or `agc agent wake main-1`. The home clones the repo,
  so the rules and gates it follows are master's.

## Left open

- `watchdog-covers-the-main-agent`: nothing watches `main` itself; a failed
  home or a loop that exits without sleeping goes unnoticed.
- `main-memory-beyond-the-session`: its memory is its one session and what
  it reads again; after a compaction it forgets what it asked and who said
  yes, unless it reads the channel again.
- `home-task-routes-enforce-the-gate`, above.
- Direct messages `@<human>/<task>` (ADR 0011) are read like any channel if
  merged; nothing here depends on them.
