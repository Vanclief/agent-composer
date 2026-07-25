import { useParams } from "react-router-dom";

export function BuilderPage() {
  const { workflowId } = useParams();

  return (
    <main className="route-placeholder">
      <p className="route-placeholder__eyebrow">AGC</p>
      <h1>Agent Workflow Builder</h1>
      <p>{workflowId ? `Loading ${workflowId}…` : "Loading workflows…"}</p>
    </main>
  );
}
