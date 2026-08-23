import { type FormEvent, useState } from "react";
import { useLaunchLocation } from "../builder/useLaunchLocation";
import { Modal } from "../ui/Modal";
import { createSession } from "./mock";
import type { Session } from "./types";

/**
 * A session is just a place to work: a project directory and, when the
 * project is a git repository, a worktree. The pickers are the same
 * ones a workflow run uses, so both surfaces agree on what exists.
 */
export function NewSessionModal({
  onCreate,
  onClose,
}: {
  onCreate: (session: Session) => void;
  onClose: () => void;
}) {
  const [name, setName] = useState("");
  const { project, worktree, locationSlot } = useLaunchLocation();

  function submit(event: FormEvent) {
    event.preventDefault();
    if (!project.trim()) {
      return;
    }
    onCreate(
      createSession({ name, project, branch: worktree }),
    );
  }

  return (
    <Modal
      title="New session"
      onClose={onClose}
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
          <button
            type="submit"
            className="builder-run-button"
            disabled={!project.trim()}
          >
            Create session
          </button>
        </>
      }
    >
      <div className="builder-field-row">
        <label htmlFor="new-session-name">Name (optional)</label>
        <input
          id="new-session-name"
          className="builder-input"
          autoFocus
          placeholder="defaults to the branch or folder name"
          value={name}
          onChange={(event) => setName(event.target.value)}
        />
      </div>
      {locationSlot}
    </Modal>
  );
}
