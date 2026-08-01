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
 * Instant best guess from the path alone, so the bar never flashes a
 * wrong value while git is being asked. Managed worktrees live at
 * worktrees/<repo>-<hash8>/<branch-dir>; anything else is a project
 * root running its checked-out branch.
 */
function guessLocation(shellRoot: string): Location {
  const managed = stripSlash(shellRoot).match(
    /\/worktrees\/(.+)-[0-9a-f]{8}\/([^/]+)$/,
  );
  if (managed) {
    const repoBase = managed[1] ?? "";
    const named = readStoredProjects().find(
      (project) => projectBaseName(project.path) === repoBase,
    );
    return {
      project: named?.name || repoBase,
      branch: managed[2] ?? "",
      worktree: true,
    };
  }
  return {
    project: projectNameFor(shellRoot),
    branch: "",
    worktree: false,
  };
}

/**
 * Where a run executes: project chip + workspace chip. Only shell_root
 * is persisted, so git resolves the rest — for worktree runs the
 * project is the main checkout and the workspace is the worktree's
 * branch; for root runs the workspace is whatever branch the checkout
 * is on.
 */
export function RunLocation({ shellRoot }: { shellRoot: string }) {
  const [location, setLocation] = useState<Location>(() =>
    guessLocation(shellRoot),
  );

  useEffect(() => {
    setLocation(guessLocation(shellRoot));
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
      <span className="run-location__seg">
        <small>Project</small>
        <span className="run-location__value">
          <FolderIcon size={12} />
          <b>{location.project}</b>
        </span>
      </span>
      {location.branch && (
        <span
          className="run-location__seg run-location__seg--branch"
          title={
            location.worktree
              ? `Workspace worktree — ${shellRoot}`
              : "Runs directly on the repository root"
          }
        >
          <small>Workspace</small>
          <span className="run-location__value">
            <BranchIcon size={12} />
            <span className="mono">{location.branch}</span>
          </span>
        </span>
      )}
    </div>
  );
}
