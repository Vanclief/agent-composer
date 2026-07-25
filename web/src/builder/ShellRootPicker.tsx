import { type KeyboardEvent, useEffect, useRef, useState } from "react";
import { FolderIcon } from "./Icons";

export function ShellRootPicker({
  value,
  options,
  defaultRoot,
  onChange,
  onAddOption,
  onRemoveOption,
  disabled,
}: {
  value: string;
  options: string[];
  defaultRoot: string;
  onChange: (value: string) => void;
  onAddOption: (value: string) => void;
  onRemoveOption: (value: string) => void;
  disabled: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState("");
  const ref = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    if (!open) {
      return;
    }
    function closeOnOutside(event: MouseEvent) {
      if (
        ref.current &&
        event.target instanceof Node &&
        !ref.current.contains(event.target)
      ) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", closeOnOutside);
    return () => document.removeEventListener("mousedown", closeOnOutside);
  }, [open]);

  function commitDraft() {
    const next = draft.trim();
    if (!next) {
      return;
    }
    onAddOption(next);
    onChange(next);
    setDraft("");
    setOpen(false);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Enter") {
      event.preventDefault();
      commitDraft();
    }
  }

  const label = value || defaultRoot || "Shell root…";

  return (
    <span className="builder-runmenu-anchor" ref={ref}>
      <button
        type="button"
        className="builder-ghost-button builder-shell-root-button"
        disabled={disabled}
        title={value ? `Shell root: ${value}` : "Shell root"}
        onClick={() => setOpen((current) => !current)}
      >
        <FolderIcon />
        <span className="mono">{label}</span>
        <span className="builder-chevron">⌄</span>
      </button>
      {open && (
        <div className="builder-shell-menu">
          <div className="builder-runmenu__head">
            <span>Shell root</span>
            <span>{options.length}</span>
          </div>
          {options.map((option) => (
            <div
              key={option}
              className={`builder-shell-menu__item ${
                option === value ? "active" : ""
              }`}
            >
              <button
                type="button"
                className="builder-shell-menu__select"
                onClick={() => {
                  onChange(option);
                  setOpen(false);
                }}
              >
                <span className="builder-runmenu__status builder-runmenu__status--ok" />
                <span className="mono">{option}</span>
              </button>
              {option === defaultRoot ? (
                <small>current</small>
              ) : (
                <button
                  type="button"
                  className="builder-shell-menu__remove"
                  title="Remove"
                  aria-label={`Remove ${option}`}
                  onClick={() => {
                    onRemoveOption(option);
                  }}
                >
                  ×
                </button>
              )}
            </div>
          ))}
          <div className="builder-shell-menu__add">
            <input
              className="builder-input mono"
              placeholder="Add an absolute path"
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              onKeyDown={handleKeyDown}
            />
            <button
              type="button"
              className="builder-ghost-button"
              onClick={commitDraft}
              disabled={!draft.trim()}
            >
              Add
            </button>
          </div>
        </div>
      )}
    </span>
  );
}
