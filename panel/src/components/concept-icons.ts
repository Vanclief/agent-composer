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
} satisfies Record<string, LucideIcon>

export type Concept = keyof typeof conceptIcons
