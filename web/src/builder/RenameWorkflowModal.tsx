import { useState } from "react";
import { renameWorkflow } from "../api";
import { Modal } from "../ui/Modal";

/**
 * Edits a workflow's display name and id. An id change cascades
 * server-side — file, draft, versions archive, run history, and
 * workflows that embed this one all follow.
 */
export function RenameWorkflowModal({
  workflowId,
  currentName,
  onRenamed,
  onClose,
}: {
  workflowId: string;
  currentName: string;
  onRenamed: (workflowId: string) => void;
  onClose: () => void;
}) {
  const [name, setName] = useState(currentName);
  const [id, setId] = useState(workflowId);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const nameChanged = name.trim() !== currentName && name.trim() !== "";
  const idChanged = id.trim() !== workflowId && id.trim() !== "";
  const dirty = nameChanged || idChanged;

  async function save() {
    if (!dirty || saving) {
      onClose();
      return;
    }
    setSaving(true);
    setError("");
    try {
      const response = await renameWorkflow(workflowId, {
        newId: idChanged ? id.trim() : undefined,
        name: nameChanged ? name.trim() : undefined,
      });
      onRenamed(response.workflow_id);
    } catch (caught) {
      setError(
        caught instanceof Error ? caught.message : String(caught),
      );
      setSaving(false);
    }
  }

  return (
    <Modal
      title="Rename workflow"
      onClose={onClose}
      onSubmit={(event) => {
        event.preventDefault();
        void save();
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
            disabled={!dirty || saving}
          >
            {saving ? "Renaming…" : "Rename"}
          </button>
        </>
      }
    >
      <div className="builder-field-row">
        <label htmlFor="rename-workflow-name">Name</label>
        <input
          id="rename-workflow-name"
          className="builder-input"
          autoFocus
          value={name}
          onChange={(event) => setName(event.target.value)}
        />
      </div>
      <div className="builder-field-row">
        <label htmlFor="rename-workflow-id">Id</label>
        <input
          id="rename-workflow-id"
          className="builder-input mono"
          value={id}
          onChange={(event) => setId(event.target.value)}
        />
        <small className="task-picker__hint">
          Changing the id also updates the file, run history, the
          versions archive, and workflows that embed this one.
        </small>
      </div>
      {error && <div className="builder-field-error">{error}</div>}
    </Modal>
  );
}
