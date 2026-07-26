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
 * Workspace (git worktree) selection for a project. Only rendered
 * when the project is a git repository. Creating a workspace makes
 * the worktree immediately; the run then executes inside it.
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
  const [creating, setCreating] = useState(false);
  // "" = new branch (type a name); otherwise an existing branch name.
  const [branchChoice, setBranchChoice] = useState("");
  const [draft, setDraft] = useState("");
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

  // A project switch invalidates the chosen workspace.
  useEffect(() => {
    onChange("");
    setCreating(false);
    setBranchChoice("");
    setDraft("");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectPath]);

  if (!isGit) {
    return error ? (
      <div className="builder-field-error">{error}</div>
    ) : null;
  }

  const main = worktrees.find((worktree) => worktree.is_main);
  const linked = worktrees.filter((worktree) => !worktree.is_main);
  // Branches that could become a workspace: no worktree yet.
  const taken = new Set(
    worktrees.map((worktree) => worktree.branch).filter(Boolean),
  );
  const availableBranches = branches.filter(
    (branch) => !taken.has(branch.name),
  );

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

  async function handleCreate() {
    const branch = branchChoice || draft.trim();
    if (!branch || busy) {
      return;
    }
    setBusy(true);
    setError("");
    try {
      const response = await createWorktree(repo, branch);
      onChange(response.branch);
      setCreating(false);
      setBranchChoice("");
      setDraft("");
      setRefresh((count) => count + 1);
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
      void handleCreate();
    }
    if (event.key === "Escape") {
      setCreating(false);
      setDraft("");
      setError("");
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
        <button
          type="button"
          className={`workspace-picker__option ${
            value === "" ? "active" : ""
          }`}
          disabled={disabled}
          onClick={() => onChange("")}
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
              onClick={() => onChange(worktree.branch ?? "")}
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

        {creating ? (
          <div className="workspace-picker__new active">
            <div className="workspace-picker__new-form">
              <select
                className="builder-select"
                value={branchChoice}
                disabled={busy}
                onChange={(event) => {
                  setBranchChoice(event.target.value);
                  setError("");
                }}
              >
                <option value="">New branch…</option>
                {availableBranches.map((branch) => (
                  <option key={branch.name} value={branch.name}>
                    {branch.name}
                    {!branch.is_local && branch.is_remote
                      ? " (origin only)"
                      : ""}
                  </option>
                ))}
              </select>
            </div>
            <div className="workspace-picker__new-form">
              {branchChoice === "" && (
                <input
                  className="builder-input mono"
                  placeholder="branch name, e.g. feature/faster-summary"
                  value={draft}
                  autoFocus
                  disabled={busy}
                  onChange={(event) => {
                    setDraft(event.target.value);
                    setError("");
                  }}
                  onKeyDown={handleKeyDown}
                />
              )}
              <button
                type="button"
                className="builder-run-button"
                disabled={(!branchChoice && !draft.trim()) || busy}
                onClick={() => void handleCreate()}
              >
                {busy ? "Creating…" : "Create"}
              </button>
            </div>
            <small>
              {branchChoice === ""
                ? "Creates a new branch from the current HEAD."
                : availableBranches.find(
                      (branch) => branch.name === branchChoice,
                    )?.is_local
                  ? "Checks out the existing branch into its own workspace."
                  : "Creates a local branch tracking origin and checks it out."}
            </small>
          </div>
        ) : (
          <button
            type="button"
            className="workspace-picker__new-toggle"
            disabled={disabled}
            onClick={() => setCreating(true)}
          >
            + New workspace
          </button>
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
