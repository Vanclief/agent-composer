import {
  Navigate,
  Route,
  Routes,
  useParams,
} from "react-router-dom";
import { BuilderPage } from "./builder/BuilderPage";
import { DebugMode } from "./debug/DebugMode";
import { WorkflowsMonitor } from "./workflows/WorkflowsMonitor";
import { MainMonitorPage } from "./monitor/MainMonitorPage";
import { WorkflowRedirect } from "./workflows/WorkflowRedirect";

/** Old per-task monitor links carried the execution in the path. */
function LegacyTaskRedirect() {
  const { executionId = "" } = useParams();
  return (
    <Navigate
      to={`/executions/${encodeURIComponent(executionId)}`}
      replace
    />
  );
}

export default function App() {
  return (
    <>
      <DebugMode />
      <Routes>
        <Route path="/" element={<MainMonitorPage view="tasks" />} />
        <Route
          path="/workflows"
          element={<MainMonitorPage view="workflows" />}
        />
        <Route
          path="/executions/:executionId"
          element={<WorkflowsMonitor />}
        />
        <Route path="/build" element={<BuilderPage />} />
        <Route
          path="/workflow"
          element={<Navigate to="/build" replace />}
        />
        <Route
          path="/workflow/:workflowId"
          element={<WorkflowRedirect />}
        />
        <Route
          path="/workflow/:workflowId/build"
          element={<BuilderPage />}
        />
        <Route
          path="/tasks/:executionId"
          element={<LegacyTaskRedirect />}
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </>
  );
}
