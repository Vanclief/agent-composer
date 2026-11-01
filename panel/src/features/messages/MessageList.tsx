// PLACEHOLDER owned by the messages feature: replace freely.
import type { Message } from '@/api/msg'
import { conceptIcons } from '@/components/concept-icons'

export function MessageList({ messages }: { messages: Message[] }) {
  const Human = conceptIcons.human
  return (
    <ul className="flex flex-col gap-2 text-sm">
      {messages.map((m) => (
        <li key={m.id} className="flex gap-2">
          <span className="flex shrink-0 items-center gap-1 font-medium">
            {!m.task && <Human className="size-3.5 text-zinc-500" />}@{m.from}
          </span>
          <span className="whitespace-pre-wrap">{m.body}</span>
        </li>
      ))}
    </ul>
  )
}
