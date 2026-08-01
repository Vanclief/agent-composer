import { useEffect, useState } from "react";
import { fetchWorktrees } from "../api";
import { BranchIcon, FolderIcon } from "./Icons";
import { projectBaseName } from "./ProjectPicker";
import { readStoredProjects } from "./useLaunchLocation";

function projectNameFor(path: string) {
  const match = readStoredProjects().find(
    (project) => project.path === path,
  );
  return match?.name || projectBaseName(path);
}

function stripSlash(path: string) {
  return path.replace(/\/+$/, "");
}

interface Location {
  /** Display name of the project (repository root). */
  project: string;
  /** Branch the run executes on; empty until git answers. */
  branch: string;
  /** True when the run happens in a linked worktree. */
  worktree: boolean;
}

/**
 * Where a run executes: project chip + workspace chip. Only shell_root
 * is persisted, so git resolves the rest — for worktree runs the
 * project is the main checkout and the workspace is the worktree's
 * branch; for root runs the workspace is whatever branch the checkout
 * is on.
 */
export function RunLocation({ shellRoot }: { shellRoot: string }) {
  const [location, setLocation] = useState<Location>(() => ({
    project: projectNameFor(shellRoot),
    branch: "",
    worktree: false,
  }));

  useEffect(() => {
    setLocation({
      project: projectNameFor(shellRoot),
      branch: "",
      worktree: false,
    });
    const controller = new AbortController();
    fetchWorktrees(shellRoot, controller.signal)
      .then((response) => {
        const worktrees = response.worktrees ?? [];
        const main = worktrees.find((info) => info.is_main);
        if (!response.is_git || !main) {
          return;
        }
        // The run's own worktree — or the main checkout when the run
        // executes directly on the repository root.
        const current =
          worktrees.find(
            (info) => stripSlash(info.path) === stripSlash(shellRoot),
          ) ?? main;
        setLocation({
          project: projectNameFor(main.path),
          branch: current.branch || current.head?.slice(0, 7) || "",
          worktree: current !== main,
        });
      })
      .catch(() => undefined);
    return () => controller.abort();
  }, [shellRoot]);

  return (
    <div className="run-location" title={shellRoot}>
      <span className="run-location__chip">
        <FolderIcon size={12} />
        <b>{location.project}</b>
      </span>
      {location.branch && (
        <span
          className="run-location__chip run-location__chip--branch"
          title={
            location.worktree
              ? `Workspace worktree — ${shellRoot}`
              : "Runs directly on the repository root"
          }
        >
          <BranchIcon size={12} />
          <span className="mono">{location.branch}</span>
        </span>
      )}
    </div>
  );
}
