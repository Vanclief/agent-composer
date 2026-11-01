// A task's tab shows its own channel, #<task name>.
import { useParams } from 'react-router'

import { ChannelView } from './ChannelView'

export function MessagesTab() {
  const { name = '' } = useParams()

  return (
    <div className="flex h-full min-h-0 flex-col">
      <ChannelView key={name} channel={`#${name}`} />
    </div>
  )
}
