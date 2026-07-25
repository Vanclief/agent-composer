import { useParams } from "react-router-dom";

export function RunsPage() {
  const { executionId } = useParams();

  return (
    <main className="route-placeholder">
      <p className="route-placeholder__eyebrow">AGC</p>
      <h1>Workflow Runs</h1>
      <p>{executionId ? `Loading ${executionId}…` : "Loading recent executions…"}</p>
    </main>
  );
}
