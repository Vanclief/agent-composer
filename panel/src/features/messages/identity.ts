// The human the panel posts as. The name lives in localStorage, and so does
// the participant id agc handed out for it: agc holds a human's name until
// it is left, so joining the same name again from a new mount or reload
// would be refused with 409. The panel never leaves on unmount for the same
// reason; it leaves only when the human switches to another name.
// With no name stored, the name is the server's owner (GET /api/defaults),
// and the constant only while the server does not say.
import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from 'react'

import { ApiError } from '@/api/client'
import { useDefaults as useServerDefaults, useJoin, useLeave } from '@/api/hooks'

const NAME_KEY = 'agc.panel.human'
const PARTICIPANT_KEY = 'agc.panel.human.participant'
const DEFAULT_NAME = 'franco'

// Joins in flight, by name, shared by every mounted view.
const inflight = new Map<string, Promise<string>>()

// The server's owner once GET /api/defaults answered; '' before.
let serverOwner = ''

// The name stored in this browser, '' when none.
export function storedName() {
  try {
    return localStorage.getItem(NAME_KEY) || ''
  } catch {
    return ''
  }
}

function readName() {
  return storedName() || serverOwner || DEFAULT_NAME
}

// Views that only need the name (unread counts, notifications) follow
// renames through these listeners, without joining.
const nameListeners = new Set<() => void>()

function subscribeName(listener: () => void) {
  nameListeners.add(listener)
  window.addEventListener('storage', listener)
  return () => {
    nameListeners.delete(listener)
    window.removeEventListener('storage', listener)
  }
}

function notifyName() {
  for (const listener of nameListeners) {
    listener()
  }
}

// GET /api/defaults, and its owner becomes the name of a browser that
// stored none. settled is true once the server answered or refused (a
// server without the route keeps the constant).
export function useDefaults() {
  const query = useServerDefaults()
  const owner = query.data?.owner ?? ''
  useEffect(() => {
    if (owner && owner !== serverOwner) {
      serverOwner = owner
      notifyName()
    }
  }, [owner])
  return { ...query, owner, settled: query.isFetched || query.isError }
}

// The signed-in person's name, without the @.
export function useMyName() {
  useDefaults()
  return useSyncExternalStore(subscribeName, readName)
}

// The stored participant id, only when it was joined under name.
function readParticipant(name: string) {
  try {
    const stored = JSON.parse(localStorage.getItem(PARTICIPANT_KEY) ?? 'null')
    if (stored && stored.name === name && typeof stored.id === 'string') {
      return stored.id as string
    }
  } catch {
    // Not ours or corrupt: join again.
  }
  return ''
}

// agc forgot the stored participant or it was left elsewhere: join again.
export function isStaleParticipant(error: unknown) {
  return error instanceof ApiError && /^participant \S+ (not found|has left)/.test(error.message)
}

export type Identity = ReturnType<typeof useIdentity>

// autoJoin false: never join on mount, only on rename or rejoin (the top
// bar's name popover).
export function useIdentity({ autoJoin = true }: { autoJoin?: boolean } = {}) {
  const name = useMyName()
  const defaults = useDefaults()
  const [participantID, setParticipantID] = useState(() => readParticipant(readName()))
  const join = useJoin()
  const leave = useLeave()
  const { mutateAsync: joinAsync } = join
  const started = useRef(false)

  // Resolves to the new participant id, or '' when the join failed (its
  // error is join.error). The promise outlives an unmount, so the id is
  // stored even if the view closed meanwhile, and a view mounted while a
  // join is in flight waits for it instead of being refused with 409.
  const joinAs = useCallback(
    (next: string) => {
      let pending = inflight.get(next)
      if (!pending) {
        pending = joinAsync(next).then(
          (p) => {
            localStorage.setItem(PARTICIPANT_KEY, JSON.stringify({ name: next, id: p.id }))
            return p.id
          },
          () => '',
        )
        inflight.set(next, pending)
        pending.finally(() => inflight.delete(next))
      }
      return pending.then((id) => {
        if (id) {
          setParticipantID(id)
        }
        return id
      })
    },
    [joinAsync],
  )

  // Join once on first use when this browser holds no participant yet;
  // without a stored name, only once the server said who its owner is.
  const ready = Boolean(storedName()) || defaults.settled
  useEffect(() => {
    if (!autoJoin || started.current || participantID || !ready) {
      return
    }
    started.current = true
    joinAs(name)
  }, [autoJoin, joinAs, name, participantID, ready])

  // Switch to another name (or retry the same one after a failed join).
  function rename(input: string) {
    const next = input.trim().replace(/^@/, '')
    if (!next || (next === name && participantID)) {
      return
    }
    if (participantID) {
      leave.mutate(participantID)
    }
    localStorage.setItem(NAME_KEY, next)
    localStorage.removeItem(PARTICIPANT_KEY)
    notifyName()
    setParticipantID('')
    joinAs(next)
  }

  function rejoin() {
    localStorage.removeItem(PARTICIPANT_KEY)
    setParticipantID('')
    return joinAs(name)
  }

  return {
    name,
    participantID,
    joining: join.isPending,
    error: join.error ?? leave.error,
    rename,
    rejoin,
  }
}

// The signed-in human's name, without joining: what the task page sends as
// `by` when accepting or rejecting (ADR 0010).
export function humanName() {
  return readName()
}
