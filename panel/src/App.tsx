// Routes, relative to the /panel basename. Each feature owns its pages
// under src/features/<feature>/ (see CLAUDE.md); this file only wires them.
import { Navigate, Route, Routes } from 'react-router'

import { AgentPage } from '@/features/agents/AgentPage'
import { AgentsPage } from '@/features/agents/AgentsPage'
import { ComputePage } from '@/features/compute/ComputePage'
import { SandboxPage } from '@/features/compute/SandboxPage'
import { MessagesPage } from '@/features/messages/MessagesPage'
import { BrowsePage } from '@/features/messages/BrowsePage'
import { MessagesTab } from '@/features/messages/MessagesTab'
import { ComputeTab } from '@/features/tasks/ComputeTab'
import { TaskPage } from '@/features/tasks/TaskPage'
import { TasksPage } from '@/features/tasks/TasksPage'
import { ActionsPage, BeliefsPage, DecisionsPage, EvidencePage, RulesPage } from '@/features/think/ThinkPages'
import { Layout } from '@/shell/layout'
import { HomeRedirect } from '@/shell/areas'

export function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<HomeRedirect />} />
        <Route path="tasks" element={<TasksPage />} />
        <Route path="tasks/:name" element={<TaskPage />}>
          {/* The center box shows Messages unless the URL says compute; brief is the old tab. */}
          <Route index element={<MessagesTab />} />
          <Route path="brief" element={<Navigate to=".." replace />} />
          <Route path="compute" element={<ComputeTab />} />
          <Route path="messages" element={<MessagesTab />} />
        </Route>
        {/* Compute (ADR 0008): every sandbox; :name is a sandbox with its terminals. */}
        <Route path="compute" element={<ComputePage />} />
        <Route path="compute/:name" element={<SandboxPage />} />
        {/* Persistent agents: :name is the agent's number or kept name. */}
        <Route path="agents" element={<AgentsPage />} />
        <Route path="agents/:name" element={<AgentPage />} />
        {/* :channel is the channel name without its #: /messages/review-42 is #review-42. */}
        <Route path="messages" element={<MessagesPage />} />
        <Route path="messages/:channel" element={<MessagesPage />} />
        <Route path="messages/browse" element={<BrowsePage />} />
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
