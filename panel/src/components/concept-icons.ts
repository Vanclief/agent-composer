// One Lucide icon per concept, used in every list, card, detail and form
// (rtsdesign rule 2). Import from here; never pick a different icon for
// one of these concepts. Need a new concept? Add it here first.
import {
  Bookmark,
  Box,
  CodeXml,
  Container,
  Cpu,
  FolderGit2,
  GitBranch,
  HardDrive,
  Hash,
  KeyRound,
  MemoryStick,
  MessagesSquare,
  ScrollText,
  SquareTerminal,
  User,
  Variable,
  type LucideIcon,
} from 'lucide-react'
import { Brain, Eye, FileText, Hammer, Library, Lightbulb, Scale, Signpost } from 'lucide-react'
import { ListTodo } from 'lucide-react'
import { Bot } from 'lucide-react'
import { Bell } from 'lucide-react'
import { Inbox, ListChecks, PackageCheck } from 'lucide-react'

export const conceptIcons = {
  // A Task: a VM agents work in.
  task: Container,
  cpu: Cpu,
  memory: MemoryStick,
  // OCI image the VM boots.
  image: Box,
  repo: FolderGit2,
  // Branch, tag or commit.
  ref: GitBranch,
  // The setup script run after the clone.
  setup: ScrollText,
  secret: KeyRound,
  env: Variable,
  volume: HardDrive,
  channel: Hash,
  savedSpec: Bookmark,
  terminal: SquareTerminal,
  message: MessagesSquare,
  human: User,
  // The raw YAML/JSON view behind a </> toggle.
  raw: CodeXml,
  // The two top-level modes: Think (knowledge) and Work (tasks, messages).
  think: Brain,
  work: Hammer,
  // ADR 0006: something we hold true, a choice made on beliefs, a must or
  // must not, and what was seen.
  belief: Lightbulb,
  decision: Signpost,
  rule: Scale,
  evidence: Eye,
  // The YAML file an item lives in.
  file: FileText,
  // A ruleset's subject, e.g. "the Linux rig".
  subject: Library,
  // Something to do because of a decision (ADR 0006).
  action: ListTodo,
  // ADR 0008: the compute a task is allocated, one microsandbox VM.
  sandbox: Box,
  // A task's brief: what to do and what done looks like.
  brief: ScrollText,
  // ADR 0009: a harness that works a task (claude_code, codex_cli, pi).
  agent: Bot,
  // A task's notify flag: toast and sound when it starts and is done.
  notify: Bell,
  // ADR 0010: a task's acceptance criteria ("Accepted when"), and the
  // agent's delivery of the work for review.
  criteria: ListChecks,
  delivery: PackageCheck,
  // ADR 0009 item 5: a person's own channel, #<name>, where agents and
  // tasks escalate.
  inbox: Inbox,
} satisfies Record<string, LucideIcon>

export type Concept = keyof typeof conceptIcons

// One hue per concept, used with its icon in every list, card, detail and
// form (rtsdesign rule 3: color codes concepts, saturation codes attention).
// Families, one hue each:
//   violet  machine: task, sandbox, image, cpu, memory
//   blue    code: repo, ref, setup
//   yellow  secret (warm, a warning)
//   lime    env
//   teal    volume
//   cyan    terminal
//   pink    messaging: channel, message, human
//   orange  savedSpec
//   zinc    raw (a view, not a resource: neutral), brief (words)
// Statuses own emerald (running), zinc (stopped), sky (transitions) and red
// (failed); no concept uses those, so a status always stands out.
// Saturation is attention: a selected chip is a strong tint, and only the
// screen's one primary button gets the solid fill.
export type ConceptColor = {
  // Icon and label, saturated. Light text is -700 for AA on white.
  text: string
  // Soft tint for chips and card headers.
  bg: string
  // Borders are opaque shades with their own dark shade: a translucent
  // border over a translucent tint fringes yellow at rounded corners for
  // violet and blue in Chromium.
  border: string
  // A card's 2px top edge.
  edge: string
  // Selected look of a plain element (tile, chip): strong tint.
  selected: string
  // The same selected look for a Toggle or ToggleGroupItem, via data-state.
  on: string
  // The primary button: solid fill, the most saturated thing on screen.
  solid: string
}

const violet: ConceptColor = {
  text: 'text-violet-700 dark:text-violet-400',
  bg: 'bg-violet-500/10',
  border: 'border-violet-200 dark:border-violet-900',
  edge: 'border-t-2 border-t-violet-400 dark:border-t-violet-500',
  selected:
    'border-violet-400 bg-violet-500/20 text-violet-800 hover:bg-violet-500/30 hover:text-violet-800 dark:border-violet-600 dark:text-violet-200 dark:hover:text-violet-200',
  on: 'data-[state=on]:border-violet-400 data-[state=on]:bg-violet-500/20 data-[state=on]:text-violet-800 data-[state=on]:hover:bg-violet-500/30 data-[state=on]:hover:text-violet-800 dark:data-[state=on]:border-violet-600 dark:data-[state=on]:text-violet-200 dark:data-[state=on]:hover:text-violet-200',
  solid: 'bg-violet-600 text-white shadow-md shadow-violet-500/25 hover:bg-violet-600/90',
}
const blue: ConceptColor = {
  text: 'text-blue-700 dark:text-blue-400',
  bg: 'bg-blue-500/10',
  border: 'border-blue-200 dark:border-blue-900',
  edge: 'border-t-2 border-t-blue-400 dark:border-t-blue-500',
  selected:
    'border-blue-400 bg-blue-500/20 text-blue-800 hover:bg-blue-500/30 hover:text-blue-800 dark:border-blue-600 dark:text-blue-200 dark:hover:text-blue-200',
  on: 'data-[state=on]:border-blue-400 data-[state=on]:bg-blue-500/20 data-[state=on]:text-blue-800 data-[state=on]:hover:bg-blue-500/30 data-[state=on]:hover:text-blue-800 dark:data-[state=on]:border-blue-600 dark:data-[state=on]:text-blue-200 dark:data-[state=on]:hover:text-blue-200',
  solid: 'bg-blue-600 text-white shadow-md shadow-blue-500/25 hover:bg-blue-600/90',
}
const yellow: ConceptColor = {
  text: 'text-yellow-700 dark:text-yellow-400',
  bg: 'bg-yellow-500/10',
  border: 'border-yellow-200 dark:border-yellow-900',
  edge: 'border-t-2 border-t-yellow-400 dark:border-t-yellow-500',
  selected:
    'border-yellow-400 bg-yellow-500/20 text-yellow-800 hover:bg-yellow-500/30 hover:text-yellow-800 dark:border-yellow-600 dark:text-yellow-200 dark:hover:text-yellow-200',
  on: 'data-[state=on]:border-yellow-400 data-[state=on]:bg-yellow-500/20 data-[state=on]:text-yellow-800 data-[state=on]:hover:bg-yellow-500/30 data-[state=on]:hover:text-yellow-800 dark:data-[state=on]:border-yellow-600 dark:data-[state=on]:text-yellow-200 dark:data-[state=on]:hover:text-yellow-200',
  solid: 'bg-yellow-500 text-zinc-950 shadow-md shadow-yellow-500/25 hover:bg-yellow-500/90',
}
const lime: ConceptColor = {
  text: 'text-lime-700 dark:text-lime-400',
  bg: 'bg-lime-500/10',
  border: 'border-lime-200 dark:border-lime-900',
  edge: 'border-t-2 border-t-lime-400 dark:border-t-lime-500',
  selected:
    'border-lime-400 bg-lime-500/20 text-lime-800 hover:bg-lime-500/30 hover:text-lime-800 dark:border-lime-600 dark:text-lime-200 dark:hover:text-lime-200',
  on: 'data-[state=on]:border-lime-400 data-[state=on]:bg-lime-500/20 data-[state=on]:text-lime-800 data-[state=on]:hover:bg-lime-500/30 data-[state=on]:hover:text-lime-800 dark:data-[state=on]:border-lime-600 dark:data-[state=on]:text-lime-200 dark:data-[state=on]:hover:text-lime-200',
  solid: 'bg-lime-500 text-zinc-950 shadow-md shadow-lime-500/25 hover:bg-lime-500/90',
}
const teal: ConceptColor = {
  text: 'text-teal-700 dark:text-teal-400',
  bg: 'bg-teal-500/10',
  border: 'border-teal-200 dark:border-teal-900',
  edge: 'border-t-2 border-t-teal-400 dark:border-t-teal-500',
  selected:
    'border-teal-400 bg-teal-500/20 text-teal-800 hover:bg-teal-500/30 hover:text-teal-800 dark:border-teal-600 dark:text-teal-200 dark:hover:text-teal-200',
  on: 'data-[state=on]:border-teal-400 data-[state=on]:bg-teal-500/20 data-[state=on]:text-teal-800 data-[state=on]:hover:bg-teal-500/30 data-[state=on]:hover:text-teal-800 dark:data-[state=on]:border-teal-600 dark:data-[state=on]:text-teal-200 dark:data-[state=on]:hover:text-teal-200',
  solid: 'bg-teal-500 text-zinc-950 shadow-md shadow-teal-500/25 hover:bg-teal-500/90',
}
const cyan: ConceptColor = {
  text: 'text-cyan-700 dark:text-cyan-400',
  bg: 'bg-cyan-500/10',
  border: 'border-cyan-200 dark:border-cyan-900',
  edge: 'border-t-2 border-t-cyan-400 dark:border-t-cyan-500',
  selected:
    'border-cyan-400 bg-cyan-500/20 text-cyan-800 hover:bg-cyan-500/30 hover:text-cyan-800 dark:border-cyan-600 dark:text-cyan-200 dark:hover:text-cyan-200',
  on: 'data-[state=on]:border-cyan-400 data-[state=on]:bg-cyan-500/20 data-[state=on]:text-cyan-800 data-[state=on]:hover:bg-cyan-500/30 data-[state=on]:hover:text-cyan-800 dark:data-[state=on]:border-cyan-600 dark:data-[state=on]:text-cyan-200 dark:data-[state=on]:hover:text-cyan-200',
  solid: 'bg-cyan-500 text-zinc-950 shadow-md shadow-cyan-500/25 hover:bg-cyan-500/90',
}
const pink: ConceptColor = {
  text: 'text-pink-700 dark:text-pink-400',
  bg: 'bg-pink-500/10',
  border: 'border-pink-200 dark:border-pink-900',
  edge: 'border-t-2 border-t-pink-400 dark:border-t-pink-500',
  selected:
    'border-pink-400 bg-pink-500/20 text-pink-800 hover:bg-pink-500/30 hover:text-pink-800 dark:border-pink-600 dark:text-pink-200 dark:hover:text-pink-200',
  on: 'data-[state=on]:border-pink-400 data-[state=on]:bg-pink-500/20 data-[state=on]:text-pink-800 data-[state=on]:hover:bg-pink-500/30 data-[state=on]:hover:text-pink-800 dark:data-[state=on]:border-pink-600 dark:data-[state=on]:text-pink-200 dark:data-[state=on]:hover:text-pink-200',
  solid: 'bg-pink-600 text-white shadow-md shadow-pink-500/25 hover:bg-pink-600/90',
}
const orange: ConceptColor = {
  text: 'text-orange-700 dark:text-orange-400',
  bg: 'bg-orange-500/10',
  border: 'border-orange-200 dark:border-orange-900',
  edge: 'border-t-2 border-t-orange-400 dark:border-t-orange-500',
  selected:
    'border-orange-400 bg-orange-500/20 text-orange-800 hover:bg-orange-500/30 hover:text-orange-800 dark:border-orange-600 dark:text-orange-200 dark:hover:text-orange-200',
  on: 'data-[state=on]:border-orange-400 data-[state=on]:bg-orange-500/20 data-[state=on]:text-orange-800 data-[state=on]:hover:bg-orange-500/30 data-[state=on]:hover:text-orange-800 dark:data-[state=on]:border-orange-600 dark:data-[state=on]:text-orange-200 dark:data-[state=on]:hover:text-orange-200',
  solid: 'bg-orange-500 text-zinc-950 shadow-md shadow-orange-500/25 hover:bg-orange-500/90',
}
const zinc: ConceptColor = {
  text: 'text-zinc-600 dark:text-zinc-400',
  bg: 'bg-zinc-500/10',
  border: 'border-zinc-200 dark:border-zinc-800',
  edge: 'border-t-2 border-t-zinc-400 dark:border-t-zinc-500',
  selected:
    'border-zinc-400 bg-zinc-500/20 text-zinc-800 hover:bg-zinc-500/30 hover:text-zinc-800 dark:border-zinc-600 dark:text-zinc-200 dark:hover:text-zinc-200',
  on: 'data-[state=on]:border-zinc-400 data-[state=on]:bg-zinc-500/20 data-[state=on]:text-zinc-800 dark:data-[state=on]:border-zinc-600 dark:data-[state=on]:text-zinc-200',
  solid: 'bg-zinc-900 text-white hover:bg-zinc-900/90 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-100/90',
}

// Think mode (ADR 0006). Its labels (grounded, opinion, in doubt) own
// emerald, amber and red like statuses do, so no concept here uses them.
//   indigo   think, belief
//   fuchsia  decision
//   orange   rule (shares with savedSpec: the two never meet on one screen)
//   teal     evidence (shares with volume, same reason)
const indigo: ConceptColor = {
  text: 'text-indigo-700 dark:text-indigo-400',
  bg: 'bg-indigo-500/10',
  border: 'border-indigo-200 dark:border-indigo-900',
  edge: 'border-t-2 border-t-indigo-400 dark:border-t-indigo-500',
  selected:
    'border-indigo-400 bg-indigo-500/20 text-indigo-800 hover:bg-indigo-500/30 hover:text-indigo-800 dark:border-indigo-600 dark:text-indigo-200 dark:hover:text-indigo-200',
  on: 'data-[state=on]:border-indigo-400 data-[state=on]:bg-indigo-500/20 data-[state=on]:text-indigo-800 data-[state=on]:hover:bg-indigo-500/30 data-[state=on]:hover:text-indigo-800 dark:data-[state=on]:border-indigo-600 dark:data-[state=on]:text-indigo-200 dark:data-[state=on]:hover:text-indigo-200',
  solid: 'bg-indigo-600 text-white shadow-md shadow-indigo-500/25 hover:bg-indigo-600/90',
}
const fuchsia: ConceptColor = {
  text: 'text-fuchsia-700 dark:text-fuchsia-400',
  bg: 'bg-fuchsia-500/10',
  border: 'border-fuchsia-200 dark:border-fuchsia-900',
  edge: 'border-t-2 border-t-fuchsia-400 dark:border-t-fuchsia-500',
  selected:
    'border-fuchsia-400 bg-fuchsia-500/20 text-fuchsia-800 hover:bg-fuchsia-500/30 hover:text-fuchsia-800 dark:border-fuchsia-600 dark:text-fuchsia-200 dark:hover:text-fuchsia-200',
  on: 'data-[state=on]:border-fuchsia-400 data-[state=on]:bg-fuchsia-500/20 data-[state=on]:text-fuchsia-800 data-[state=on]:hover:bg-fuchsia-500/30 data-[state=on]:hover:text-fuchsia-800 dark:data-[state=on]:border-fuchsia-600 dark:data-[state=on]:text-fuchsia-200 dark:data-[state=on]:hover:text-fuchsia-200',
  solid: 'bg-fuchsia-600 text-white shadow-md shadow-fuchsia-500/25 hover:bg-fuchsia-600/90',
}

export const conceptColors = {
  task: violet,
  cpu: violet,
  memory: violet,
  image: violet,
  repo: blue,
  ref: blue,
  setup: blue,
  secret: yellow,
  env: lime,
  volume: teal,
  channel: pink,
  savedSpec: orange,
  terminal: cyan,
  message: pink,
  human: pink,
  raw: zinc,
  think: indigo,
  work: violet,
  belief: indigo,
  decision: fuchsia,
  rule: orange,
  evidence: teal,
  file: zinc,
  subject: zinc,
  // lime is env's in Work; the two never meet on one screen.
  action: lime,
  sandbox: violet,
  brief: zinc,
  // Workers, people and agents alike, share the messaging pink.
  agent: pink,
  // A setting, not a resource: neutral.
  notify: zinc,
  criteria: zinc,
  delivery: zinc,
  inbox: pink,
} satisfies Record<Concept, ConceptColor>
