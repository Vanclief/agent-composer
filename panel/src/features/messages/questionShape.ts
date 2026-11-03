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
// An http(s) URL, or a root-relative /api/ path on the panel's own origin
// (agc's shot urls without AGC_PUBLIC_URL), whose path ends in an image
// extension.
const IMAGE_RE = /^(?:https?:\/\/|\/api\/)[^?#\s]+\.(?:png|jpe?g|gif|webp|avif|svg)(?:[?#]\S*)?$/i
// A root-relative image path standing alone, bare or as <path>.
const LOCAL_IMAGE_RE = /^<?(\/api\/[^\s<>]+)>?$/
const FENCE_RE = /^\s*(```|~~~)/

// The URL a value is made of, or '' when it is anything else.
export function urlOf(value: string) {
  const match = value.trim().match(URL_RE)
  return match ? (match[1] ?? match[2] ?? match[3]) : ''
}

// The image URL a Before/After value is made of, or ''.
export function imageURLOf(value: string) {
  const url = urlOf(value) || (value.trim().match(LOCAL_IMAGE_RE)?.[1] ?? '')
  return isImageURL(url) ? url : ''
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
      const shot = label === 'Before' || label === 'After' ? imageURLOf(value) : ''
      const url = shot || urlOf(value)
      parts.push({ type: 'row', label, value, url: url || undefined, image: shot ? true : undefined })
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

// ---------- shots in any message ----------

// A message body cut where it has Before:/After: lines whose value is an
// image URL (agc's Delivered message ends with them): the text between as
// markdown, each run of such lines (blank lines between them allowed) as
// one row of labelled images. Lines inside a code fence are always text.
export type BodyPart = { type: 'text'; text: string } | { type: 'shots'; shots: { label: string; url: string }[] }

const SHOT_RE = /^(Before|After):\s*(.*)$/

export function splitShots(body: string): BodyPart[] {
  const parts: BodyPart[] = []
  let text: string[] = []
  let fenced = false
  const flush = () => {
    if (text.join('').trim() !== '') {
      parts.push({ type: 'text', text: text.join('\n') })
    }
    text = []
  }
  for (const line of body.replace(/\r\n/g, '\n').split('\n')) {
    if (FENCE_RE.test(line)) {
      fenced = !fenced
    }
    const match = fenced ? null : line.trim().match(SHOT_RE)
    const url = match ? imageURLOf(match[2]) : ''
    if (!match || !url) {
      text.push(line)
      continue
    }
    const last = parts.at(-1)
    if (last?.type === 'shots' && text.join('').trim() === '') {
      text = []
      last.shots.push({ label: match[1], url })
      continue
    }
    flush()
    parts.push({ type: 'shots', shots: [{ label: match[1], url }] })
  }
  flush()
  return parts
}
