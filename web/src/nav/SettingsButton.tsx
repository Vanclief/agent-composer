import {
  type FormEvent,
  useEffect,
  useState,
} from "react";
import {
  fetchHarnesses,
  fetchSettings,
  updateSettings,
} from "../api";
import { CogIcon } from "../builder/Icons";
import type { HarnessInfo } from "../types/api";
import { Modal } from "../ui/Modal";

function SettingsModal({ onClose }: { onClose: () => void }) {
  const [harnesses, setHarnesses] = useState<HarnessInfo[]>([]);
  const [harness, setHarness] = useState("");
  const [model, setModel] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    Promise.all([
      fetchSettings(controller.signal),
      fetchHarnesses(controller.signal),
    ])
      .then(([settings, harnessList]) => {
        setHarness(settings?.composer?.harness ?? "");
        setModel(settings?.composer?.model ?? "");
        setHarnesses(harnessList?.harnesses ?? []);
      })
      .catch((caught: unknown) => {
        setError(
          caught instanceof Error ? caught.message : String(caught),
        );
      })
      .finally(() => setLoading(false));
    return () => controller.abort();
  }, []);

  const knownModels =
    harnesses.find((info) => info.id === harness)?.models ?? [];

  async function save(event: FormEvent) {
    event.preventDefault();
    if (saving) {
      return;
    }
    if (harness && !model.trim()) {
      setError("Pick a model for the chosen harness.");
      return;
    }
    setSaving(true);
    setError("");
    try {
      await updateSettings({
        composer: { harness, model: model.trim() },
      });
      onClose();
    } catch (caught) {
      setError(
        caught instanceof Error ? caught.message : String(caught),
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      title="Settings"
      onClose={onClose}
      onSubmit={(event) => void save(event)}
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
            disabled={loading || saving}
          >
            {saving ? "Saving…" : "Save"}
          </button>
        </>
      }
    >
      <div className="builder-field-row">
        <label htmlFor="settings-composer-harness">
          Composer harness
        </label>
        <select
          id="settings-composer-harness"
          className="builder-select mono"
          value={harness}
          disabled={loading}
          onChange={(event) => {
            setHarness(event.target.value);
            setModel("");
          }}
        >
          <option value="">Auto — first available</option>
          {harnesses.map((info) => (
            <option key={info.id} value={info.id}>
              {info.id}
              {info.available ? "" : " (not installed)"}
            </option>
          ))}
        </select>
        <small className="task-picker__hint">
          The agent behind “Describe a change…” — it creates and edits
          workflow blueprints.
        </small>
      </div>
      {harness && (
        <div className="builder-field-row">
          <label htmlFor="settings-composer-model">
            Composer model
          </label>
          <input
            id="settings-composer-model"
            className="builder-input mono"
            list="settings-composer-models"
            value={model}
            disabled={loading}
            onChange={(event) => setModel(event.target.value)}
          />
          <datalist id="settings-composer-models">
            {knownModels.map((name) => (
              <option key={name} value={name} />
            ))}
          </datalist>
        </div>
      )}
      {error && <div className="builder-field-error">{error}</div>}
    </Modal>
  );
}

/** Gear pinned to the rail bottom — opens app settings. */
export function SettingsRailButton() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button
        type="button"
        title="Settings"
        onClick={() => setOpen(true)}
      >
        <span className="left-rail__icon">
          <CogIcon />
        </span>
        <span className="left-rail__label">Settings</span>
      </button>
      {open && <SettingsModal onClose={() => setOpen(false)} />}
    </>
  );
}
