import { useState } from "react";
import { deleteWorkflow } from "../api";
import { Modal } from "../ui/Modal";

/**
 * Confirms removing a workflow from the library. Past runs stay in
 * Monitor and the version archive is kept — only the installed file
 * and any pending draft go.
 */
export function DeleteWorkflowModal({
  workflowId,
  name,
  onDeleted,
  onClose,
}: {
  workflowId: string;
  name: string;
  onDeleted: () => void;
  onClose: () => void;
}) {
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");

  async function remove() {
    if (deleting) {
      return;
    }
    setDeleting(true);
    setError("");
    try {
      await deleteWorkflow(workflowId);
      onDeleted();
    } catch (caught) {
      setError(
        caught instanceof Error ? caught.message : String(caught),
      );
      setDeleting(false);
    }
  }

  return (
    <Modal
      title={`Delete ${name || workflowId}`}
      onClose={onClose}
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
            type="button"
            className="builder-danger-button"
            disabled={deleting}
            onClick={() => void remove()}
          >
            {deleting ? "Deleting…" : "Delete workflow"}
          </button>
        </>
      }
    >
      <p className="builder-modal__text">
        This removes <b>{name || workflowId}</b> from your library —
        the saved file and any unsaved draft. Past runs stay in
        Monitor and the version archive is kept.
      </p>
      {error && <div className="builder-field-error">{error}</div>}
    </Modal>
  );
}
