import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { fetchLatestTaskForWorkflow } from "../tasks/data";

export function WorkflowRedirect() {
  const navigate = useNavigate();
  const { workflowId = "" } = useParams();
  const id = workflowId.trim();
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    let active = true;

    fetchLatestTaskForWorkflow(id, controller.signal)
      .then((task) => {
        if (!active) {
          return;
        }
        navigate(
          task
            ? `/workflows/${encodeURIComponent(
                id,
              )}/executions/${encodeURIComponent(task.id)}`
            : `/workflow/${encodeURIComponent(id)}/build`,
          { replace: true },
        );
      })
      .catch((caught: unknown) => {
        if (active) {
          setError(
            caught instanceof Error ? caught.message : String(caught),
          );
        }
      });

    return () => {
      active = false;
      controller.abort();
    };
  }, [id, navigate]);

  if (error) {
    return (
      <div className="route-placeholder">
        <p className="route-placeholder__eyebrow">Workflow</p>
        <h1>Could not resolve the latest task</h1>
        <p>{error}</p>
        <p>
          <Link to={`/workflow/${encodeURIComponent(id)}/build`}>
            Open Build
          </Link>
        </p>
      </div>
    );
  }

  return (
    <div className="route-placeholder">
      <p className="route-placeholder__eyebrow">Workflow</p>
      <h1>Opening workflow…</h1>
    </div>
  );
}
