// One attached terminal job: a strip (command, state, Detach/Reconnect) over
// an xterm bound to the job's WebSocket. The xterm lives as long as the pane
// is mounted, so a hidden pane keeps its scrollback; the socket lives while
// attached. Binary frames carry bytes both ways, resize is a text frame, and
// the server replays recent output on every attach.
import { useEffect, useRef, useState } from 'react'
import { FitAddon } from '@xterm/addon-fit'
import { Terminal as XTerm, type ITheme } from '@xterm/xterm'
import { RotateCw, Unplug } from 'lucide-react'

import { terminalSocketURL, type Terminal } from '@/api/terminals'
import { conceptIcons } from '@/components/concept-icons'
import { IconChip } from '@/components/icon-chip'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { AGENT_JOB } from './JobIcon'
import { JobShape } from './JobShape'

// The close reason the server sends when the program exits.
const JOB_COMPLETED = 'job completed'

// The light theme's terminal: zinc-950 background, zinc-200 text.
const ZINC_THEME: ITheme = {
  background: '#09090b',
  foreground: '#e4e4e7',
  cursor: '#e4e4e7',
  cursorAccent: '#09090b',
  selectionBackground: '#3f3f46',
}

// The dark look (docs/design/panel.md "Palette"), the oklch tokens in hex
// since xterm wants sRGB: the warm ground, pale text, the orange cursor, and
// the ANSI colors mapped to the system's red, green, yellow and cyan (blue a
// deeper cyan, magenta the orange); black and white are the ground and the
// text, the bright ones a step lighter.
const EVA_THEME: ITheme = {
  background: '#0a0705', // ground, oklch(0.13 0.008 60)
  foreground: '#ece3d6', // text, oklch(0.92 0.02 80)
  cursor: '#ff7e22', // --eva
  cursorAccent: '#030303', // --eva-ink
  selectionBackground: '#ff7e224d', // --eva at 30%
  black: '#1a1512',
  red: '#f90a1f', // --eva-red
  green: '#3ce074', // --eva-green
  yellow: '#fbcb00', // --eva-yellow
  blue: '#26a7d0',
  magenta: '#ff7e22', // --eva
  cyan: '#00d8e1', // --eva-cyan
  white: '#ece3d6',
  brightBlack: '#5d5449',
  brightRed: '#ff554c',
  brightGreen: '#7af69a',
  brightYellow: '#ffe362',
  brightBlue: '#5ec6ec',
  brightMagenta: '#ffa553',
  brightCyan: '#73eef4',
  brightWhite: '#f9f4ee',
}

// The theme for the page's look: dark is the `dark` class on <html>.
function themeNow() {
  return document.documentElement.classList.contains('dark') ? EVA_THEME : ZINC_THEME
}

type Connection =
  | { state: 'connecting' | 'open' | 'exited' | 'detached' }
  // Closed for any other reason; reason is the server's, verbatim.
  | { state: 'closed'; reason: string }

type TerminalPaneProps = {
  // Sandbox name or id.
  sandbox: string
  job: Terminal
  visible: boolean
  // Called once the server closes with "job completed".
  onExited: (job: string) => void
}

export function TerminalPane({ sandbox, job, visible, onExited }: TerminalPaneProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<XTerm | null>(null)
  const socketRef = useRef<WebSocket | null>(null)
  const [connection, setConnection] = useState<Connection>({ state: 'connecting' })
  const [attached, setAttached] = useState(true)
  // Bumped by Reconnect to open a fresh socket.
  const [attempt, setAttempt] = useState(0)

  // The xterm: created once, fitted to its container, input sent as bytes.
  useEffect(() => {
    const container = containerRef.current
    if (!container) {
      return
    }
    const term = new XTerm({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: '"JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, monospace',
      theme: themeNow(),
      scrollback: 5000,
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(container)
    termRef.current = term

    const fitToContainer = () => {
      // A hidden pane has no size; it fits again when shown.
      if (container.clientWidth > 0 && container.clientHeight > 0) {
        fit.fit()
      }
    }
    fitToContainer()
    const observer = new ResizeObserver(fitToContainer)
    observer.observe(container)
    // Follow the theme toggle.
    const themes = new MutationObserver(() => {
      term.options.theme = themeNow()
    })
    themes.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })

    const encoder = new TextEncoder()
    const input = term.onData((data) => {
      const socket = socketRef.current
      if (socket?.readyState === WebSocket.OPEN) {
        socket.send(encoder.encode(data))
      }
    })
    const resize = term.onResize(() => {
      const socket = socketRef.current
      if (socket) {
        sendResize(socket, term)
      }
    })

    return () => {
      observer.disconnect()
      themes.disconnect()
      input.dispose()
      resize.dispose()
      term.dispose()
      termRef.current = null
    }
  }, [])

  // The socket, while attached. Each attach starts from a clean screen
  // because the server replays the recent output.
  useEffect(() => {
    const term = termRef.current
    if (!attached || !term) {
      return
    }
    term.reset()
    const socket = new WebSocket(terminalSocketURL(sandbox, job.id))
    socket.binaryType = 'arraybuffer'
    socketRef.current = socket

    socket.onopen = () => {
      setConnection({ state: 'open' })
      sendResize(socket, term)
    }
    socket.onmessage = (event: MessageEvent<ArrayBuffer | string>) => {
      term.write(typeof event.data === 'string' ? event.data : new Uint8Array(event.data))
    }
    socket.onclose = (event) => {
      socketRef.current = null
      if (event.reason === JOB_COMPLETED) {
        setConnection({ state: 'exited' })
        onExited(job.id)
        return
      }
      setConnection({ state: 'closed', reason: event.reason || `code ${event.code}` })
    }

    return () => {
      socket.onopen = null
      socket.onmessage = null
      socket.onclose = null
      if (socketRef.current === socket) {
        socketRef.current = null
      }
      socket.close(1000, 'detached')
    }
  }, [sandbox, job.id, attached, attempt, onExited])

  useEffect(() => {
    if (visible) {
      termRef.current?.focus()
    }
  }, [visible])

  const detach = () => {
    setAttached(false)
    setConnection({ state: 'detached' })
  }
  const reconnect = () => {
    setAttached(true)
    setAttempt((n) => n + 1)
    setConnection({ state: 'connecting' })
  }

  const live = connection.state === 'connecting' || connection.state === 'open'

  return (
    <div
      data-job={job.id}
      data-connection={connection.state}
      className={cn('min-h-0 flex-1 flex-col', visible ? 'flex' : 'hidden')}
    >
      <div data-testid="terminal-strip" className="flex h-11 shrink-0 items-center gap-2 border-b border-border px-3">
        <span data-testid="terminal-title" title={`Sandbox ${sandbox}, job ${job.id}`} className="hidden">
          <span>{sandbox}</span>
          <span aria-hidden>/</span>
          <span>{job.id}</span>
        </span>
        <IconChip
          icon={job.id === AGENT_JOB ? conceptIcons.task : conceptIcons.terminal}
          concept="terminal"
          title={job.id}
          className="text-sm"
        >
          {job.command.join(' ')}
        </IconChip>
        <JobShape running={job.running} />
        {connection.state === 'exited' && <ExitedBadge />}
        {connection.state === 'closed' && (
          <span role="alert" className="truncate font-mono text-xs text-red-700 dark:text-red-300">
            {connection.reason}
          </span>
        )}
        <div className="ml-auto flex items-center gap-2">
          {live && (
            <Button variant="outline" size="sm" onClick={detach} title="Close the socket; the job keeps running">
              <Unplug />
              Detach
            </Button>
          )}
          {(connection.state === 'closed' || connection.state === 'detached') && (
            <Button variant="outline" size="sm" onClick={reconnect}>
              <RotateCw />
              Reconnect
            </Button>
          )}
        </div>
      </div>
      <div data-testid="terminal-screen" className="min-h-0 flex-1 bg-[#09090b] p-2">
        <div ref={containerRef} className="h-full w-full" />
      </div>
    </div>
  )
}

export function ExitedBadge() {
  return (
    <Badge variant="outline" className="px-1.5 py-0 text-[10px] text-zinc-500">
      exited
    </Badge>
  )
}

function sendResize(socket: WebSocket, term: XTerm) {
  if (socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify({ type: 'resize', rows: term.rows, cols: term.cols }))
  }
}
