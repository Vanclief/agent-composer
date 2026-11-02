// The shape a reviewer asks in (sibling task inbox-questions-only):
//
//   Accept <task>?
//   <What it does for the owner, one sentence.>
//   Changes: <UI|breaking|UI and breaking>: <one short line>
//   Asks: <one open question> | nothing
//   Diff: <compare link>
//
// The first line is the title; "Changes:", "Asks:" and "Diff:" lines are
// labelled rows; a line that is only a URL is a diff link. Everything else
// is free text, rendered as markdown like any message, so older free-form
// questions read as before. Lines inside a code fence are always text.

export const ROW_LABELS = ['Changes', 'Asks', 'Diff'] as const

export type RowLabel = (typeof ROW_LABELS)[number]

export type QuestionPart =
  | { type: 'text'; text: string }
  | { type: 'row'; label: RowLabel; value: string; url?: string }
  | { type: 'link'; url: string }

export type QuestionShape = {
  title: string
  parts: QuestionPart[]
  // The task an "Accept <task>?" title names, else ''.
  accept: string
}

const ROW_RE = new RegExp(`^(${ROW_LABELS.join('|')}):\\s*(.*)$`)
// A bare URL, <url>, or a markdown link [text](url) standing alone.
const URL_RE = /^(?:<(https?:\/\/\S+)>|(https?:\/\/\S+)|\[[^\]]*\]\((https?:\/\/[^)\s]+)\))$/
const ACCEPT_RE = /^Accept\s+[`#]?([a-z0-9][a-z0-9_-]{0,63})`?\s*\?$/
const FENCE_RE = /^\s*(```|~~~)/

// The URL a value is made of, or '' when it is anything else.
export function urlOf(value: string) {
  const match = value.trim().match(URL_RE)
  return match ? (match[1] ?? match[2] ?? match[3]) : ''
}

// The task an "Accept <task>?" title names, else ''.
export function acceptTask(title: string) {
  return title.trim().match(ACCEPT_RE)?.[1] ?? ''
}

export function parseQuestion(body: string): QuestionShape {
  const lines = body.replace(/\r\n/g, '\n').split('\n')
  while (lines.length > 0 && lines[0].trim() === '') {
    lines.shift()
  }
  let title = ''
  if (lines.length > 0 && !FENCE_RE.test(lines[0])) {
    title = (lines.shift() ?? '').trim()
  }

  const parts: QuestionPart[] = []
  let text: string[] = []
  let fenced = false
  const flush = () => {
    const joined = text.join('\n').trim()
    if (joined) {
      parts.push({ type: 'text', text: joined })
    }
    text = []
  }

  for (const line of lines) {
    if (FENCE_RE.test(line)) {
      fenced = !fenced
      text.push(line)
      continue
    }
    if (fenced) {
      text.push(line)
      continue
    }
    const row = line.trim().match(ROW_RE)
    if (row) {
      flush()
      const value = row[2].trim()
      const url = urlOf(value)
      parts.push({ type: 'row', label: row[1] as RowLabel, value, url: url || undefined })
      continue
    }
    const url = urlOf(line)
    if (url) {
      flush()
      parts.push({ type: 'link', url })
      continue
    }
    text.push(line)
  }
  flush()

  return { title, parts, accept: acceptTask(title) }
}
