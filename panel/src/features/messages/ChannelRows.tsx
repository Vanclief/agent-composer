// The Talk group's channel list, mounted by the shell (src/shell/layout.tsx).
// Only followed channels (follow.ts): Asks (my inbox #<me>, where questions
// waiting on me arrive) pinned first, even before it has a message; then DMs
// (each agent's inbox #<agent> as @<agent> with its state light, and my DMs
// @<me>/<task>, newest activity first); then Channels (task channels first,
// then the rest, at most ROW_LIMIT, then one "N more…" row to Browse), and
// "Browse channels…" last. Each row has its unread badge, the questions
// waiting for me (QuestionBadge), and is bold while unread. The Asks row
// counts every open question for me, wherever it was asked (franco,
// 2026-11-02), plus the inbox's unread.
import { Compass, Lock, MoreHorizontal } from 'lucide-react'
import { NavLink } from 'react-router'

import { useAgents, useParticipants, useTasks } from '@/api/hooks'
import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { ErrorText } from '@/components/error-text'
import { StatusLed } from '@/components/status-pill'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import { channelPath, dmTask, useFollow } from './follow'
import { RowBadges } from './QuestionBadge'
import { useOpenQuestions } from './questions'
import { type ChannelUnread } from './unread'

// The most followed channels Channels lists before its "N more…" row.
export const ROW_LIMIT = 8

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

// A persistent agent's inbox #<agent> (ADR 0016), as a DM with it: its
// state light (the word verbatim from the backend), then @<agent>.
function AgentDMRow({
  channel,
  state,
  unread,
  questions,
}: {
  channel: string
  state: string
  unread: ChannelUnread
  questions: number
}) {
  const agent = channel.replace(/^#/, '')
  return (
    <li>
      <NavLink
        to={channelPath(channel)}
        title={`Direct with @${agent} (${state}), ${channel}`}
        data-testid="agent-dm-row"
        data-channel={channel}
        data-agent={agent}
        data-unread={unread.count > 0}
        className={({ isActive }) => rowClass(isActive, unread.count > 0)}
      >
        <span className="flex size-3.5 shrink-0 items-center justify-center">
          <StatusLed status={state} />
        </span>
        <span className="truncate font-mono">@{agent}</span>
        <RowBadges count={unread.count} mention={unread.mention} questions={questions} />
      </NavLink>
    </li>
  )
}

// The followed channels past ROW_LIMIT, one dim row to Browse.
function MoreRow({ count }: { count: number }) {
  return (
    <li>
      <NavLink
        to="/messages/browse"
        data-testid="more-row"
        title={`${count} more followed channels in Browse`}
        className={cn(rowClass(false, false), 'text-muted-foreground')}
      >
        <MoreHorizontal aria-hidden className="size-3.5 shrink-0" />
        <span className="truncate">{count} more…</span>
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
  const agents = useAgents()
  const { channelsQuery, inbox, me } = unread
  const AsksIcon = conceptIcons.question
  const inboxUnread = unread.of(inbox)

  const taskNames = new Set((tasks.data ?? []).map((t) => `#${t.name}`))
  const agentStates = new Map((agents.data ?? []).map((a) => [`#${a.name}`, a.state]))
  // Newest activity first: message ids grow across every channel.
  const rest = unread.channels
    .filter((c) => c.channel !== inbox && following(c.channel))
    .sort((a, b) => b.last_id - a.last_id)
  const isDM = (channel: string) => channel.startsWith(`@${me}/`) || agentStates.has(channel)
  const dms = rest.filter((c) => isDM(c.channel))
  const channels = [
    ...rest.filter((c) => !isDM(c.channel) && taskNames.has(c.channel)),
    ...rest.filter((c) => !isDM(c.channel) && !taskNames.has(c.channel)),
  ]

  return (
    <ul data-testid="channel-rows" className="mt-0.5 flex flex-col gap-0.5 pl-6">
      <li>
        <NavLink
          to={`/messages/${encodeURIComponent(unread.me)}`}
          title={`Asks: questions and escalations waiting on you, ${inbox}`}
          data-testid="inbox-row"
          data-channel={inbox}
          data-unread={inboxUnread.count > 0}
          className={({ isActive }) => rowClass(isActive, inboxUnread.count > 0)}
        >
          <AsksIcon aria-hidden className={cn('size-3.5 shrink-0', conceptColors.question.text)} />
          <span className="truncate">Asks</span>
          <RowBadges count={inboxUnread.count} mention={inboxUnread.mention} questions={questions.total} />
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
        dms.length > 0 && (
          <Group label="DMs">
            {dms.map((c) => {
              const state = agentStates.get(c.channel)
              return state !== undefined ? (
                <AgentDMRow
                  key={c.channel}
                  channel={c.channel}
                  state={state}
                  unread={unread.of(c.channel)}
                  questions={questions.of(c.channel).length}
                />
              ) : (
                <DMRow
                  key={c.channel}
                  channel={c.channel}
                  me={me}
                  unread={unread.of(c.channel)}
                  questions={questions.of(c.channel).length}
                />
              )
            })}
          </Group>
        )
      )}
      <Group label="Channels">
        {channelsQuery.isSuccess &&
          channels
            .slice(0, ROW_LIMIT)
            .map((c) => (
              <ChannelRow
                key={c.channel}
                channel={c.channel}
                readOnly={c.read_only}
                unread={unread.of(c.channel)}
                questions={questions.of(c.channel).length}
              />
            ))}
        {channelsQuery.isSuccess && channels.length > ROW_LIMIT && <MoreRow count={channels.length - ROW_LIMIT} />}
        <li>
          <NavLink
            to="/messages/browse"
            data-testid="browse-link"
            className={({ isActive }) => cn(rowClass(isActive, false), !isActive && 'text-muted-foreground')}
          >
            <Compass aria-hidden className="size-3.5 shrink-0" />
            <span className="truncate">Browse channels…</span>
          </NavLink>
        </li>
      </Group>
    </ul>
  )
}
