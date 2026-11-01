// The human the panel posts as. The name lives in localStorage, and so does
// the participant id agc handed out for it: agc holds a human's name until
// it is left, so joining the same name again from a new mount or reload
// would be refused with 409. The panel never leaves on unmount for the same
// reason; it leaves only when the human switches to another name.
import { useCallback, useEffect, useRef, useState } from 'react'

import { ApiError } from '@/api/client'
import { useJoin, useLeave } from '@/api/hooks'

const NAME_KEY = 'agc.panel.human'
const PARTICIPANT_KEY = 'agc.panel.human.participant'
const DEFAULT_NAME = 'franco'

// Joins in flight, by name, shared by every mounted view.
const inflight = new Map<string, Promise<string>>()

function readName() {
  return localStorage.getItem(NAME_KEY) || DEFAULT_NAME
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

export function useIdentity() {
  const [name, setName] = useState(readName)
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

  // Join once on first use when this browser holds no participant yet.
  useEffect(() => {
    if (started.current || participantID) {
      return
    }
    started.current = true
    joinAs(name)
  }, [joinAs, name, participantID])

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
    setName(next)
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
