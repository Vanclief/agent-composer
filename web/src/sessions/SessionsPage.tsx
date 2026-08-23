import { useEffect, useState } from "react";
import { Navigate, useNavigate, useParams } from "react-router-dom";
import { RunLocation } from "../builder/RunLocation";
import { appRailItems } from "../layout/appRail";
import { LeftRail } from "../layout/LeftRail";
import { TopBar } from "../layout/TopBar";
import { SettingsRailButton } from "../nav/SettingsButton";
import { ChatPanel } from "./ChatPanel";
import {
  activity,
  getSession,
  listSessions,
  removeSession,
  renameSession,
  setActiveSession,
  useSessionsStore,
} from "./mock";
import { NewSessionModal } from "./NewSessionModal";
import { ProcessesPanel } from "./ProcessesPanel";
import { TabStrip } from "./TabStrip";
import type { Session } from "./types";
import { useTabKeys } from "./useTabKeys";

function sessionPath(id: string) {
  return `/sessions/${encodeURIComponent(id)}`;
}

/**
 * The Sessions section: one workspace per project or worktree, picked
 * from the tmux-style strip along the bottom. The route carries the
 * session in view so a tab survives a reload and can be linked to.
 */
export function SessionsPage() {
  useSessionsStore();
  const navigate = useNavigate();
  const { sessionId = "" } = useParams();
  const sessions = listSessions();
  const session = getSession(sessionId);
  const activeId = session?.id ?? "";

  const [creating, setCreating] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [draftName, setDraftName] = useState("");

  useEffect(() => {
    document.title = session ? `AGC — ${session.name}` : "AGC — Sessions";
  }, [session]);

  // The tab you are looking at is the active one, which clears its
  // unread mark — replies landing elsewhere keep theirs.
  useEffect(() => {
    if (activeId) {
      setActiveSession(activeId);
    }
  }, [activeId]);

  const { armed } = useTabKeys({
    sessions,
    activeId,
    onSelect: (id) => navigate(sessionPath(id)),
    onNew: () => setCreating(true),
  });

  const first = sessions[0];
  if (!session && first) {
    return <Navigate to={sessionPath(first.id)} replace />;
  }

  function stopRenaming(commit: boolean) {
    if (commit && session && draftName.trim()) {
      renameSession(session.id, draftName);
    }
    setRenaming(false);
  }

  function closeSession(target: Session) {
    const state = activity(target.id);
    if (
      state.running > 0 &&
      !window.confirm(
        `${target.name} still has ${state.running} running ` +
          `process${state.running === 1 ? "" : "es"}. Close it anyway?`,
      )
    ) {
      return;
    }
    // Land on the tab that takes its place, like closing a tmux window.
    const index = sessions.findIndex((item) => item.id === target.id);
    const neighbour = sessions[index + 1] ?? sessions[index - 1];
    removeSession(target.id);
    navigate(neighbour ? sessionPath(neighbour.id) : "/sessions", {
      replace: true,
    });
  }

  return (
    <div
      className="task-console has-rail sessions-page"
      data-component="SessionsPage"
    >
      <TopBar
        title={
          session ? (
            renaming ? (
              <input
                className="session-title__input"
                aria-label="Session name"
                autoFocus
                value={draftName}
                onChange={(event) => setDraftName(event.target.value)}
                onBlur={() => stopRenaming(true)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    stopRenaming(true);
                  }
                  if (event.key === "Escape") {
                    event.preventDefault();
                    stopRenaming(false);
                  }
                }}
              />
            ) : (
              <button
                type="button"
                className="session-title"
                title="Rename session"
                onClick={() => {
                  setDraftName(session.name);
                  setRenaming(true);
                }}
              >
                {session.name}
              </button>
            )
          ) : (
            "Sessions"
          )
        }
        context={
          session && (
            <>
              <RunLocation project={session.project} />
              {session.branch && (
                <span
                  className="sessions-page__branch mono"
                  title={`Worktree branch — ${session.branch}`}
                >
                  {session.branch}
                </span>
              )}
            </>
          )
        }
        actions={
          <>
            <button
              type="button"
              className="builder-run-button"
              onClick={() => setCreating(true)}
            >
              New session
            </button>
            {session && (
              <button
                type="button"
                className="builder-icon-button"
                title="Close session"
                aria-label="Close session"
                onClick={() => closeSession(session)}
              >
                ×
              </button>
            )}
          </>
        }
      />

      <main className="sessions-page__workspace">
        <LeftRail
          items={appRailItems()}
          active="sessions"
          footer={<SettingsRailButton />}
        />
        {session ? (
          <>
            <ChatPanel session={session} />
            <ProcessesPanel session={session} />
          </>
        ) : (
          <div className="sessions-page__empty">
            <div className="sessions-page__empty-card">
              <p className="sessions-page__eyebrow">Sessions</p>
              <h2>No sessions yet</h2>
              <p>
                A session is a project or worktree with its own main
                agent and processes.
              </p>
              <button
                type="button"
                className="builder-run-button"
                onClick={() => setCreating(true)}
              >
                New session
              </button>
            </div>
          </div>
        )}
        <TabStrip
          sessions={sessions}
          activeId={activeId}
          armed={armed}
          onSelect={(id) => navigate(sessionPath(id))}
          onNew={() => setCreating(true)}
        />
      </main>

      {creating && (
        <NewSessionModal
          onCreate={(created) => {
            setCreating(false);
            navigate(sessionPath(created.id));
          }}
          onClose={() => setCreating(false)}
        />
      )}
    </div>
  );
}
