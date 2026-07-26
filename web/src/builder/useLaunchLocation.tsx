import { useEffect, useState } from "react";
import { fetchConfig } from "../api";
import {
  type Project,
  projectBaseName,
  ProjectPicker,
} from "./ProjectPicker";
import { WorkspacePicker } from "./WorkspacePicker";

function sanitizeProjects(value: unknown): Project[] {
  if (!Array.isArray(value)) {
    return [];
  }
  const seen = new Set<string>();
  const projects: Project[] = [];
  for (const item of value) {
    const path =
      typeof (item as Project)?.path === "string"
        ? (item as Project).path.trim()
        : "";
    if (!path || seen.has(path)) {
      continue;
    }
    seen.add(path);
    const name =
      typeof (item as Project)?.name === "string"
        ? (item as Project).name.trim()
        : "";
    projects.push({ name: name || projectBaseName(path), path });
  }
  return projects;
}

function readStoredProjects(): Project[] {
  try {
    const stored = JSON.parse(
      localStorage.getItem("agc.projects") || "null",
    ) as unknown;
    if (Array.isArray(stored)) {
      return sanitizeProjects(stored);
    }
  } catch {
    // Ignore invalid values left by an older browser session.
  }

  // Migrate the pre-naming list of bare paths.
  try {
    const legacy = JSON.parse(
      localStorage.getItem("agc.shellRoots") || "[]",
    ) as unknown;
    if (Array.isArray(legacy)) {
      return sanitizeProjects(
        legacy.map((path) => ({ name: "", path }) as Project),
      );
    }
  } catch {
    // Same: ignore.
  }
  return [];
}

/**
 * Where a run executes: a Project (named root folder, persisted
 * locally) plus — when the project is a git repository — an optional
 * Workspace (git worktree of a branch). Returns the chosen values and
 * the labeled form sections to drop into a launch modal.
 */
export function useLaunchLocation(disabled = false) {
  const [projects, setProjects] = useState<Project[]>(readStoredProjects);
  const [shellRoot, setShellRoot] = useState(
    () => localStorage.getItem("agc.shellRoot") || "",
  );
  const [defaultShellRoot, setDefaultShellRoot] = useState("");
  const [worktree, setWorktree] = useState("");

  useEffect(() => {
    let active = true;
    fetchConfig()
      .then((config) => {
        if (!active || !config.shell_root) {
          return;
        }
        setDefaultShellRoot(config.shell_root);
        setProjects((current) =>
          current.some((project) => project.path === config.shell_root)
            ? current
            : [
                {
                  name: projectBaseName(config.shell_root),
                  path: config.shell_root,
                },
                ...current,
              ],
        );
        setShellRoot((current) => current || config.shell_root);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    try {
      localStorage.setItem("agc.projects", JSON.stringify(projects));
      localStorage.setItem("agc.shellRoot", shellRoot);
    } catch {
      // Storage can be disabled without preventing workflow execution.
    }
  }, [projects, shellRoot]);

  const projectPath = shellRoot || defaultShellRoot;

  const locationSlot = (
    <>
      <div className="builder-field-row">
        <ProjectPicker
          projects={projects}
          selectedPath={projectPath}
          defaultRoot={defaultShellRoot}
          onSelect={setShellRoot}
          onAdd={(project) =>
            setProjects((current) =>
              current.some((item) => item.path === project.path)
                ? current
                : [...current, project],
            )
          }
          onRemove={(path) => {
            setProjects((current) =>
              current.filter((item) => item.path !== path),
            );
            if (shellRoot === path) {
              setShellRoot(defaultShellRoot);
            }
          }}
          disabled={disabled}
        />
      </div>
      <div className="builder-field-row">
        <WorkspacePicker
          projectPath={projectPath}
          value={worktree}
          onChange={setWorktree}
          disabled={disabled}
        />
      </div>
    </>
  );

  return { shellRoot, worktree, locationSlot };
}
