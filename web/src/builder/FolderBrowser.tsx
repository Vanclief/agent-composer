import {
  type KeyboardEvent,
  useEffect,
  useState,
} from "react";
import { browseDirectories } from "../api";
import type { DirectoryEntry } from "../types/api";
import { FolderIcon } from "./Icons";

/**
 * Server-backed folder navigation — the browser sandbox cannot reveal
 * absolute paths, but the AGC server runs on the same machine.
 * `value` is the currently selected absolute path.
 */
export function FolderBrowser({
  value,
  onChange,
  disabled,
}: {
  value: string;
  onChange: (path: string) => void;
  disabled: boolean;
}) {
  // The path being listed; "" asks the server for the home directory.
  const [browsePath, setBrowsePath] = useState(value);
  const [draft, setDraft] = useState(value);
  const [parent, setParent] = useState("");
  const [directories, setDirectories] = useState<DirectoryEntry[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    let active = true;

    browseDirectories(browsePath, controller.signal)
      .then((response) => {
        if (!active) {
          return;
        }
        setParent(response.parent ?? "");
        setDirectories(response.directories ?? []);
        setError("");
        setDraft(response.path);
        onChange(response.path);
      })
      .catch((caught: unknown) => {
        if (active) {
          setError(
            caught instanceof Error ? caught.message : String(caught),
          );
        }
      });

    return () => {
      active = false;
      controller.abort();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [browsePath]);

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Enter") {
      event.preventDefault();
      setBrowsePath(draft.trim());
    }
  }

  return (
    <div className="folder-browser">
      <input
        className="builder-input mono"
        placeholder="/absolute/path — or browse below"
        value={draft}
        disabled={disabled}
        onChange={(event) => setDraft(event.target.value)}
        onKeyDown={handleKeyDown}
        onBlur={() => {
          const next = draft.trim();
          if (next && next !== browsePath) {
            setBrowsePath(next);
          }
        }}
      />
      {error && <div className="builder-field-error">{error}</div>}

      <div className="folder-browser__list scrollnice">
        {parent && (
          <button
            type="button"
            className="folder-browser__entry folder-browser__entry--up"
            disabled={disabled}
            onClick={() => setBrowsePath(parent)}
          >
            <FolderIcon size={13} />
            <b>..</b>
            <small className="mono">{parent}</small>
          </button>
        )}
        {directories.map((directory) => (
          <div
            key={directory.path}
            className={`folder-browser__row ${
              value === directory.path ? "active" : ""
            }`}
          >
            <button
              type="button"
              className="folder-browser__entry"
              disabled={disabled}
              onDoubleClick={() => setBrowsePath(directory.path)}
              onClick={() => {
                setDraft(directory.path);
                onChange(directory.path);
              }}
            >
              <FolderIcon size={13} />
              <b>{directory.name}</b>
              {directory.has_git && (
                <small className="folder-browser__git">git</small>
              )}
            </button>
            <button
              type="button"
              className="folder-browser__open"
              title={`Open ${directory.name}`}
              aria-label={`Open ${directory.name}`}
              disabled={disabled}
              onClick={() => setBrowsePath(directory.path)}
            >
              →
            </button>
          </div>
        ))}
        {directories.length === 0 && !error && (
          <div className="folder-browser__empty">No subfolders</div>
        )}
      </div>
    </div>
  );
}
