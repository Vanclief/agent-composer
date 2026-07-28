import {
  type KeyboardEvent,
  useEffect,
  useState,
} from "react";
import {
  createWorktree,
  fetchWorktrees,
  removeWorktree,
} from "../api";
import type { BranchInfo, WorktreeInfo } from "../types/api";

/**
 * Workspace (git worktree) selection for a project. Collapsed to the
 * current choice; expanding shows the repository root, existing
 * workspaces, and a branch finder (GitHub-style: type to filter,
 * click to pick, create when nothing matches).
 */
export function WorkspacePicker({
  projectPath,
  value,
  onChange,
  disabled,
}: {
  projectPath: string;
  value: string;
  onChange: (branch: string) => void;
  disabled: boolean;
}) {
  const [isGit, setIsGit] = useState(false);
  const [repo, setRepo] = useState("");
  const [worktrees, setWorktrees] = useState<WorktreeInfo[]>([]);
  const [branches, setBranches] = useState<BranchInfo[]>([]);
  const [expanded, setExpanded] = useState(false);
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState(false);
  const [fetchingOrigin, setFetchingOrigin] = useState(false);
  const [error, setError] = useState("");
  // Set when a delete was refused (dirty worktree) — offers force.
  const [forcePath, setForcePath] = useState("");
  const [refresh, setRefresh] = useState(0);

  useEffect(() => {
    if (!projectPath.trim()) {
      setIsGit(false);
      return;
    }
    const controller = new AbortController();
    let active = true;

    fetchWorktrees(projectPath, controller.signal)
      .then((response) => {
        if (!active) {
          return;
        }
        setIsGit(response.is_git);
        setRepo(response.repo ?? "");
        setWorktrees(response.worktrees ?? []);
        setBranches(response.branches ?? []);
        setError("");
        setForcePath("");
      })
      .catch((caught: unknown) => {
        if (active) {
          setIsGit(false);
          setError(
            caught instanceof Error ? caught.message : String(caught),
          );
        }
      });

    return () => {
      active = false;
      controller.abort();
    };
  }, [projectPath, refresh]);

  // A project switch collapses and clears the finder; the parent owns
  // the selected workspace and restores it per project.
  useEffect(() => {
    setExpanded(false);
    setQuery("");
  }, [projectPath]);

  if (!isGit) {
    return error ? (
      <div className="builder-field-error">{error}</div>
    ) : null;
  }

  const main = worktrees.find((worktree) => worktree.is_main);
  const linked = worktrees.filter((worktree) => !worktree.is_main);
  const taken = new Set(
    worktrees.map((worktree) => worktree.branch).filter(Boolean),
  );
  const trimmedQuery = query.trim();
  const matchingBranches = branches.filter(
    (branch) =>
      !taken.has(branch.name) &&
      (!trimmedQuery ||
        branch.name
          .toLowerCase()
          .includes(trimmedQuery.toLowerCase())),
  );
  const exactMatch = branches.some(
    (branch) => branch.name === trimmedQuery,
  );

  function choose(branch: string) {
    onChange(branch);
    setExpanded(false);
    setQuery("");
  }

  async function refreshFromOrigin() {
    if (fetchingOrigin) {
      return;
    }
    setFetchingOrigin(true);
    setError("");
    try {
      const response = await fetchWorktrees(projectPath, undefined, true);
      setWorktrees(response.worktrees ?? []);
      setBranches(response.branches ?? []);
    } catch (caught) {
      setError(
        caught instanceof Error ? caught.message : String(caught),
      );
    } finally {
      setFetchingOrigin(false);
    }
  }

  async function handleCreate(branch: string) {
    if (!branch || busy) {
      return;
    }
    setBusy(true);
    setError("");
    try {
      const response = await createWorktree(repo, branch);
      setRefresh((count) => count + 1);
      choose(response.branch);
    } catch (caught) {
      setError(
        caught instanceof Error ? caught.message : String(caught),
      );
    } finally {
      setBusy(false);
    }
  }

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Enter") {
      event.preventDefault();
      const only = matchingBranches[0];
      if (matchingBranches.length === 1 && only) {
        choose(only.name);
      } else if (trimmedQuery && !exactMatch) {
        void handleCreate(trimmedQuery);
      }
    }
    if (event.key === "Escape") {
      setQuery("");
    }
  }

  async function handleRemove(worktree: WorktreeInfo, force: boolean) {
    setBusy(true);
    setError("");
    try {
      await removeWorktree(repo, worktree.path, force);
      if (value === worktree.branch) {
        onChange("");
      }
      setForcePath("");
      setRefresh((count) => count + 1);
    } catch (caught) {
      setForcePath(worktree.path);
      setError(
        caught instanceof Error ? caught.message : String(caught),
      );
    } finally {
      setBusy(false);
    }
  }

  const selectedPath = linked.find(
    (worktree) => worktree.branch === value,
  )?.path;

  return (
    <div className="workspace-picker">
      <div className="builder-modal__field-head">
        <label>Workspace</label>
        <button
          type="button"
          className="workspace-picker__refresh"
          title="Fetch origin so remote branches are current"
          disabled={disabled || fetchingOrigin}
          onClick={() => void refreshFromOrigin()}
        >
          {fetchingOrigin ? "Fetching…" : "↻ origin"}
        </button>
        <span className="workspace-picker__hint mono">
          git · {main?.branch ?? "detached"}
        </span>
      </div>

      <div className="workspace-picker__options">
        {!expanded ? (
          <div className="workspace-picker__option workspace-picker__option--row active">
            <button
              type="button"
              disabled={disabled}
              title="Change workspace"
              onClick={() => setExpanded(true)}
            >
              <b>{value || "Repository root"}</b>
              <small className={value ? "mono" : undefined}>
                {value
                  ? (selectedPath ?? "will be prepared at launch")
                  : `runs directly on ${main?.branch ?? "the checkout"}`}
              </small>
            </button>
            <button
              type="button"
              className="project-picker__change"
              disabled={disabled}
              onClick={() => setExpanded(true)}
            >
              Change
            </button>
          </div>
        ) : (
          <>
            <button
              type="button"
              className={`workspace-picker__option ${
                value === "" ? "active" : ""
              }`}
              disabled={disabled}
              onClick={() => choose("")}
            >
              <b>Repository root</b>
              <small>
                runs directly on {main?.branch ?? "the checkout"}
              </small>
            </button>

            {linked.map((worktree) => (
              <div
                key={worktree.path}
                className={`workspace-picker__option workspace-picker__option--row ${
                  value === worktree.branch ? "active" : ""
                }`}
              >
                <button
                  type="button"
                  disabled={disabled}
                  onClick={() => choose(worktree.branch ?? "")}
                >
                  <b>{worktree.branch || worktree.path}</b>
                  <small className="mono">{worktree.path}</small>
                </button>
                <button
                  type="button"
                  className="workspace-picker__remove"
                  title="Delete workspace (keeps the branch)"
                  aria-label={`Delete workspace ${worktree.branch}`}
                  disabled={disabled || busy}
                  onClick={() => void handleRemove(worktree, false)}
                >
                  ×
                </button>
              </div>
            ))}

            <div className="branch-finder">
              <input
                className="builder-input"
                placeholder="Find or create a branch…"
                value={query}
                autoFocus
                disabled={disabled || busy}
                onChange={(event) => {
                  setQuery(event.target.value);
                  setError("");
                }}
                onKeyDown={handleKeyDown}
              />
              <div className="branch-finder__list scrollnice">
                {matchingBranches.map((branch) => (
                  <button
                    type="button"
                    key={branch.name}
                    className="branch-finder__row"
                    disabled={disabled || busy}
                    onClick={() => choose(branch.name)}
                  >
                    <b>{branch.name}</b>
                    {!branch.is_local && branch.is_remote && (
                      <small>origin only</small>
                    )}
                  </button>
                ))}
                {trimmedQuery && !exactMatch && (
                  <button
                    type="button"
                    className="branch-finder__row branch-finder__row--create"
                    disabled={disabled || busy}
                    onClick={() => void handleCreate(trimmedQuery)}
                  >
                    <b>
                      {busy
                        ? "Creating…"
                        : `+ Create branch “${trimmedQuery}”`}
                    </b>
                    <small>from {main?.branch ?? "HEAD"}</small>
                  </button>
                )}
                {matchingBranches.length === 0 &&
                  (!trimmedQuery || exactMatch) && (
                    <div className="branch-finder__empty">
                      No other branches
                    </div>
                  )}
              </div>
            </div>
          </>
        )}
      </div>

      {error && (
        <div className="builder-field-error">
          {error}
          {forcePath && (
            <button
              type="button"
              className="workspace-picker__force"
              disabled={busy}
              onClick={() => {
                const target = linked.find(
                  (worktree) => worktree.path === forcePath,
                );
                if (target) {
                  void handleRemove(target, true);
                }
              }}
            >
              Force delete (discards uncommitted changes)
            </button>
          )}
        </div>
      )}
    </div>
  );
}
