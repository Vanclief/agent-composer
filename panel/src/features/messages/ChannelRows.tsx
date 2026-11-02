// The Talk group's channel list under Channels, mounted by the shell
// (src/shell/layout.tsx). Only followed channels (follow.ts): Inbox (#<me>)
// pinned first, even before it has a message; then Direct (my DMs,
// @<me>/<task>), then Tasks (channels named after a task), then Other; last
// "Browse channels…" to every channel. Each row has its unread badge, the
// questions waiting for me (QuestionBadge), and is bold while unread.
import { Compass, Lock } from 'lucide-react'
import { NavLink } from 'react-router'

import { useParticipants, useTasks } from '@/api/hooks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import { channelPath, dmTask, useFollow } from './follow'
import { RowBadges } from './QuestionBadge'
import { useOpenQuestions } from './questions'
import { type ChannelUnread } from './unread'

function rowClass(isActive: boolean, unread: boolean) {
  return cn(
    'flex items-center gap-1.5 rounded-md border px-2 py-1.5 text-sm transition-colors',
    isActive
      ? cn('font-medium', conceptColors.message.selected)
      : 'border-transparent text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800/60 dark:hover:text-zinc-100',
    unread && 'font-semibold text-zinc-900 dark:text-white',
  )
}

function ChannelRow({
  channel,
  readOnly,
  unread,
  questions,
}: {
  channel: string
  readOnly: boolean
  unread: ChannelUnread
  questions: number
}) {
  const name = channel.replace(/^#/, '')
  return (
    <li>
      <NavLink
        to={channelPath(channel)}
        title={readOnly ? 'Read only' : undefined}
        data-channel={channel}
        data-unread={unread.count > 0}
        className={({ isActive }) => rowClass(isActive, unread.count > 0)}
      >
        {readOnly && <Lock aria-label="Read only" className="size-3.5 shrink-0" />}
        <span className="truncate font-mono">#{name}</span>
        <RowBadges count={unread.count} mention={unread.mention} questions={questions} />
      </NavLink>
    </li>
  )
}

// The other side of a DM: the newest participant who is not me, else the
// task's name.
function DMRow({
  channel,
  me,
  unread,
  questions,
}: {
  channel: string
  me: string
  unread: ChannelUnread
  questions: number
}) {
  const participants = useParticipants(channel)
  const task = dmTask(channel)
  const other = (participants.data ?? []).filter((p) => p.name !== me && p.name !== 'agc').at(-1)?.name
  const Icon = conceptIcons.human
  return (
    <li>
      <NavLink
        to={channelPath(channel)}
        title={`Direct with ${other ? `@${other}` : `task ${task}`}, ${channel}`}
        data-testid="dm-row"
        data-channel={channel}
        data-unread={unread.count > 0}
        className={({ isActive }) => rowClass(isActive, unread.count > 0)}
      >
        <Icon aria-hidden className={cn('size-3.5 shrink-0', conceptColors.human.text)} />
        <span className="shrink-0 font-mono">@{other ?? task}</span>
        {other && <span className="min-w-0 truncate text-xs font-normal text-muted-foreground">{task}</span>}
        <RowBadges count={unread.count} mention={unread.mention} questions={questions} />
      </NavLink>
    </li>
  )
}

function Group({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <li role="group" aria-label={label} data-testid="channel-group" className="mt-2">
      <span className="block px-2 pb-0.5 text-[11px] font-medium tracking-wide text-zinc-500 uppercase">{label}</span>
      <ul className="flex flex-col gap-0.5">{children}</ul>
    </li>
  )
}

export function ChannelRows() {
  const { unread, following } = useFollow()
  const questions = useOpenQuestions()
  const tasks = useTasks()
  const { channelsQuery, inbox, me } = unread
  const InboxIcon = conceptIcons.inbox
  const inboxUnread = unread.of(inbox)

  const taskNames = new Set((tasks.data ?? []).map((t) => `#${t.name}`))
  const rest = unread.channels.filter((c) => c.channel !== inbox && following(c.channel))
  const direct = rest.filter((c) => c.channel.startsWith(`@${me}/`))
  const taskChannels = rest.filter((c) => taskNames.has(c.channel))
  const other = rest.filter((c) => !taskNames.has(c.channel) && !c.channel.startsWith(`@${me}/`))

  return (
    <ul data-testid="channel-rows" className="mt-0.5 flex flex-col gap-0.5 pl-6">
      <li>
        <NavLink
          to={`/messages/${encodeURIComponent(unread.me)}`}
          title={`Your inbox, ${inbox}`}
          data-testid="inbox-row"
          data-channel={inbox}
          data-unread={inboxUnread.count > 0}
          className={({ isActive }) => rowClass(isActive, inboxUnread.count > 0)}
        >
          <InboxIcon aria-hidden className={cn('size-3.5 shrink-0', conceptColors.inbox.text)} />
          <span className="truncate">Inbox</span>
          <RowBadges count={inboxUnread.count} mention={inboxUnread.mention} questions={questions.of(inbox).length} />
        </NavLink>
      </li>
      {channelsQuery.isPending ? (
        <li data-testid="channel-rows-loading" className="flex flex-col gap-0.5">
          <Skeleton className="h-7" />
          <Skeleton className="h-7" />
        </li>
      ) : channelsQuery.error ? (
        <li>
          <ErrorText error={channelsQuery.error} className="px-2 py-1.5 text-xs" />
        </li>
      ) : (
        <>
          {direct.length > 0 && (
            <Group label="Direct">
              {direct.map((c) => (
                <DMRow
                  key={c.channel}
                  channel={c.channel}
                  me={me}
                  unread={unread.of(c.channel)}
                  questions={questions.of(c.channel).length}
                />
              ))}
            </Group>
          )}
          {taskChannels.length > 0 && (
            <Group label="Tasks">
              {taskChannels.map((c) => (
                <ChannelRow
                  key={c.channel}
                  channel={c.channel}
                  readOnly={c.read_only}
                  unread={unread.of(c.channel)}
                  questions={questions.of(c.channel).length}
                />
              ))}
            </Group>
          )}
          {other.length > 0 && (
            <Group label="Other">
              {other.map((c) => (
                <ChannelRow
                  key={c.channel}
                  channel={c.channel}
                  readOnly={c.read_only}
                  unread={unread.of(c.channel)}
                  questions={questions.of(c.channel).length}
                />
              ))}
            </Group>
          )}
        </>
      )}
      <li className="mt-2">
        <NavLink
          to="/messages/browse"
          data-testid="browse-link"
          className={({ isActive }) => cn(rowClass(isActive, false), !isActive && 'text-muted-foreground')}
        >
          <Compass aria-hidden className="size-3.5 shrink-0" />
          <span className="truncate">Browse channels…</span>
        </NavLink>
      </li>
    </ul>
  )
}
