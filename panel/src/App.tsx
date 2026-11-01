// Routes, relative to the /panel basename. Each feature owns its pages
// under src/features/<feature>/ (see CLAUDE.md); this file only wires them.
import { Navigate, Route, Routes } from 'react-router'

import { MessagesPage } from '@/features/messages/MessagesPage'
import { MessagesTab } from '@/features/messages/MessagesTab'
import { SpecTab } from '@/features/tasks/SpecTab'
import { TaskPage } from '@/features/tasks/TaskPage'
import { TasksPage } from '@/features/tasks/TasksPage'
import { TerminalsTab } from '@/features/terminals/TerminalsTab'
import { Layout } from '@/shell/layout'

export function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Navigate to="/tasks" replace />} />
        <Route path="tasks" element={<TasksPage />} />
        <Route path="tasks/:name" element={<TaskPage />}>
          <Route index element={<Navigate to="terminals" replace />} />
          <Route path="terminals" element={<TerminalsTab />} />
          <Route path="messages" element={<MessagesTab />} />
          <Route path="spec" element={<SpecTab />} />
        </Route>
        {/* :channel is the channel name without its #: /messages/review-42 is #review-42. */}
        <Route path="messages" element={<MessagesPage />} />
        <Route path="messages/:channel" element={<MessagesPage />} />
        <Route path="*" element={<Navigate to="/tasks" replace />} />
      </Route>
    </Routes>
  )
}
