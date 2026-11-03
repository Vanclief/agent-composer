// A message body as markdown (GFM; raw HTML is never rendered, it shows
// as the literal text, so `/reject <why>` keeps its `<why>`): code blocks in mono with a
// copy button, links in a new tab, @name and #channel as chips (a channel
// chip opens /messages/<name>). Every body shows in full: no height cap,
// no fade, no "Show more".
import { useMemo, useState, type ReactNode } from 'react'
import { Check, Copy } from 'lucide-react'
import Markdown, { type Components } from 'react-markdown'
import { Link } from 'react-router'
import remarkGfm from 'remark-gfm'

import { conceptColors, conceptIcons } from '@/components/concept-icons'
import { cn } from '@/lib/utils'

import { useMyName } from './identity'

// ---------- @name and #channel chips (a remark plugin) ----------

type MdNode = { type: string; value?: string; children?: MdNode[]; data?: object }

// Whole words only, like mentions() in unread.ts: not someone@host, not
// &#38;, not a/#b. A channel starts with a letter so "PR #42" stays text.
const CHIP_RE = /(^|[^\w@.\-/&#])([@#])([a-z][a-z0-9_-]{0,63})/g

function splitChips(text: string): MdNode[] {
  const out: MdNode[] = []
  let last = 0
  for (const match of text.matchAll(CHIP_RE)) {
    const start = (match.index ?? 0) + match[1].length
    if (start > last) {
      out.push({ type: 'text', value: text.slice(last, start) })
    }
    const kind = match[2] === '@' ? 'mention' : 'channel'
    out.push({
      type: 'agcChip',
      data: { hName: 'span', hProperties: { dataChip: kind, dataName: match[3] } },
      children: [{ type: 'text', value: match[2] + match[3] }],
    })
    last = start + 1 + match[3].length
  }
  if (last === 0) {
    return [{ type: 'text', value: text }]
  }
  if (last < text.length) {
    out.push({ type: 'text', value: text.slice(last) })
  }
  return out
}

function walk(node: MdNode) {
  // Link text stays the link's; code is inlineCode/code, never text.
  if (!node.children || node.type === 'link' || node.type === 'linkReference') {
    return
  }
  node.children = node.children.flatMap((child) => {
    if (child.type === 'text') {
      return splitChips(child.value ?? '')
    }
    walk(child)
    return [child]
  })
}

function remarkChips() {
  return (tree: MdNode) => walk(tree)
}

const REMARK = [remarkGfm, remarkChips]

// ---------- code blocks ----------

type HastNode = { type: string; value?: string; children?: HastNode[] }

function textOf(node: HastNode | undefined): string {
  if (!node) {
    return ''
  }
  if (node.type === 'text') {
    return node.value ?? ''
  }
  return (node.children ?? []).map(textOf).join('')
}

function CodeBlock({ text, children }: { text: string; children: ReactNode }) {
  const [copied, setCopied] = useState(false)
  async function copy() {
    try {
      await navigator.clipboard.writeText(text.replace(/\n$/, ''))
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      // Clipboard refused (insecure origin): the text is still selectable.
    }
  }
  const Icon = copied ? Check : Copy
  return (
    <div data-testid="code-block" className="group/code relative my-1.5">
      <pre className="overflow-x-auto rounded-md border border-border bg-zinc-100 px-3 py-2 font-mono text-xs leading-5 dark:bg-zinc-900 [&>code]:bg-transparent [&>code]:p-0">
        {children}
      </pre>
      <button
        type="button"
        onClick={copy}
        aria-label={copied ? 'Copied' : 'Copy code'}
        title={copied ? 'Copied' : 'Copy code'}
        className="absolute top-1.5 right-1.5 grid size-6 place-items-center rounded border border-border bg-background text-muted-foreground opacity-0 transition-opacity group-hover/code:opacity-100 hover:text-foreground focus-visible:opacity-100"
      >
        <Icon aria-hidden className="size-3.5" />
      </button>
    </div>
  )
}

// ---------- chips ----------

function MentionChip({ name, me }: { name: string; me: boolean }) {
  return (
    <span
      data-testid="mention-chip"
      data-me={me}
      title={me ? 'Mentions you' : `@${name}`}
      className={cn(
        'rounded px-1 font-mono text-[0.9em] font-medium',
        me ? 'bg-pink-600 text-white' : [conceptColors.human.bg, conceptColors.human.text],
      )}
    >
      @{name}
    </span>
  )
}

function ChannelChip({ name }: { name: string }) {
  const Icon = conceptIcons.channel
  return (
    <Link
      to={`/messages/${encodeURIComponent(name)}`}
      data-testid="channel-chip"
      title={`Open #${name}`}
      className={cn(
        'inline-flex items-baseline gap-0.5 rounded px-1 font-mono text-[0.9em] font-medium hover:brightness-125',
        conceptColors.channel.bg,
        conceptColors.channel.text,
      )}
    >
      <Icon aria-hidden className="size-3 self-center" />
      {name}
    </Link>
  )
}

function components(me: string): Components {
  return {
    p: ({ children }) => <p className="my-1 break-words whitespace-pre-wrap first:mt-0 last:mb-0">{children}</p>,
    a: ({ href, children }) => (
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        className="text-blue-700 underline underline-offset-2 hover:brightness-125 dark:text-blue-400"
      >
        {children}
      </a>
    ),
    // No remote images in chat: the alt text links to the image.
    img: ({ src, alt }) => (
      <a
        href={typeof src === 'string' ? src : undefined}
        target="_blank"
        rel="noopener noreferrer"
        className="text-blue-700 underline underline-offset-2 dark:text-blue-400"
      >
        {alt || 'image'}
      </a>
    ),
    span: ({ node, children, ...props }) => {
      const data = props as Record<string, unknown>
      const name = String(data['data-name'] ?? '')
      if (data['data-chip'] === 'mention') {
        return <MentionChip name={name} me={name === me} />
      }
      if (data['data-chip'] === 'channel') {
        return <ChannelChip name={name} />
      }
      void node
      return <span {...props}>{children}</span>
    },
    pre: ({ node, children }) => <CodeBlock text={textOf(node as unknown as HastNode)}>{children}</CodeBlock>,
    code: ({ children, className }) => (
      <code className={cn('rounded bg-zinc-500/15 px-1 py-px font-mono text-[0.85em]', className)}>{children}</code>
    ),
    h1: ({ children }) => <h3 className="mt-2 mb-1 text-base font-semibold first:mt-0">{children}</h3>,
    h2: ({ children }) => <h4 className="mt-2 mb-1 text-sm font-semibold first:mt-0">{children}</h4>,
    h3: ({ children }) => <h5 className="mt-2 mb-1 text-sm font-semibold first:mt-0">{children}</h5>,
    h4: ({ children }) => <h6 className="mt-2 mb-1 text-sm font-medium first:mt-0">{children}</h6>,
    ul: ({ children }) => <ul className="my-1 list-disc pl-5">{children}</ul>,
    ol: ({ children }) => <ol className="my-1 list-decimal pl-5">{children}</ol>,
    li: ({ children }) => <li className="break-words whitespace-pre-wrap">{children}</li>,
    blockquote: ({ children }) => (
      <blockquote className="my-1 border-l-2 border-zinc-300 pl-2 text-muted-foreground dark:border-zinc-700">
        {children}
      </blockquote>
    ),
    hr: () => <hr className="my-2 border-border" />,
    table: ({ children }) => (
      <div className="my-1 overflow-x-auto">
        <table className="border-collapse text-xs">{children}</table>
      </div>
    ),
    th: ({ children }) => <th className="border border-border px-2 py-1 text-left font-semibold">{children}</th>,
    td: ({ children }) => <td className="border border-border px-2 py-1">{children}</td>,
  }
}

// The markdown alone, unclamped.
export function MarkdownText({ body, className }: { body: string; className?: string }) {
  const me = useMyName()
  const parts = useMemo(() => components(me), [me])
  return (
    <div className={cn('min-w-0 text-sm leading-5', className)}>
      <Markdown remarkPlugins={REMARK} components={parts}>
        {body}
      </Markdown>
    </div>
  )
}

// A message body, always in full (Talk, threads, question cards).
export function MessageBody({ body, className }: { body: string; className?: string }) {
  return (
    <div data-testid="message-body" className="min-w-0">
      <MarkdownText body={body} className={className} />
    </div>
  )
}
