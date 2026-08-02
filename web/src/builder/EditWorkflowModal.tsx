import { useState } from "react";
import { renameWorkflow } from "../api";
import { Modal } from "../ui/Modal";

/**
 * Edits a workflow's identity: display name, id, and description. An
 * id change cascades server-side — file, draft, versions archive, run
 * history, and workflows that embed this one all follow.
 */
export function EditWorkflowModal({
  workflowId,
  currentName,
  currentDescription,
  onRenamed,
  onClose,
}: {
  workflowId: string;
  currentName: string;
  currentDescription: string;
  onRenamed: (workflowId: string) => void;
  onClose: () => void;
}) {
  const [name, setName] = useState(currentName);
  const [id, setId] = useState(workflowId);
  const [description, setDescription] = useState(currentDescription);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const nameChanged = name.trim() !== currentName && name.trim() !== "";
  const idChanged = id.trim() !== workflowId && id.trim() !== "";
  const descriptionChanged = description.trim() !== currentDescription;
  const dirty = nameChanged || idChanged || descriptionChanged;

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
        description: descriptionChanged
          ? description.trim()
          : undefined,
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
      title="Edit workflow"
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
            {saving ? "Saving…" : "Save"}
          </button>
        </>
      }
    >
      <div className="builder-field-row">
        <label htmlFor="edit-workflow-name">Name</label>
        <input
          id="edit-workflow-name"
          className="builder-input"
          autoFocus
          value={name}
          onChange={(event) => setName(event.target.value)}
        />
      </div>
      <div className="builder-field-row">
        <label htmlFor="edit-workflow-id">Id</label>
        <input
          id="edit-workflow-id"
          className="builder-input mono"
          value={id}
          onChange={(event) => setId(event.target.value)}
        />
        <small className="task-picker__hint">
          Changing the id also updates the file, run history, the
          versions archive, and workflows that embed this one.
        </small>
      </div>
      <div className="builder-field-row">
        <label htmlFor="edit-workflow-description">Description</label>
        <textarea
          id="edit-workflow-description"
          className="builder-textarea"
          rows={4}
          placeholder="What this workflow does — it also guides the composer agent."
          value={description}
          onChange={(event) => setDescription(event.target.value)}
        />
      </div>
      {error && <div className="builder-field-error">{error}</div>}
    </Modal>
  );
}
