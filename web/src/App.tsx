import {
  Navigate,
  Route,
  Routes,
  useLocation,
  useParams,
} from "react-router-dom";
import { BuilderPage } from "./builder/BuilderPage";
import { DebugMode } from "./debug/DebugMode";
import { MainMonitorPage } from "./monitor/MainMonitorPage";
import { WorkflowRedirect } from "./workflows/WorkflowRedirect";

/**
 * Runs have exactly one home: the workflows monitor. Old detail URLs
 * (/executions/:id, /tasks/:id) fold into it, keeping ?node/?convo.
 */
function ExecutionRedirect() {
  const { executionId = "" } = useParams();
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  params.set("execution", executionId);
  return (
    <Navigate
      to={{ pathname: "/workflows", search: `?${params}` }}
      replace
    />
  );
}

export default function App() {
  return (
    <>
      <DebugMode />
      <Routes>
        <Route path="/" element={<Navigate to="/workflows" replace />} />
        <Route
          path="/workflows"
          element={<MainMonitorPage view="workflows" />}
        />
        <Route
          path="/workflows/:workflowId"
          element={<MainMonitorPage view="workflows" />}
        />
        <Route
          path="/workflows/:workflowId/executions/:executionId"
          element={<MainMonitorPage view="workflows" />}
        />
        <Route
          path="/workflows/:workflowId/executions/:executionId/node/:nodeId"
          element={<MainMonitorPage view="workflows" />}
        />
        <Route
          path="/workflows/:workflowId/executions/:executionId/node/:nodeId/convo"
          element={<MainMonitorPage view="workflows" />}
        />
        <Route
          path="/executions/:executionId"
          element={<ExecutionRedirect />}
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
          element={<ExecutionRedirect />}
        />
        <Route path="*" element={<Navigate to="/workflows" replace />} />
      </Routes>
    </>
  );
}
