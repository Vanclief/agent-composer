import { useState } from "react";
import { fetchWorktrees } from "../api";
import { Modal } from "../ui/Modal";
import { FolderBrowser } from "./FolderBrowser";

export interface Project {
  name: string;
  path: string;
}

export function projectBaseName(path: string) {
  const parts = path.replace(/\/+$/, "").split("/");
  return parts[parts.length - 1] || path;
}

/** Its own dialog, stacked over the launch modal. */
function NewProjectModal({
  existing,
  onCreate,
  onClose,
}: {
  existing: Project[];
  onCreate: (project: Project) => void;
  onClose: () => void;
}) {
  const [name, setName] = useState("");
  const [path, setPath] = useState("");
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState("");

  async function submit() {
    const trimmedPath = path.trim();
    if (!trimmedPath || checking) {
      return;
    }
    if (existing.some((project) => project.path === trimmedPath)) {
      setError("That folder is already a project.");
      return;
    }
    setChecking(true);
    setError("");
    try {
      const response = await fetchWorktrees(trimmedPath);
      if (!response.exists) {
        setError("That folder does not exist on this machine.");
        return;
      }
      onCreate({
        name: name.trim() || projectBaseName(trimmedPath),
        path: trimmedPath,
      });
      onClose();
    } catch (caught) {
      setError(
        caught instanceof Error ? caught.message : String(caught),
      );
    } finally {
      setChecking(false);
    }
  }

  return (
    <Modal
      title="New project"
      onClose={onClose}
      onSubmit={(event) => {
        event.preventDefault();
        event.stopPropagation();
        void submit();
      }}
      footer={
        <>
          <button
            type="button"
            className="builder-ghost-button"
            onClick={onClose}
          >
            Cancel
          </button>
          <button
            type="submit"
            className="builder-run-button"
            disabled={!path.trim() || checking}
          >
            {checking ? "Checking…" : "Add project"}
          </button>
        </>
      }
    >
      <div className="builder-field-row">
        <div className="builder-modal__field-head">
          <label>Folder</label>
        </div>
        <FolderBrowser
          value={path}
          onChange={(next) => {
            setPath(next);
            setError("");
          }}
          disabled={checking}
        />
        {error && <div className="builder-field-error">{error}</div>}
      </div>
      <div className="builder-field-row">
        <div className="builder-modal__field-head">
          <label>
            Name <span>(optional)</span>
          </label>
        </div>
        <input
          className="builder-input"
          placeholder={
            path.trim() ? projectBaseName(path.trim()) : "My project"
          }
          value={name}
          disabled={checking}
          onChange={(event) => setName(event.target.value)}
        />
      </div>
    </Modal>
  );
}

/**
 * Project selection: named root folders where runs execute. The list
 * is inline; creating a project opens its own dialog.
 */
export function ProjectPicker({
  projects,
  selectedPath,
  defaultRoot,
  onSelect,
  onAdd,
  onRemove,
  disabled,
}: {
  projects: Project[];
  selectedPath: string;
  defaultRoot: string;
  onSelect: (path: string) => void;
  onAdd: (project: Project) => void;
  onRemove: (path: string) => void;
  disabled: boolean;
}) {
  const [showNewProject, setShowNewProject] = useState(false);
  // Collapsed by default: only the chosen project shows; expanding
  // reveals a filterable list for switching, deleting, and adding.
  const [expanded, setExpanded] = useState(false);
  const [query, setQuery] = useState("");

  const selected = projects.find(
    (project) => project.path === selectedPath,
  );
  const trimmedQuery = query.trim().toLowerCase();
  const filtered = projects.filter(
    (project) =>
      !trimmedQuery ||
      project.name.toLowerCase().includes(trimmedQuery) ||
      project.path.toLowerCase().includes(trimmedQuery),
  );

  function choose(path: string) {
    onSelect(path);
    setExpanded(false);
    setQuery("");
  }

  return (
    <div className="workspace-picker">
      <div className="builder-modal__field-head">
        <label>Project</label>
      </div>

      <div className="workspace-picker__options">
        {!expanded ? (
          <div className="workspace-picker__option workspace-picker__option--row active">
            <button
              type="button"
              disabled={disabled}
              title="Change project"
              onClick={() => setExpanded(true)}
            >
              <b>{selected?.name ?? "Choose a project…"}</b>
              <small className="mono">
                {selected?.path ?? selectedPath}
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
            <div className="branch-finder">
              <input
                className="builder-input"
                placeholder="Find a project…"
                value={query}
                autoFocus
                disabled={disabled}
                onChange={(event) => setQuery(event.target.value)}
                onKeyDown={(event) => {
                  const only = filtered[0];
                  if (
                    event.key === "Enter" &&
                    filtered.length === 1 &&
                    only
                  ) {
                    event.preventDefault();
                    choose(only.path);
                  }
                  if (event.key === "Escape") {
                    setQuery("");
                  }
                }}
              />
              <div className="branch-finder__list project-finder__list scrollnice">
                {filtered.map((project) => (
                  <div
                    key={project.path}
                    className={`workspace-picker__option workspace-picker__option--row ${
                      selectedPath === project.path ? "active" : ""
                    }`}
                  >
                    <button
                      type="button"
                      disabled={disabled}
                      onClick={() => choose(project.path)}
                    >
                      <b>{project.name}</b>
                      <small className="mono">{project.path}</small>
                    </button>
                    {project.path === defaultRoot ? (
                      <small className="workspace-picker__tag">
                        default
                      </small>
                    ) : (
                      <button
                        type="button"
                        className="workspace-picker__remove"
                        title="Remove project from this list (the folder is untouched)"
                        aria-label={`Remove project ${project.name}`}
                        disabled={disabled}
                        onClick={() => onRemove(project.path)}
                      >
                        ×
                      </button>
                    )}
                  </div>
                ))}
                {filtered.length === 0 && (
                  <div className="branch-finder__empty">
                    No matching projects
                  </div>
                )}
              </div>
            </div>

            <button
              type="button"
              className="workspace-picker__new-toggle"
              disabled={disabled}
              onClick={() => setShowNewProject(true)}
            >
              + Add project
            </button>
          </>
        )}
      </div>

      {showNewProject && (
        <NewProjectModal
          existing={projects}
          onCreate={(project) => {
            onAdd(project);
            choose(project.path);
          }}
          onClose={() => setShowNewProject(false)}
        />
      )}
    </div>
  );
}
