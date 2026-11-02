// Times in the message list: relative in headers ("3 min ago", the exact
// time on hover), a clock in the gutter, and a label per day separator.
import { useEffect, useState } from 'react'

const MINUTE = 60_000
const HOUR = 60 * MINUTE

export function clock(iso: string) {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

export function exact(iso: string) {
  return new Date(iso).toLocaleString()
}

export function relative(iso: string, now: number) {
  const age = now - Date.parse(iso)
  if (age < MINUTE) {
    return 'just now'
  }
  if (age < HOUR) {
    return `${Math.floor(age / MINUTE)} min ago`
  }
  if (age < 24 * HOUR) {
    return `${Math.floor(age / HOUR)} h ago`
  }
  const at = new Date(iso)
  return at.toLocaleDateString([], { month: 'short', day: 'numeric' }) + ' ' + clock(iso)
}

// The local calendar day, as a key.
export function dayKey(iso: string) {
  const d = new Date(iso)
  return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`
}

export function dayLabel(iso: string, now: number) {
  const key = dayKey(iso)
  if (key === dayKey(new Date(now).toISOString())) {
    return 'Today'
  }
  if (key === dayKey(new Date(now - 24 * HOUR).toISOString())) {
    return 'Yesterday'
  }
  const d = new Date(iso)
  const sameYear = d.getFullYear() === new Date(now).getFullYear()
  return d.toLocaleDateString([], {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    year: sameYear ? undefined : 'numeric',
  })
}

// The current time, ticking every 30s so relative times stay true.
export function useNow() {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 30_000)
    return () => window.clearInterval(timer)
  }, [])
  return now
}
