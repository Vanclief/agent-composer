// A task's tab shows its own channel, #<task name>. Opening it follows it.
import { useEffect } from 'react'
import { useParams } from 'react-router'

import { ChannelView } from './ChannelView'
import { follow } from './follow'

export function MessagesTab() {
  const { name = '' } = useParams()
  useEffect(() => {
    if (name) {
      follow(`#${name}`)
    }
  }, [name])

  return (
    <div className="flex h-full min-h-0 flex-col">
      <ChannelView key={name} channel={`#${name}`} />
    </div>
  )
}
