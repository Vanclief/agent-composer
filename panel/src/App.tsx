// Routes, relative to the /panel basename. Each feature owns its pages
// under src/features/<feature>/ (see CLAUDE.md); this file only wires them.
import { Navigate, Route, Routes } from 'react-router'

import { ComputePage } from '@/features/compute/ComputePage'
import { SandboxPage } from '@/features/compute/SandboxPage'
import { MessagesPage } from '@/features/messages/MessagesPage'
import { MessagesTab } from '@/features/messages/MessagesTab'
import { BriefTab } from '@/features/tasks/BriefTab'
import { ComputeTab } from '@/features/tasks/ComputeTab'
import { TaskPage } from '@/features/tasks/TaskPage'
import { TasksPage } from '@/features/tasks/TasksPage'
import { ActionsPage, BeliefsPage, DecisionsPage, EvidencePage, RulesPage } from '@/features/think/ThinkPages'
import { Layout } from '@/shell/layout'
import { HomeRedirect } from '@/shell/mode'

export function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<HomeRedirect />} />
        <Route path="tasks" element={<TasksPage />} />
        <Route path="tasks/:name" element={<TaskPage />}>
          <Route index element={<Navigate to="brief" replace />} />
          <Route path="brief" element={<BriefTab />} />
          <Route path="compute" element={<ComputeTab />} />
          <Route path="messages" element={<MessagesTab />} />
        </Route>
        {/* Compute (ADR 0008): every sandbox; :name is a sandbox with its terminals. */}
        <Route path="compute" element={<ComputePage />} />
        <Route path="compute/:name" element={<SandboxPage />} />
        {/* :channel is the channel name without its #: /messages/review-42 is #review-42. */}
        <Route path="messages" element={<MessagesPage />} />
        <Route path="messages/:channel" element={<MessagesPage />} />
        {/* Think: read-only views of the knowledge files; :id opens an item's detail. */}
        <Route path="think" element={<Navigate to="/think/beliefs" replace />} />
        <Route path="think/beliefs/:id?" element={<BeliefsPage />} />
        <Route path="think/decisions/:id?" element={<DecisionsPage />} />
        <Route path="think/rules/:id?" element={<RulesPage />} />
        <Route path="think/actions/:id?" element={<ActionsPage />} />
        <Route path="think/evidence/:id?" element={<EvidencePage />} />
        <Route path="*" element={<Navigate to="/tasks" replace />} />
      </Route>
    </Routes>
  )
}
