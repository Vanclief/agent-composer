/**
 * Sessions — the tmux-style workspace tabs. Each session binds a
 * project directory (optionally a worktree branch) to one long-lived
 * main-agent conversation and the processes started beside it.
 *
 * These shapes mirror what the daemon would eventually serve; the
 * mock in ./mock.ts is the only thing that knows they are not real.
 */

export interface Session {
  id: string;
  name: string;
  /** Directory the agent and processes run in. */
  project: string;
  /** Worktree branch, empty when running on the checkout itself. */
  branch: string;
  /** Which agent answers — the Claude Code / ChatGPT style picker. */
  harness: string;
  model: string;
  effort: string;
  createdAt: string;
  /** Resumable harness session — set after the first reply. */
  sessionRef: string;
}

export type TurnStatus = "running" | "succeeded" | "failed";

export interface Turn {
  id: string;
  request: string;
  status: TurnStatus;
  reply?: string;
  /** Tool activity the agent reported while answering. */
  actions?: string[];
  /** The agent that answered — shown under the reply, like the Composer. */
  model?: string;
  effort?: string;
  error?: string;
  at: string;
}

export type ProcessStatus = "stopped" | "running" | "exited" | "failed";

export interface ProcessDefinition {
  /** Procfile key — `dev`, `api`, `test`… */
  name: string;
  command: string;
  /** Ad hoc commands are not in the Procfile; they vanish when removed. */
  adHoc: boolean;
}

export interface Process {
  id: string;
  sessionId: string;
  name: string;
  command: string;
  adHoc: boolean;
  status: ProcessStatus;
  /** PORT handed to the process so parallel worktrees do not collide. */
  port?: number;
  pid?: number;
  exitCode?: number;
  startedAt?: string;
  endedAt?: string;
}

export interface LogLine {
  /** Monotonic per process — the cursor for incremental reads. */
  seq: number;
  at: string;
  stream: "stdout" | "stderr";
  text: string;
}

/** What the tab strip shows beside a session's name. */
export interface SessionActivity {
  running: number;
  failed: number;
  /** The agent answered while the tab was not in view. */
  unread: boolean;
  thinking: boolean;
}
