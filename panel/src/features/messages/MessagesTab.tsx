// PLACEHOLDER owned by the messages feature: replace freely, keep the
// export. A task's tab shows its own channel, #<task name>.
import { useParams } from 'react-router'

import { useChannelMessages } from '@/api/hooks'
import { ErrorText } from '@/components/error-text'

import { MessageList } from './MessageList'

export function MessagesTab() {
  const { name = '' } = useParams()
  const feed = useChannelMessages(`#${name}`)

  return (
    <div className="flex flex-col gap-3 p-6">
      <h3 className="text-lg font-semibold">MessagesTab</h3>
      <ErrorText error={feed.error} />
      <MessageList messages={feed.messages} />
    </div>
  )
}
