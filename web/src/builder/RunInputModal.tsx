import {
  type FormEvent,
  type ReactNode,
  useMemo,
  useState,
} from "react";
import type { JsonObject } from "../types/api";
import { Modal } from "../ui/Modal";
import { PlayIcon } from "./Icons";

type InputMode = "text" | "json";

function storageKey(workflowId: string) {
  return `agc.runInputs.${workflowId}`;
}

function readStoredValues(workflowId: string) {
  try {
    const raw = JSON.parse(
      localStorage.getItem(storageKey(workflowId)) || "{}",
    ) as unknown;
    if (raw && typeof raw === "object" && !Array.isArray(raw)) {
      return Object.fromEntries(
        Object.entries(raw as Record<string, unknown>).filter(
          (entry): entry is [string, string] =>
            typeof entry[1] === "string",
        ),
      );
    }
  } catch {
    // Ignore invalid values left by an older browser session.
  }
  return {};
}

export function RunInputModal({
  workflowId,
  inputDefinitions,
  title,
  headerSlot,
  locationSlot,
  onRun,
  onClose,
}: {
  workflowId: string;
  inputDefinitions: Record<string, string>;
  title?: string;
  /** Rendered above the inputs — used to pick what is being run. */
  headerSlot?: ReactNode;
  /** Project + workspace sections (self-labeled). */
  locationSlot?: ReactNode;
  onRun: (input: JsonObject) => void;
  onClose: () => void;
}) {
  const entries = useMemo(
    () => Object.entries(inputDefinitions),
    [inputDefinitions],
  );
  const [values, setValues] = useState<Record<string, string>>(() => {
    const stored = readStoredValues(workflowId);
    return Object.fromEntries(
      entries.map(([name]) => [name, stored[name] ?? ""]),
    );
  });
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
      try {
        localStorage.setItem(
          storageKey(workflowId),
          JSON.stringify(values),
        );
      } catch {
        // Storage can be disabled without preventing workflow execution.
      }
      onRun(payload);
    }
  }

  const hasLocation = Boolean(headerSlot || locationSlot);

  return (
    <Modal
      title={title || `Run ${workflowId}`}
      onClose={onClose}
      wide={hasLocation}
      onSubmit={submit}
      footer={
        <>
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
        </>
      }
    >
      <div className={hasLocation ? "launch-grid" : undefined}>
        <div className="launch-grid__col">
          {headerSlot}
          {locationSlot}
        </div>
        <div className="launch-grid__col">
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
      </div>
    </Modal>
  );
}
