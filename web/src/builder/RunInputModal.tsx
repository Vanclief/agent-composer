import { type FormEvent, useMemo, useState } from "react";
import type { JsonObject } from "../types/api";
import { PlayIcon } from "./Icons";

type InputMode = "text" | "json";

export function RunInputModal({
  workflowId,
  inputDefinitions,
  onRun,
  onClose,
}: {
  workflowId: string;
  inputDefinitions: Record<string, string>;
  onRun: (input: JsonObject) => void;
  onClose: () => void;
}) {
  const entries = useMemo(
    () => Object.entries(inputDefinitions),
    [inputDefinitions],
  );
  const [values, setValues] = useState<Record<string, string>>(() =>
    Object.fromEntries(entries.map(([name]) => [name, ""])),
  );
  const [modes, setModes] = useState<Record<string, InputMode>>(() =>
    Object.fromEntries(
      entries.map(([name, type]) => [
        name,
        type === "string" ? "text" : "json",
      ]),
    ),
  );
  const [errors, setErrors] = useState<Record<string, string>>({});

  function submit(event: FormEvent) {
    event.preventDefault();
    const payload: JsonObject = {};
    const nextErrors: Record<string, string> = {};

    for (const [name] of entries) {
      const raw = values[name] ?? "";
      if (modes[name] === "json") {
        try {
          payload[name] = JSON.parse(raw) as unknown;
        } catch (caught) {
          nextErrors[name] = `Invalid JSON: ${
            caught instanceof Error ? caught.message : String(caught)
          }`;
        }
      } else {
        payload[name] = raw;
      }
    }

    setErrors(nextErrors);
    if (Object.keys(nextErrors).length === 0) {
      onRun(payload);
    }
  }

  return (
    <div className="builder-modal-overlay" onMouseDown={onClose}>
      <form
        className="builder-modal"
        onMouseDown={(event) => event.stopPropagation()}
        onSubmit={submit}
      >
        <div className="builder-modal__head">
          <h3>Run {workflowId}</h3>
          <button
            type="button"
            className="builder-icon-button"
            onClick={onClose}
            aria-label="Close"
          >
            ×
          </button>
        </div>
        <div className="builder-modal__body scrollnice">
          {entries.length === 0 && (
            <div className="builder-modal__empty">
              This workflow has no inputs.
            </div>
          )}
          {entries.map(([name, type]) => (
            <div key={name} className="builder-field-row">
              <div className="builder-modal__field-head">
                <label>
                  {name} <span>({type})</span>
                </label>
                <select
                  className="builder-select"
                  value={modes[name]}
                  onChange={(event) =>
                    setModes((current) => ({
                      ...current,
                      [name]: event.target.value as InputMode,
                    }))
                  }
                >
                  <option value="text">Text</option>
                  <option value="json">JSON</option>
                </select>
              </div>
              <textarea
                className="builder-textarea"
                rows={modes[name] === "json" ? 6 : 4}
                placeholder={
                  modes[name] === "json"
                    ? `Enter ${name} as JSON…`
                    : `Enter ${name}…`
                }
                value={values[name] ?? ""}
                onChange={(event) => {
                  setValues((current) => ({
                    ...current,
                    [name]: event.target.value,
                  }));
                  setErrors((current) => {
                    if (!current[name]) {
                      return current;
                    }
                    const next = { ...current };
                    delete next[name];
                    return next;
                  });
                }}
              />
              {errors[name] && (
                <div className="builder-field-error">{errors[name]}</div>
              )}
            </div>
          ))}
        </div>
        <div className="builder-modal__foot">
          <button
            type="button"
            className="builder-ghost-button"
            onClick={onClose}
          >
            Cancel
          </button>
          <button type="submit" className="builder-run-button">
            <PlayIcon /> Run
          </button>
        </div>
      </form>
    </div>
  );
}
