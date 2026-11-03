// The post box: who you post as, the draft, the images it holds, and one
// Send button. Enter sends, Shift+Enter is a new line. Drop an image on the
// box or paste one and it waits above the draft as a thumbnail with ✕;
// while a file is dragged over it the box is framed orange with
// "Drop image". Over the server's limits Send says why in its label.
import { useRef, useState, type ClipboardEvent, type DragEvent, type KeyboardEvent } from 'react'
import { X } from 'lucide-react'

import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

import { attachmentProblem, dragHasFiles, imageFiles, toUploads, useAttachments, type Attachment } from './attachments'
import type { Identity } from './identity'
import { useSend } from './send'

function Thumbnails({ attachments, onRemove }: { attachments: Attachment[]; onRemove: (key: string) => void }) {
  return (
    <ul data-testid="composer-images" aria-label="Images to send" className="flex flex-wrap gap-2">
      {attachments.map((a) => (
        <li key={a.key} data-testid="composer-image" data-name={a.file.name} className="relative size-16">
          <img src={a.url} alt={a.file.name || 'image'} className="size-16 object-cover" />
          <button
            type="button"
            aria-label={`Remove ${a.file.name || 'image'}`}
            title="Remove"
            data-testid="composer-image-remove"
            onClick={() => onRemove(a.key)}
            className="absolute top-0 right-0 flex size-5 items-center justify-center bg-black/70 text-white hover:bg-black"
          >
            <X className="size-3.5" />
          </button>
        </li>
      ))}
    </ul>
  )
}

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
      data-slot="identity"
      data-joined={joined}
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

// replyTo makes it a thread's composer: it posts with reply_to, and the
// identity chip stays with the channel's composer below the feed.
export function Composer({ channel, identity, replyTo }: { channel: string; identity: Identity; replyTo?: number }) {
  const Send = conceptIcons.message
  const post = useSend(identity)
  const [draft, setDraft] = useState('')
  const [editing, setEditing] = useState(false)
  const { attachments, add, remove, clear } = useAttachments()
  const [dragging, setDragging] = useState(false)
  const [reading, setReading] = useState(false)
  const depth = useRef(0)
  const thread = replyTo !== undefined
  const box = thread ? 'Reply in thread' : `Message ${channel}`

  const joined = Boolean(identity.participantID)
  const empty = draft.trim() === '' && attachments.length === 0
  const problem = attachmentProblem(attachments)
  const busy = post.isPending || reading
  let label = 'Send'
  if (!joined) {
    label = identity.joining ? 'Send: joining' : 'Send: join first'
  } else if (empty) {
    label = 'Send: empty'
  } else if (problem) {
    label = problem
  } else if (busy) {
    label = 'Sending'
  }
  const disabled = !joined || empty || Boolean(problem) || busy

  async function send() {
    if (disabled) {
      return
    }
    let images
    if (attachments.length > 0) {
      setReading(true)
      try {
        images = await toUploads(attachments)
      } finally {
        setReading(false)
      }
    }
    const posted = await post.send({ channel, body: draft, reply_to: replyTo, images })
    if (posted) {
      setDraft('')
      clear()
    }
  }

  // The drag events of the box and everything in it: depth counts enters
  // and leaves so moving between children keeps the frame up.
  function onDragEnter(e: DragEvent<HTMLDivElement>) {
    if (!dragHasFiles(e.dataTransfer.types)) {
      return
    }
    e.preventDefault()
    depth.current += 1
    setDragging(true)
  }

  function onDragOver(e: DragEvent<HTMLDivElement>) {
    if (!dragHasFiles(e.dataTransfer.types)) {
      return
    }
    e.preventDefault()
    e.dataTransfer.dropEffect = 'copy'
  }

  function onDragLeave() {
    depth.current = Math.max(0, depth.current - 1)
    if (depth.current === 0) {
      setDragging(false)
    }
  }

  function onDrop(e: DragEvent<HTMLDivElement>) {
    if (!dragHasFiles(e.dataTransfer.types)) {
      return
    }
    e.preventDefault()
    depth.current = 0
    setDragging(false)
    add(imageFiles(e.dataTransfer.files))
  }

  // A pasted image is attached; pasted text stays the textarea's.
  function onPaste(e: ClipboardEvent<HTMLTextAreaElement>) {
    const files = imageFiles(e.clipboardData.items)
    if (files.length > 0) {
      e.preventDefault()
      add(files)
    }
  }

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault()
      send()
    }
  }

  return (
    <div
      data-slot="composer"
      data-dragging={dragging || undefined}
      onDragEnter={onDragEnter}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
      className={cn(
        'relative flex shrink-0 flex-col gap-2 border-t border-border p-3',
        dragging && 'outline-2 outline-[var(--eva)]',
      )}
    >
      {dragging && (
        <div
          data-testid="drop-label"
          aria-hidden
          className="pointer-events-none absolute inset-0 z-10 flex items-center justify-center gap-2 bg-black/60 font-semibold tracking-[0.18em] text-[var(--eva)] uppercase"
        >
          <conceptIcons.picture className="size-4" /> Drop image
        </div>
      )}
      {identity.error && !thread && (
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
      {attachments.length > 0 && <Thumbnails attachments={attachments} onRemove={remove} />}
      <Textarea
        aria-label={box}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={onKeyDown}
        onPaste={onPaste}
        placeholder={box}
        rows={2}
        className="max-h-48 min-h-14 resize-none"
      />
      <div className="flex items-center gap-2">
        {!thread && <IdentityChip key={identity.name} identity={identity} editing={editing} onEdit={setEditing} />}
        <Button
          data-act="send"
          className={cn('ml-auto', conceptColors.message.solid)}
          size="sm"
          disabled={disabled}
          onClick={send}
        >
          <Send /> {label}
        </Button>
      </div>
    </div>
  )
}
