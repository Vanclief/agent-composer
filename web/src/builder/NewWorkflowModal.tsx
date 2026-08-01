import { useState } from "react";
import { createWorkflow } from "../api";
import { Modal } from "../ui/Modal";

/** Mirrors the server's id rule, for the live preview only. */
function previewId(name: string) {
  return name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
}

/**
 * Names a new workflow and scaffolds it as a draft. The composer and
 * inspector fill in the nodes afterwards; Save installs it.
 */
export function NewWorkflowModal({
  onCreated,
  onClose,
}: {
  onCreated: (workflowId: string) => void;
  onClose: () => void;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");

  const workflowId = previewId(name);

  async function create() {
    if (!workflowId || creating) {
      return;
    }
    setCreating(true);
    setError("");
    try {
      const response = await createWorkflow(
        name.trim(),
        description.trim(),
      );
      onCreated(response.workflow_id);
    } catch (caught) {
      setError(
        caught instanceof Error ? caught.message : String(caught),
      );
      setCreating(false);
    }
  }

  return (
    <Modal
      title="New workflow"
      onClose={onClose}
      onSubmit={(event) => {
        event.preventDefault();
        void create();
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
            disabled={!workflowId || creating}
          >
            {creating ? "Creating…" : "Create draft"}
          </button>
        </>
      }
    >
      <div className="builder-field-row">
        <label htmlFor="new-workflow-name">Name</label>
        <input
          id="new-workflow-name"
          className="builder-input"
          autoFocus
          placeholder="e.g. Release Notes Writer"
          value={name}
          onChange={(event) => setName(event.target.value)}
        />
        {workflowId && (
          <small className="task-picker__hint mono">
            id: {workflowId}
          </small>
        )}
      </div>
      <div className="builder-field-row">
        <label htmlFor="new-workflow-description">
          Description (optional)
        </label>
        <textarea
          id="new-workflow-description"
          className="builder-textarea"
          rows={3}
          placeholder="What this workflow should do — it also guides the composer agent."
          value={description}
          onChange={(event) => setDescription(event.target.value)}
        />
      </div>
      {error && <div className="builder-field-error">{error}</div>}
    </Modal>
  );
}
