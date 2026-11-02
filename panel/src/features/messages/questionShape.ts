// The shape a reviewer asks in (sibling task inbox-questions-only):
//
//   Accept <task>?
//   <What it does for the owner, one sentence.>
//   Changes: <UI|breaking|UI and breaking>: <one short line>
//   Asks: <one open question> | nothing
//   Diff: <compare link>
//   Before: <image url>   (UI changes only)
//   After: <image url>
//
// The first line is the title; "Changes:", "Asks:", "Diff:", "Before:" and
// "After:" lines are labelled rows; a line that is only a URL is a diff
// link. A Before/After value that is an image URL is an image. Everything else
// is free text, rendered as markdown like any message, so older free-form
// questions read as before. Lines inside a code fence are always text.

export const ROW_LABELS = ['Changes', 'Asks', 'Diff', 'Before', 'After'] as const

export type RowLabel = (typeof ROW_LABELS)[number]

export type QuestionPart =
  | { type: 'text'; text: string }
  | { type: 'row'; label: RowLabel; value: string; url?: string; image?: boolean }
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
// An http(s) URL whose path ends in an image extension.
const IMAGE_RE = /^https?:\/\/[^?#\s]+\.(?:png|jpe?g|gif|webp|avif|svg)(?:[?#]\S*)?$/i
const FENCE_RE = /^\s*(```|~~~)/

// The URL a value is made of, or '' when it is anything else.
export function urlOf(value: string) {
  const match = value.trim().match(URL_RE)
  return match ? (match[1] ?? match[2] ?? match[3]) : ''
}

// Whether url points at an image.
export function isImageURL(url: string) {
  return IMAGE_RE.test(url)
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
      const label = row[1] as RowLabel
      const url = urlOf(value)
      const image = (label === 'Before' || label === 'After') && isImageURL(url)
      parts.push({ type: 'row', label, value, url: url || undefined, image: image || undefined })
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
