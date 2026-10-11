/**
 * Mock backend for Sessions.
 *
 * Everything the real daemon would own — sessions, transcripts,
 * managed processes and their logs — lives here as module state so
 * the UI can be prototyped against the intended contract. Sessions
 * and transcripts persist in localStorage; processes are in-memory,
 * exactly like they would be in the daemon (a restart forgets them,
 * the Procfile brings them back in one click).
 *
 * Swapping this for the real thing means replacing the bodies of the
 * exported functions with fetch calls and dropping the simulation.
 */
import { useSyncExternalStore } from "react";
import { readStoredProjects } from "../builder/useLaunchLocation";
import type {
  LogLine,
  Process,
  ProcessDefinition,
  Session,
  SessionActivity,
  Turn,
} from "./types";

const SESSIONS_KEY = "agc.sessions";
const TRANSCRIPTS_KEY = "agc.sessions.transcripts";
const LOG_LIMIT = 2000;

// ── state ──────────────────────────────────────────────────────────

let sessions: Session[] = [];
const transcripts = new Map<string, Turn[]>();
const processes = new Map<string, Process>();
const logs = new Map<string, LogLine[]>();
const adHocDefinitions = new Map<string, ProcessDefinition[]>();
const timers = new Map<string, number>();
const unread = new Set<string>();
const thinking = new Set<string>();
let activeSessionId = "";
let version = 0;
const listeners = new Set<() => void>();

function emit() {
  version += 1;
  for (const listener of listeners) {
    listener();
  }
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** Re-renders the caller whenever anything in the store changes. */
export function useSessionsStore() {
  return useSyncExternalStore(subscribe, () => version);
}

function now() {
  return new Date().toISOString();
}

function newId() {
  return crypto.randomUUID();
}

// ── persistence ────────────────────────────────────────────────────

function load() {
  try {
    const stored = JSON.parse(
      localStorage.getItem(SESSIONS_KEY) || "null",
    ) as unknown;
    if (Array.isArray(stored)) {
      // Tabs saved before the picker existed have no effort yet.
      sessions = (stored as Session[]).map((session) => ({
        ...session,
        effort: session.effort || "medium",
      }));
    }
    const storedTurns = JSON.parse(
      localStorage.getItem(TRANSCRIPTS_KEY) || "null",
    ) as unknown;
    if (storedTurns && typeof storedTurns === "object") {
      for (const [id, turns] of Object.entries(
        storedTurns as Record<string, Turn[]>,
      )) {
        // A reload cannot resume an in-flight mock reply.
        transcripts.set(
          id,
          turns.map((turn) =>
            turn.status === "running"
              ? { ...turn, status: "failed", error: "Interrupted by reload" }
              : turn,
          ),
        );
      }
    }
  } catch {
    // Ignore invalid values left by an older browser session.
  }

  if (sessions.length === 0) {
    seed();
  }
}

function save() {
  try {
    localStorage.setItem(SESSIONS_KEY, JSON.stringify(sessions));
    localStorage.setItem(
      TRANSCRIPTS_KEY,
      JSON.stringify(Object.fromEntries(transcripts)),
    );
  } catch {
    // Storage may be unavailable; the session keeps working in memory.
  }
}

/** First run: one tab per project you already launch runs from. */
function seed() {
  const projects = readStoredProjects().slice(0, 3);
  sessions = projects.map((project) => ({
    id: newId(),
    name: project.name || project.path.split("/").filter(Boolean).pop() || "",
    project: project.path,
    branch: "",
    harness: "",
    model: "",
    effort: "medium",
    createdAt: now(),
    sessionRef: "",
  }));
  save();
}

load();

// ── sessions ───────────────────────────────────────────────────────

export function listSessions(): Session[] {
  return sessions;
}

export function getSession(id: string) {
  return sessions.find((session) => session.id === id);
}

export function createSession(input: {
  name: string;
  project: string;
  branch: string;
  harness?: string;
  model?: string;
  effort?: string;
}): Session {
  const session: Session = {
    id: newId(),
    name: input.name.trim() || input.branch || projectBase(input.project),
    project: input.project,
    branch: input.branch,
    harness: input.harness ?? "",
    model: input.model ?? "",
    effort: input.effort ?? "medium",
    createdAt: now(),
    sessionRef: "",
  };
  sessions = [...sessions, session];
  save();
  emit();
  return session;
}

export function renameSession(id: string, name: string) {
  sessions = sessions.map((session) =>
    session.id === id ? { ...session, name: name.trim() || session.name } : session,
  );
  save();
  emit();
}

/** The agent that answers from now on — remembered per tab. */
export function setSessionAgent(
  id: string,
  agent: { harness: string; model: string; effort: string },
) {
  sessions = sessions.map((session) =>
    session.id === id ? { ...session, ...agent } : session,
  );
  save();
  emit();
}

export function removeSession(id: string) {
  for (const process of processes.values()) {
    if (process.sessionId === id) {
      stopProcess(process.id);
      processes.delete(process.id);
      logs.delete(process.id);
    }
  }
  sessions = sessions.filter((session) => session.id !== id);
  transcripts.delete(id);
  adHocDefinitions.delete(id);
  unread.delete(id);
  save();
  emit();
}

export function moveSession(id: string, direction: -1 | 1) {
  const index = sessions.findIndex((session) => session.id === id);
  const target = index + direction;
  if (index < 0 || target < 0 || target >= sessions.length) {
    return;
  }
  const next = [...sessions];
  const [moved] = next.splice(index, 1);
  if (!moved) {
    return;
  }
  next.splice(target, 0, moved);
  sessions = next;
  save();
  emit();
}

/** The tab in view — replies landing elsewhere mark their tab unread. */
export function setActiveSession(id: string) {
  activeSessionId = id;
  if (unread.delete(id)) {
    emit();
  }
}

export function activity(sessionId: string): SessionActivity {
  let running = 0;
  let failed = 0;
  for (const process of processes.values()) {
    if (process.sessionId !== sessionId) {
      continue;
    }
    if (process.status === "running") {
      running += 1;
    }
    if (process.status === "failed") {
      failed += 1;
    }
  }
  return {
    running,
    failed,
    unread: unread.has(sessionId),
    thinking: thinking.has(sessionId),
  };
}

function projectBase(path: string) {
  return path.replace(/\/+$/, "").split("/").pop() || path;
}

// ── chat ───────────────────────────────────────────────────────────

export function turns(sessionId: string): Turn[] {
  return transcripts.get(sessionId) ?? [];
}

function updateTurn(sessionId: string, turnId: string, patch: Partial<Turn>) {
  const current = transcripts.get(sessionId) ?? [];
  transcripts.set(
    sessionId,
    current.map((turn) => (turn.id === turnId ? { ...turn, ...patch } : turn)),
  );
  save();
  emit();
}

export function clearTranscript(sessionId: string) {
  transcripts.delete(sessionId);
  sessions = sessions.map((session) =>
    session.id === sessionId ? { ...session, sessionRef: "" } : session,
  );
  save();
  emit();
}

export function sendMessage(sessionId: string, request: string) {
  const session = getSession(sessionId);
  if (!session) {
    return Promise.reject(new Error("Session not found"));
  }

  const turn: Turn = {
    id: newId(),
    request,
    status: "running",
    at: now(),
  };
  transcripts.set(sessionId, [...turns(sessionId), turn]);
  thinking.add(sessionId);
  save();
  emit();

  const delay = 700 + Math.random() * 900;
  return new Promise<void>((resolve) => {
    window.setTimeout(() => {
      const answer = answerFor(session, request);
      thinking.delete(sessionId);
      if (!session.sessionRef) {
        sessions = sessions.map((item) =>
          item.id === sessionId
            ? { ...item, sessionRef: `mock-${newId().slice(0, 8)}` }
            : item,
        );
      }
      if (activeSessionId !== sessionId) {
        unread.add(sessionId);
      }
      const current = getSession(sessionId) ?? session;
      updateTurn(sessionId, turn.id, {
        status: "succeeded",
        reply: answer.reply,
        actions: answer.actions,
        model: current.model,
        effort: current.effort,
      });
      resolve();
    }, delay);
  });
}

/**
 * The agent's pretend brain. It can do exactly what the real one will
 * do through MCP: list processes, start/stop/restart them, and read
 * their logs. Anything else gets a canned answer.
 */
function answerFor(
  session: Session,
  request: string,
): { reply: string; actions: string[] } {
  const text = request.toLowerCase();
  const defs = definitions(session.id);
  const named = defs.find((definition) =>
    new RegExp(`\\b${definition.name}\\b`).test(text),
  );
  const mine = processesFor(session.id);

  if (/\b(restart|relaunch)\b/.test(text)) {
    const target =
      (named && processFor(session.id, named.name)) ??
      mine.find((process) => process.status === "failed") ??
      mine.find((process) => process.status === "running");
    if (!target) {
      return {
        reply: "Nothing to restart — no process has run in this session yet.",
        actions: ["process_list"],
      };
    }
    stopProcess(target.id);
    const started = startProcess(session.id, target.name);
    return {
      reply: `Restarted \`${target.name}\` (${target.command})${
        started?.port ? ` on PORT ${started.port}` : ""
      }. I'll read its logs if it misbehaves again.`,
      actions: [`process_stop ${target.name}`, `process_start ${target.name}`],
    };
  }

  if (/\b(start|run|launch|spin up|bring up)\b/.test(text) && named) {
    const existing = processFor(session.id, named.name);
    if (existing?.status === "running") {
      return {
        reply: `\`${named.name}\` is already running${
          existing.port ? ` on PORT ${existing.port}` : ""
        } (pid ${existing.pid}).`,
        actions: ["process_list"],
      };
    }
    const started = startProcess(session.id, named.name);
    return {
      reply: `Started \`${named.name}\` → \`${named.command}\`${
        started?.port ? ` with PORT=${started.port}` : ""
      }. It shows up in the Processes panel; ask me for its logs any time.`,
      actions: [`process_start ${named.name}`],
    };
  }

  if (/\b(stop|kill|shut down|shutdown)\b/.test(text)) {
    const target = named
      ? processFor(session.id, named.name)
      : mine.find((process) => process.status === "running");
    if (!target || target.status !== "running") {
      return {
        reply: named
          ? `\`${named.name}\` is not running.`
          : "Nothing is running in this session.",
        actions: ["process_list"],
      };
    }
    stopProcess(target.id);
    return {
      reply: `Stopped \`${target.name}\` (pid ${target.pid}).`,
      actions: [`process_stop ${target.name}`],
    };
  }

  if (/\b(log|logs|output|error|errors|crash|crashed|fail|failed|why|broke|wrong|happened)\b/.test(text)) {
    const target =
      (named && processFor(session.id, named.name)) ??
      mine.find((process) => process.status === "failed") ??
      mine.find((process) => process.status === "running") ??
      mine[0];
    if (!target) {
      return {
        reply: `Nothing has run in this session yet. The Procfile defines ${defs
          .map((definition) => `\`${definition.name}\``)
          .join(", ")} — want me to start one?`,
        actions: ["process_list"],
      };
    }
    const tail = readLogs(target.id)
      .slice(-8)
      .map((line) => (line.text ? `\`${line.text}\`` : ""))
      .join("\n");
    const verdict =
      target.status === "failed"
        ? `\`${target.name}\` exited with code ${target.exitCode}. The last lines point at a nil dereference after a locked SQLite database — most likely two processes opened the same file. I can restart it, or look at the code path first.`
        : target.status === "running"
          ? `\`${target.name}\` is healthy${
              target.port ? ` on PORT ${target.port}` : ""
            } — no errors in the recent output.`
          : `\`${target.name}\` finished with code ${target.exitCode}.`;
    return {
      reply: `${verdict}\n\nLast lines of \`${target.name}\`:\n${tail || "(no output)"}`,
      actions: [`process_logs ${target.name} --tail 40`],
    };
  }

  if (/\b(status|running|processes|what's up|whats up)\b/.test(text)) {
    if (mine.length === 0) {
      return {
        reply: `No processes yet. Procfile: ${defs
          .map((definition) => `\`${definition.name}\` (${definition.command})`)
          .join(", ")}.`,
        actions: ["process_list"],
      };
    }
    return {
      reply: mine
        .map(
          (process) =>
            `• \`${process.name}\` — ${process.status}${
              process.port ? ` on ${process.port}` : ""
            }${process.exitCode !== undefined ? ` (exit ${process.exitCode})` : ""}`,
        )
        .join("\n"),
      actions: ["process_list"],
    };
  }

  const where = session.branch
    ? `${projectBase(session.project)} on \`${session.branch}\``
    : projectBase(session.project);
  return {
    reply: `(mock) I'm the main agent for **${session.name}**, working in ${where}. This reply is canned — the real one runs your harness with \`--resume\` in that directory and gets process tools over MCP. Try: "start dev", "why did api crash?", "status", "restart api".`,
    actions: [],
  };
}

// ── processes ──────────────────────────────────────────────────────

/** Procfile-style definitions; the same for every project in the mock. */
const PROCFILE: ProcessDefinition[] = [
  { name: "dev", command: "npm run dev", adHoc: false },
  { name: "api", command: "go run . rest", adHoc: false },
  { name: "test", command: "go test ./...", adHoc: false },
];

export function definitions(sessionId: string): ProcessDefinition[] {
  return [...PROCFILE, ...(adHocDefinitions.get(sessionId) ?? [])];
}

export function processesFor(sessionId: string): Process[] {
  return [...processes.values()].filter(
    (process) => process.sessionId === sessionId,
  );
}

export function processFor(sessionId: string, name: string) {
  return processesFor(sessionId).find((process) => process.name === name);
}

export function readLogs(processId: string, since = 0): LogLine[] {
  const lines = logs.get(processId) ?? [];
  if (since <= 0) {
    return lines;
  }
  return lines.filter((line) => line.seq > since);
}

let nextPort = 3000;
let nextPid = 48_000;

function append(processId: string, text: string, stream: LogLine["stream"] = "stdout") {
  const lines = logs.get(processId) ?? [];
  const seq = (lines[lines.length - 1]?.seq ?? 0) + 1;
  lines.push({ seq, at: now(), stream, text });
  if (lines.length > LOG_LIMIT) {
    lines.splice(0, lines.length - LOG_LIMIT);
  }
  logs.set(processId, lines);
}

function setProcess(process: Process) {
  processes.set(process.id, process);
  emit();
}

export function startProcess(sessionId: string, name: string) {
  const definition = definitions(sessionId).find(
    (item) => item.name === name,
  );
  if (!definition) {
    return undefined;
  }
  const previous = processFor(sessionId, name);
  if (previous?.status === "running") {
    return previous;
  }
  if (previous) {
    processes.delete(previous.id);
    logs.delete(previous.id);
  }

  const usesPort = /\b(dev|rest|serve|api|start)\b/.test(definition.command);
  const process: Process = {
    id: newId(),
    sessionId,
    name,
    command: definition.command,
    adHoc: definition.adHoc,
    status: "running",
    port: usesPort ? nextPort++ : undefined,
    pid: nextPid++,
    startedAt: now(),
  };
  setProcess(process);
  append(process.id, `$ ${definition.command}`);
  simulate(process);
  return process;
}

export function runCommand(sessionId: string, command: string) {
  const trimmed = command.trim();
  if (!trimmed) {
    return undefined;
  }
  const base = trimmed.split(/\s+/)[0]?.replace(/[^a-z0-9_-]/gi, "") || "cmd";
  const existing = definitions(sessionId).map((definition) => definition.name);
  let name = base;
  let suffix = 2;
  while (existing.includes(name)) {
    name = `${base}-${suffix}`;
    suffix += 1;
  }
  adHocDefinitions.set(sessionId, [
    ...(adHocDefinitions.get(sessionId) ?? []),
    { name, command: trimmed, adHoc: true },
  ]);
  return startProcess(sessionId, name);
}

export function stopProcess(processId: string) {
  const process = processes.get(processId);
  if (!process || process.status !== "running") {
    return;
  }
  clearTimers(processId);
  append(processId, `^C — sent SIGTERM to process group ${process.pid}`);
  setProcess({
    ...process,
    status: "exited",
    exitCode: 130,
    endedAt: now(),
  });
}

/** Forget a finished process (and an ad hoc definition with it). */
export function removeProcess(processId: string) {
  const process = processes.get(processId);
  if (!process) {
    return;
  }
  if (process.status === "running") {
    stopProcess(processId);
  }
  processes.delete(processId);
  logs.delete(processId);
  if (process.adHoc) {
    adHocDefinitions.set(
      process.sessionId,
      (adHocDefinitions.get(process.sessionId) ?? []).filter(
        (definition) => definition.name !== process.name,
      ),
    );
  }
  emit();
}

function clearTimers(processId: string) {
  const handle = timers.get(processId);
  if (handle !== undefined) {
    window.clearInterval(handle);
    timers.delete(processId);
  }
}

function exit(process: Process, code: number) {
  clearTimers(process.id);
  const current = processes.get(process.id);
  if (!current || current.status !== "running") {
    return;
  }
  setProcess({
    ...current,
    status: code === 0 ? "exited" : "failed",
    exitCode: code,
    endedAt: now(),
  });
}

// ── simulation ─────────────────────────────────────────────────────

const DEV_LINES = [
  "[vite] hmr update /src/App.tsx",
  "[vite] hmr update /src/styles/sessions.css",
  "GET / 200 in 9ms",
  "GET /api/workflows 200 in 31ms",
  "GET /api/workflow/executions?limit=50 200 in 44ms",
  "[vite] page reload src/sessions/mock.ts",
  "GET /api/harnesses 200 in 6ms",
];

const API_LINES = [
  '{"level":"info","method":"GET","path":"/api/workflows","status":200,"ms":12}',
  '{"level":"info","method":"GET","path":"/api/workflow/executions","status":200,"ms":38}',
  '{"level":"info","msg":"scheduler tick","pending":0}',
  '{"level":"info","method":"POST","path":"/api/workflow/executions","status":201,"ms":71}',
  '{"level":"warn","msg":"harness claude-code took 2.4s to start"}',
];

const API_CRASH = [
  '{"level":"error","op":"executions.Create","err":"database is locked"}',
  "panic: runtime error: invalid memory address or nil pointer dereference",
  "[signal SIGSEGV: segmentation violation code=0x2 addr=0x18 pc=0x104b2c3d0]",
  "",
  "goroutine 42 [running]:",
  "github.com/vanclief/agent-composer/core/controller.(*Controller).Create(...)",
  "\t/core/controller/executions.go:88 +0x1d0",
  "exit status 2",
];

const TEST_LINES = [
  "=== RUN   TestResolveWorktree",
  "--- PASS: TestResolveWorktree (0.01s)",
  "=== RUN   TestResolveWorktree/existing_branch",
  "--- PASS: TestResolveWorktree/existing_branch (0.00s)",
  "=== RUN   TestComposeRoundTrip",
  "--- PASS: TestComposeRoundTrip (0.21s)",
  "ok  \tgithub.com/vanclief/agent-composer/worktree\t0.412s",
  "ok  \tgithub.com/vanclief/agent-composer/core/resources/workflow\t1.083s",
];

function pick<T>(items: T[]) {
  return items[Math.floor(Math.random() * items.length)] as T;
}

function simulate(process: Process) {
  const { id, name, port } = process;

  if (name === "dev") {
    append(id, "");
    append(id, "> agc-web@0.0.0 dev");
    append(id, "> vite");
    append(id, "");
    append(id, "  VITE v6.3.5  ready in 412 ms");
    append(id, `  ➜  Local:   http://localhost:${port}/`);
    append(id, "  ➜  Network: use --host to expose");
    emit();
    timers.set(
      id,
      window.setInterval(() => {
        append(id, pick(DEV_LINES));
        emit();
      }, 1800),
    );
    return;
  }

  if (name === "api") {
    append(id, `{"level":"info","msg":"listening","addr":":${port}"}`);
    append(id, '{"level":"info","msg":"sqlite opened","path":"~/.agent_composer/agc.db"}');
    emit();
    let ticks = 0;
    timers.set(
      id,
      window.setInterval(() => {
        ticks += 1;
        if (ticks >= 12) {
          for (const line of API_CRASH) {
            append(id, line, "stderr");
          }
          exit(process, 2);
          return;
        }
        append(id, pick(API_LINES));
        emit();
      }, 1500),
    );
    return;
  }

  if (name === "test") {
    let index = 0;
    timers.set(
      id,
      window.setInterval(() => {
        if (index >= TEST_LINES.length) {
          exit(process, 0);
          return;
        }
        append(id, TEST_LINES[index] ?? "");
        index += 1;
        emit();
      }, 650),
    );
    return;
  }

  // Ad hoc: a short-lived command that echoes and exits cleanly.
  let ticks = 0;
  timers.set(
    id,
    window.setInterval(() => {
      ticks += 1;
      if (ticks > 3) {
        exit(process, 0);
        return;
      }
      append(id, `… ${name}: step ${ticks}/3`);
      emit();
    }, 900),
  );
}
