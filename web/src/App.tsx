import { Navigate, Route, Routes } from "react-router-dom";
import { BuilderPage } from "./builder/BuilderPage";
import { RunsPage } from "./runs/RunsPage";

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<BuilderPage />} />
      <Route path="/workflow" element={<BuilderPage />} />
      <Route path="/workflow/:workflowId" element={<BuilderPage />} />
      <Route path="/runs" element={<RunsPage />} />
      <Route path="/runs/:executionId" element={<RunsPage />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
