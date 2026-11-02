# agc: YC application, first draft

Drafted by Claude Code for Franco Valencia. Written in Franco's voice, so check every sentence is true before it
is sent. Nothing here has been submitted, and no account was created or used.

## What I read, and where (all read only, 2026-11-02)

| Fact | Source |
|---|---|
| Open batch is Winter 2027, January to March, in person in San Francisco. | https://www.ycombinator.com/apply |
| **On-time deadline: November 2 at 8pm PT. That is today, the day this was drafted.** Applying before it gets a decision by December 11; later applications are still considered with no date promised. | https://www.ycombinator.com/apply |
| Interviews mostly in November and December, by video. YC invests as soon as a company is accepted. | https://www.ycombinator.com/apply |
| Early decision lets you apply to Spring, Summer and Fall batches now. | https://www.ycombinator.com/apply |
| Standard deal: $500,000. $125,000 on a post-money SAFE for 7%, plus $375,000 on an uncapped MFN SAFE. Pro rata right. No fees. | https://www.ycombinator.com/deal |
| YC invests in US, Canada, Cayman and Singapore corporations; if not incorporated, they help after acceptance. | https://www.ycombinator.com/deal |
| Solo founders are accepted, but YC advises a co-founder. | https://www.ycombinator.com/faq |

**Not read: the application questions.** The form is at https://apply.ycombinator.com/home behind a login, and the
rules for this task forbid logging in. The public pages do not list the questions. The section below follows the
questions YC has long asked, **from memory, unverified**. Franco must paste the real current questions over these
before sending.

**Nothing was written under `evidence/`.** I am not a witness (rule `evidence-who-is-never-a-model`). Franco can add
the facts above as evidence once he has seen the pages himself.

## Requests for Startups: which ones fit

Source: https://www.ycombinator.com/rfs (Winter 2027). YC says these are a fraction of what it funds and that you do
not need to match one. An honest read is that agc fits none cleanly.

- **Own Your Intelligence**: closest. It asks for someone to "turn workflows into evals, expertise into training data,
  production failures into improvements". agc keeps expertise as evidence-backed beliefs and rules a company owns in
  plain files. It is a stretch: agc does not train or host models.
- **AI for the Air-Gapped Economy**: partial. agc is local-first (SQLite, files in git, no external service) and runs
  agents in local microsandbox VMs. We have not built or tested anything for air-gapped deployment.
- **The $15-an-Hour Expert**: weak. agc is aimed at the person who has to judge agent output, which is related, but
  I have no evidence of a market in those jobs.

Not a fit: space, robots, LLMs that write well, government benefits, physical-buildout finance, vertical foundation
models, City Hall, data center supply chain.

## One sentence

agc keeps what you believe tied to evidence a person saw, so AI agents can propose changes but only gates you set
decide what lands.

## One paragraph

AI agents now produce more than a person can check, and the usual fix is to let a model summarize what it found and
file it as knowledge. That is how a company ends up believing things that nothing ever witnessed. agc is a
git-and-YAML layer with a CLI and REST API. Rules say what must not be done. Evidence records what a person or a
mechanical check saw, with who and when, and the tool refuses a model as the witness. Beliefs cite that evidence, so
each one is computed as grounded, an opinion, contested or in doubt. Decisions and actions rest on beliefs, so when
evidence is retracted you see everything that falls with it. Agents can only submit proposals, and gates say which
land without a human and which don't, with rejection rates and overrides kept as data. One repository, this one, runs
on it today.

## The thesis

An LLM is a great parser and a terrible sensor. It reads a messy document and produces clean structure very well. It
does not observe anything. If a model's reading is allowed to become an evidence record, beliefs will look grounded
while resting on nothing. So the product is the layer that keeps a human's beliefs grounded in evidence while agents
propose and gates decide what lands. The model's job is to parse and to propose; the source of truth is always a
person or a mechanical check. A model that read a real source is recorded as `translated_by`, with the source named;
a model's own conclusion is an opinion. This is ADR 0006 (rules and beliefs) and ADR 0007 (proposals and gates).

The risk we design against is the human who keeps approving what they can no longer judge. So approval needs
legibility (the reasons are visible), sources the system did not produce, a watched rejection rate, and recorded
overrides.

## Application answers (questions unverified, see above)

**Describe what your company does in 50 characters or less.**
Keep beliefs grounded while agents propose. (42)

**Founders: who writes code, who does what, how long have you known each other?**
I'm Franco Valencia, the only founder so far. I wrote almost all of agc: 248 of the 249 commits in the repository
(the other is Claude's), going back to October 2025. I write the code. [OPEN QUESTION 1: no co-founder is named
here, and I have not made one up.]

**Please tell us about the time you most successfully hacked some (non-computer) system.**
[OPEN QUESTION 6: this is yours alone. Nothing in the repository answers it, so there is no draft.]

**Please tell us in one or two sentences about the most impressive thing other than this startup that each founder has
built or achieved.**
[OPEN QUESTION 6: yours to write.]

**Tell us about things you've built before. Include URLs.**
https://github.com/vanclief/agent-composer is this project. [Add the rest yourself.]

**What is your company going to make?**
agc, a command-line tool and REST API that store a team's rules, evidence, beliefs, decisions and actions as plain
YAML in git, and compute from them what is grounded, what is only an opinion, what is contested, and what falls if a
piece of evidence is retracted. Agents submit proposals instead of changing things; gates decide which proposals land
without a human. It also runs tasks for agents in microsandbox VMs, because the agents that propose need somewhere to
work.

**Where do you live now, and where would the company be based after YC?**
[OPEN QUESTION: yours. YC's batch is in person in San Francisco.]

**Explain your decision regarding location.**
[OPEN QUESTION: yours.]

**How far along are you?**
A working Go CLI and REST API. One repository, agc's own, uses it on itself: its rules, evidence, beliefs,
decisions, actions, proposals and gates live in files that agc loads and checks, and CI runs `agc rules check`.
Zero users outside that repository. Zero revenue. No customers, no waitlist, no pilots. One person.

**How long have each of you been working on this? Full-time?**
The first commit is from 2025-10-22. The knowledge system (ADR 0006 and 0007) was written on 2026-11-01. The earlier
commits are a workflow runner for coding agents, which this grew out of. [OPEN QUESTION: whether it has been
full-time; the git log does not say.]

**What tech stack are you using?**
Go, SQLite by default (Postgres supported), bun for models, echo for REST, plain YAML in git for the knowledge
files, microsandbox VMs for tasks, a React panel for the control surface. Agents run through existing coding-agent
harnesses (Claude Code, Codex).

**Are people using your product?**
Only me and the agents working in this repository. That's one user.

**Do you have revenue?**
No.

**If you are applying with the same idea as a previous batch, did anything change?**
[OPEN QUESTION: whether you have applied before.]

**Why did you pick this idea? Do you have domain expertise? How do you know people need what you're making?**
I picked it because I hit the problem myself: I run coding agents on my own repository and found I could not say what
I believed about it, or why, in a form CI could check. I don't have evidence that anyone else needs this yet. That
is the biggest unknown, and the first thing I would do is find out. [Edit this to what actually happened to you.]

**Who are your competitors? What do you understand that they don't?**
Agent-memory and company-brain startups (for example Hyper, Memory Store, Mosaic, Hyperspell), agent control planes
(Omnara, Replicas) and observability tools; details in `competitors.md`, gathered from search snippets, not read in
full. What I think they get wrong is the source. Most extract memory from conversations with a model and treat the
result as knowledge. agc says a model can parse and propose but cannot be the witness, so a belief is only as strong
as the weakest evidence under it. I may be wrong that anyone will pay for that distinction.

**How do or will you make money? How much could you make?**
[OPEN QUESTION 3: yours. I have no pricing, no buyer, and no number. Any figure written here would be invented.]

**Which category best describes your company?**
Developer tools (AI infrastructure). [Confirm against YC's list.]

**Have you formed a legal entity yet?**
No evidence in the repository that one exists. [OPEN QUESTION 2: incorporation. YC invests in US, Canada, Cayman and
Singapore entities and helps incorporate after acceptance.]

**Have you taken funding or made any equity agreements?** [OPEN QUESTION: yours.]

**Do you have any other commitments for the next year (job, school)?** [OPEN QUESTION: yours.]

**If you had any other ideas you considered applying with, please list them.** [OPEN QUESTION: yours.]

**Please tell us about an interesting project, preferably outside of class or work, that two or more of you created
together.** [OPEN QUESTION 1: needs a co-founder; skip if solo.]

**Founder video (about a minute).** [Franco records this. A suggested spine: the thesis paragraph, the exact
progress line (one repo, one user, no revenue), and what you want to learn in the batch.]

## Open questions for Franco

These are yours to answer. Nothing above fills them in.

1. **Co-founder.** Is there one? YC accepts solo founders but advises against it, and it asks how long founders have
   known each other. Do you want to use YC Co-founder Matching?
2. **Incorporation.** No entity is evident. US, Canada, Cayman or Singapore, and where do you live?
3. **How it makes money.** Who pays, for what, and how much? Options visible in the repo are an open-source CLI with a
   hosted control panel, per-seat team plans, or enterprise on-prem (the RFS "air-gapped" angle), but I have no
   evidence for any of them.
4. **What you want to be true in two years.** Draft only a version of the answer you would stand behind.
5. **Timing.** The on-time deadline is tonight at 8pm PT. Early Decision for a later batch is on the apply page if
   tonight is too soon.
6. **Personal answers.** The non-computer hack, your most impressive other achievement, and the founder video.
7. **Evidence for the pitch.** Do you have even one outside person who has used agc? If not, the draft says so, and
   you may want to find one before interview day.
8. **Questions.** Paste the real current application questions so the answers can be matched to them.
