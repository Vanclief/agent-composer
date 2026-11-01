// The post box: who you post as, the draft, and one Send button. Enter
// sends, Shift+Enter is a new line.
import { useState, type KeyboardEvent } from 'react'

import { usePostMessage } from '@/api/hooks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

import { isStaleParticipant, type Identity } from './identity'

function IdentityChip({
  identity,
  editing,
  onEdit,
}: {
  identity: Identity
  editing: boolean
  onEdit: (on: boolean) => void
}) {
  const Human = conceptIcons.human
  const [draft, setDraft] = useState(identity.name)

  if (editing) {
    return (
      <form
        className="flex items-center gap-1"
        onSubmit={(e) => {
          e.preventDefault()
          identity.rename(draft)
          onEdit(false)
        }}
      >
        <Human aria-hidden className={cn('size-4', conceptColors.human.text)} />
        <Input
          autoFocus
          aria-label="Name"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') {
              onEdit(false)
            }
          }}
          placeholder="franco"
          className="h-7 w-40 font-mono text-xs"
        />
        <Button type="submit" size="xs" variant="outline">
          Join
        </Button>
      </form>
    )
  }

  const joined = Boolean(identity.participantID)
  return (
    <button
      type="button"
      onClick={() => {
        setDraft(identity.name)
        onEdit(true)
      }}
      title={joined ? 'Posting as this name. Click to change.' : 'Not joined. Click to change the name.'}
      aria-label={`Identity @${identity.name}`}
      className={cn(
        'inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 font-mono text-xs',
        joined
          ? [conceptColors.human.bg, conceptColors.human.border, conceptColors.human.text]
          : 'border-dashed border-zinc-400 text-muted-foreground dark:border-zinc-600',
        identity.joining && 'animate-pulse',
      )}
    >
      <Human aria-hidden className="size-3.5" />@{identity.name}
    </button>
  )
}

export function Composer({ channel, identity }: { channel: string; identity: Identity }) {
  const Send = conceptIcons.message
  const post = usePostMessage()
  const [draft, setDraft] = useState('')
  const [editing, setEditing] = useState(false)

  const joined = Boolean(identity.participantID)
  const empty = draft.trim() === ''
  let label = 'Send'
  if (!joined) {
    label = identity.joining ? 'Send: joining' : 'Send: join first'
  } else if (empty) {
    label = 'Send: empty'
  } else if (post.isPending) {
    label = 'Sending'
  }
  const disabled = !joined || empty || post.isPending

  async function send() {
    if (disabled) {
      return
    }
    const body = draft
    try {
      await post.mutateAsync({ participant_id: identity.participantID, channel, body })
    } catch (error) {
      if (!isStaleParticipant(error)) {
        return
      }
      const id = await identity.rejoin()
      if (!id) {
        return
      }
      try {
        await post.mutateAsync({ participant_id: id, channel, body })
      } catch {
        // Shown by post.error.
        return
      }
    }
    setDraft('')
  }

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault()
      send()
    }
  }

  return (
    <div className="flex shrink-0 flex-col gap-2 border-t border-border p-3">
      {identity.error && (
        <div className="flex items-start gap-2">
          <ErrorText error={identity.error} className="flex-1" />
          {!editing && (
            <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
              <conceptIcons.human /> Join as…
            </Button>
          )}
        </div>
      )}
      <ErrorText error={post.error} />
      <Textarea
        aria-label={`Message ${channel}`}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={onKeyDown}
        placeholder={`Message ${channel}`}
        rows={2}
        className="max-h-48 min-h-14 resize-none"
      />
      <div className="flex items-center gap-2">
        <IdentityChip key={identity.name} identity={identity} editing={editing} onEdit={setEditing} />
        <Button className={cn('ml-auto', conceptColors.message.solid)} size="sm" disabled={disabled} onClick={send}>
          <Send /> {label}
        </Button>
      </div>
    </div>
  )
}
