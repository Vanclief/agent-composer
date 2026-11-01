// agc Tasks control panel. One Alpine component, hash routing, no build.
// Statuses are always shown exactly as the backend sends them.

const POLL_MS = 5000
const READ_LIMIT = 100
const KEEP_MESSAGES = 500
const WAIT_TIMEOUT = '25s'
const NAME_KEY = 'agc.panel.name'
const PARTICIPANT_KEY = 'agc.panel.participant'

const SPEC_EXAMPLE = `# Task spec. Only image is required; delete what you do not need.
image: agc-agent              # any OCI image works
cpus: 2
memory: 4096                  # MiB
repo: github.com/vanclief/agent-composer   # cloned to /home/agent/<repo name>
ref: master                   # branch, tag or sha
setup: |                      # runs once after the clone
  go mod download
secrets:                      # names only; NAME@host1,host2 limits where it may be sent
  - CLAUDE_CODE_OAUTH_TOKEN
  - GITHUB_TOKEN
env:
  GOFLAGS: -mod=mod
volumes:                      # NAME:DEST[:rw] or host:PATH:DEST[:rw]; read-only unless :rw
  - go-mod-cache:/home/agent/go/pkg/mod:rw
channels:                     # #<task name> is always available
  - "#releases"
  - "#alerts:read"
`

// Pill colors by status word. Unknown words get the neutral style; the
// word itself is always printed as sent.
const STATUS_STYLES = {
  running: { pill: 'border-emerald-500/30 text-emerald-700 dark:text-emerald-400', dot: 'bg-emerald-500' },
  stopped: { pill: 'border-zinc-400/40 text-zinc-600 dark:text-zinc-400', dot: 'bg-zinc-400' },
  starting: { pill: 'border-sky-500/30 text-sky-700 dark:text-sky-400', dot: 'bg-sky-500 animate-pulse' },
  stopping: { pill: 'border-sky-500/30 text-sky-700 dark:text-sky-400', dot: 'bg-sky-500 animate-pulse' },
  creating: { pill: 'border-sky-500/30 text-sky-700 dark:text-sky-400', dot: 'bg-sky-500 animate-pulse' },
  crashed: { pill: 'border-red-500/30 text-red-700 dark:text-red-400', dot: 'bg-red-500' },
  error: { pill: 'border-red-500/30 text-red-700 dark:text-red-400', dot: 'bg-red-500' },
}
const NEUTRAL_STYLE = { pill: 'border-zinc-500/40 border-dashed text-zinc-600 dark:text-zinc-300', dot: 'bg-zinc-500' }

// Live objects that must not be wrapped in Alpine's reactive proxies.
let session = null // { term, fit, ws, observer, detached }
let chatAbort = null
let chatGen = 0

function errorMessage(body, res) {
  if (body && body.error && typeof body.error === 'object' && body.error.message) return body.error.message
  if (body && typeof body.error === 'string' && body.error) return body.error
  if (body && typeof body.message === 'string' && body.message) return body.message
  return (res.status + ' ' + (res.statusText || 'request failed')).trim()
}

async function api(method, path, body, signal) {
  const opts = { method, headers: {}, signal }
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(path, opts)
  const text = await res.text()
  let data = null
  try {
    data = text ? JSON.parse(text) : null
  } catch (e) {
    data = null
  }
  if (!res.ok) {
    const err = new Error(errorMessage(data, res))
    err.status = res.status
    throw err
  }
  return data
}

const enc = encodeURIComponent

function taskPath(name) {
  return '/api/tasks/' + enc(name)
}

// Mirrors the server's title -> name rule: the name is the VM name, the
// task's #channel and the URL segment. Empty means no letters or digits.
function slugify(title) {
  let s = (title || '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
  if (/^[0-9]/.test(s)) s = 'task-' + s
  return s.slice(0, 63).replace(/-+$/, '')
}

function wsURL(path) {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return proto + '//' + location.host + path
}

function repoDir(spec) {
  const repo = spec && typeof spec.repo === 'string' ? spec.repo.trim() : ''
  if (!repo) return '/home/agent'
  const last = repo.replace(/\/+$/, '').split('/').pop().replace(/\.git$/, '')
  return last ? '/home/agent/' + last : '/home/agent'
}

function channelFromHash(part) {
  if (!part) return ''
  return part.startsWith('#') ? part : '#' + part
}

function channelHash(channel) {
  return '#/messages/' + enc(channel.replace(/^#/, ''))
}

function normalizeChannel(c) {
  if (typeof c === 'string') return { channel: c, last_id: 0, count: null, read_only: false }
  return {
    channel: c.channel || c.name || '',
    last_id: c.last_id ?? c.last_message_id ?? 0,
    count: c.count ?? null,
    read_only: !!c.read_only,
  }
}

function panel() {
  return {
    sidebarOpen: true,

    // Route
    view: 'tasks', // tasks | task | messages
    taskName: '',
    tab: 'terminals', // terminals | messages | spec
    terminalId: '',
    msgChannel: '',

    // Tasks list
    tasks: [],
    tasksLoaded: false,
    tasksError: '',
    query: '',
    busy: {}, // task name -> 'Starting…' | 'Stopping…' | 'Deleting…'
    actionError: '',

    // Task page
    task: null,
    taskError: '',
    terminals: [],
    terminalsLoaded: false,
    terminalsError: '',
    newTerm: { command: 'claude', dir: '/home/agent', busy: false, error: '' },
    termStatus: '',

    // Messages
    channels: [],
    channelsLoaded: false,
    channelsError: '',
    gotoChannel: '',
    chat: { channel: '', messages: [], cursor: 0, loading: false, error: '', draft: '', replyTo: null, posting: false, postError: '' },
    me: { name: localStorage.getItem(NAME_KEY) || 'franco', id: localStorage.getItem(PARTICIPANT_KEY) || '' },

    // Modals
    create: { open: false, title: '', yaml: SPEC_EXAMPLE, error: '', busy: false },
    specs: { list: [], loading: false, loaded: false, selected: '', confirmDelete: false, busy: false },
    // Inline "save spec" form, shared by the create modal and the Spec tab.
    specSave: { open: false, name: '', busy: false, error: '', saved: '' },
    confirmDelete: null, // task name

    init() {
      window.addEventListener('hashchange', () => this.route())
      setInterval(() => this.tick(), POLL_MS)
      this.watchIcons()
      this.route()
    },

    // Lucide swaps <i data-lucide> for <svg>. Alpine adds new <i> nodes
    // whenever a template renders, so convert them as they appear.
    watchIcons() {
      let queued = false
      const run = () => {
        queued = false
        if (document.querySelector('i[data-lucide]')) lucide.createIcons()
      }
      new MutationObserver(() => {
        if (queued) return
        queued = true
        requestAnimationFrame(run)
      }).observe(document.body, { childList: true, subtree: true })
      run()
    },

    // ---------- routing ----------

    route() {
      let parts = []
      try {
        parts = location.hash.replace(/^#\/?/, '').split('/').filter(Boolean).map(decodeURIComponent)
      } catch (e) {
        parts = []
      }

      let view = 'tasks'
      let taskName = ''
      let tab = 'terminals'
      let terminalId = ''
      let msgChannel = ''

      if (parts[0] === 'tasks' && parts[1]) {
        view = 'task'
        taskName = parts[1]
        if (parts[2] === 'messages' || parts[2] === 'spec') tab = parts[2]
        if (parts[2] === 'terminals' && parts[3]) terminalId = parts[3]
      } else if (parts[0] === 'messages') {
        view = 'messages'
        msgChannel = channelFromHash(parts[1])
      } else if (parts[0] !== 'tasks') {
        history.replaceState(null, '', '#/tasks')
      }

      // Leave whatever is no longer on screen.
      if (this.terminalId && this.terminalId !== terminalId) this.closeTerminal()
      const chatChannel = view === 'messages' ? msgChannel : view === 'task' && tab === 'messages' ? '#' + taskName : ''
      if (this.chat.channel !== chatChannel) this.stopChat()

      const taskChanged = taskName !== this.taskName
      this.view = view
      this.taskName = taskName
      this.tab = tab
      this.terminalId = terminalId
      this.msgChannel = msgChannel
      this.actionError = ''

      if (view === 'tasks') this.loadTasks()

      if (view === 'task') {
        if (taskChanged) {
          this.task = null
          this.taskError = ''
          this.terminals = []
          this.terminalsLoaded = false
          this.terminalsError = ''
          this.newTerm = { command: 'claude', dir: '/home/agent', busy: false, error: '' }
          this.resetSpecSave()
        }
        this.loadTask(taskChanged)
        if (tab === 'terminals') this.loadTerminals()
        if (terminalId && (!session || session.id !== terminalId)) this.$nextTick(() => this.openTerminal())
      }

      if (view === 'messages') this.loadChannels()
      if (chatChannel && this.chat.channel !== chatChannel) this.openChat(chatChannel)
    },

    tick() {
      if (document.hidden) return
      if (this.view === 'tasks') this.loadTasks()
      if (this.view === 'task') {
        this.loadTask(false)
        if (this.tab === 'terminals') this.loadTerminals()
      }
      if (this.view === 'messages') this.loadChannels()
    },

    get crumbs() {
      if (this.view === 'messages') {
        const c = [{ label: 'Messages', href: '#/messages' }]
        if (this.msgChannel) c.push({ label: this.msgChannel, href: channelHash(this.msgChannel) })
        return c
      }
      const c = [{ label: 'Tasks', href: '#/tasks' }]
      if (this.view === 'task') {
        const label = this.task && this.task.name === this.taskName ? this.taskTitle(this.task) : this.taskName
        c.push({ label, href: this.taskHref(this.taskName) })
        if (this.terminalId) c.push({ label: this.terminalId, href: this.terminalHref(this.terminalId) })
      }
      return c
    },

    taskHref(name, tab) {
      return '#/tasks/' + enc(name) + (tab && tab !== 'terminals' ? '/' + tab : '')
    },

    terminalHref(id) {
      return '#/tasks/' + enc(this.taskName) + '/terminals/' + enc(id)
    },

    channelHref(channel) {
      return channelHash(channel)
    },

    // ---------- status display ----------

    pillClass(status) {
      return (STATUS_STYLES[status] || NEUTRAL_STYLE).pill
    },

    dotClass(status) {
      return (STATUS_STYLES[status] || NEUTRAL_STYLE).dot
    },

    canOpen(t) {
      return t.status !== 'stopped'
    },
    canStop(t) {
      return t.status === 'running'
    },
    canStart(t) {
      return t.status === 'stopped'
    },
    canDelete(t) {
      return t.status !== 'running'
    },

    // ---------- tasks ----------

    taskTitle(t) {
      return (t && (t.title || t.name)) || ''
    },

    get filteredTasks() {
      const q = this.query.trim().toLowerCase()
      if (!q) return this.tasks
      return this.tasks.filter((t) => (t.title || '').toLowerCase().includes(q) || (t.name || '').toLowerCase().includes(q))
    },

    async loadTasks() {
      try {
        const data = await api('GET', '/api/tasks')
        this.tasks = (data && data.tasks) || []
        this.tasksError = ''
      } catch (e) {
        this.tasksError = e.message
      }
      this.tasksLoaded = true
    },

    async loadTask(reset) {
      const name = this.taskName
      try {
        const t = await api('GET', taskPath(name))
        if (name !== this.taskName) return
        const first = !this.task || reset
        this.task = t
        this.taskError = ''
        if (first) this.newTerm.dir = repoDir(t.spec)
      } catch (e) {
        if (name === this.taskName) this.taskError = e.message
      }
    },

    replaceTask(t) {
      if (!t || !t.name) return
      this.tasks = this.tasks.map((x) => (x.name === t.name ? t : x))
      if (this.task && this.task.name === t.name) this.task = t
    },

    async taskAction(name, action) {
      const labels = { start: 'Starting…', stop: 'Stopping…' }
      this.busy = { ...this.busy, [name]: labels[action] }
      this.actionError = ''
      try {
        const t = await api('POST', taskPath(name) + '/' + action)
        this.replaceTask(t)
      } catch (e) {
        this.actionError = action + ' ' + name + ': ' + e.message
      }
      const { [name]: _, ...rest } = this.busy
      this.busy = rest
      if (this.view === 'task' && this.tab === 'terminals') this.loadTerminals()
    },

    askDelete(name) {
      this.confirmDelete = name
      this.actionError = ''
    },

    async deleteTask() {
      const name = this.confirmDelete
      this.busy = { ...this.busy, [name]: 'Deleting…' }
      try {
        await api('DELETE', taskPath(name))
        this.tasks = this.tasks.filter((t) => t.name !== name)
        this.confirmDelete = null
        if (this.view === 'task' && this.taskName === name) location.hash = '#/tasks'
      } catch (e) {
        this.actionError = 'delete ' + name + ': ' + e.message
        this.confirmDelete = null
      }
      const { [name]: _, ...rest } = this.busy
      this.busy = rest
    },

    openCreate() {
      this.create = { open: true, title: '', yaml: this.create.yaml || SPEC_EXAMPLE, error: '', busy: false }
      this.specs.confirmDelete = false
      this.resetSpecSave()
      this.loadSpecs()
      this.$nextTick(() => this.$refs.createTitle && this.$refs.createTitle.focus())
    },

    closeCreate() {
      this.create.open = false
      this.resetSpecSave()
    },

    get createSlug() {
      return slugify(this.create.title)
    },

    resetCreateYAML() {
      this.create.yaml = SPEC_EXAMPLE
    },

    async submitCreate() {
      const title = this.create.title.trim()
      const name = slugify(title)
      if (!name) return
      let spec
      try {
        spec = jsyaml.load(this.create.yaml)
      } catch (e) {
        this.create.error = 'YAML: ' + e.message
        return
      }
      if (spec == null) spec = {}
      if (typeof spec !== 'object' || Array.isArray(spec)) {
        this.create.error = 'The spec must be a YAML mapping (key: value lines).'
        return
      }
      // Accept the workflow form too: a single top-level environment block.
      if (Object.keys(spec).length === 1 && spec.environment && typeof spec.environment === 'object') spec = spec.environment

      this.create.busy = true
      this.create.error = ''
      try {
        const t = await api('POST', '/api/tasks', { title, spec })
        this.closeCreate()
        this.create.yaml = SPEC_EXAMPLE
        this.specs.selected = ''
        location.hash = this.taskHref(t && t.name ? t.name : name)
      } catch (e) {
        this.create.error = e.message
      }
      this.create.busy = false
    },

    specYAML(spec) {
      if (!spec) return ''
      try {
        return jsyaml.dump(spec, { lineWidth: -1 })
      } catch (e) {
        return JSON.stringify(spec, null, 2)
      }
    },

    // ---------- saved specs ----------

    async loadSpecs() {
      this.specs.loading = true
      try {
        const data = await api('GET', '/api/specs')
        this.specs.list = (data && data.specs) || []
        this.specs.loaded = true
        if (!this.specs.list.some((s) => s.name === this.specs.selected)) this.specs.selected = ''
      } catch (e) {
        this.create.error = e.message
      }
      this.specs.loading = false
    },

    // A saved spec is only a starting point: editing the YAML afterwards
    // keeps the selection.
    pickSpec(name) {
      const s = this.specs.list.find((x) => x.name === name)
      this.specs.selected = s ? s.name : ''
      this.specs.confirmDelete = false
      this.create.yaml = s ? s.yaml : SPEC_EXAMPLE
    },

    async deleteSpec() {
      const name = this.specs.selected
      if (!name) return
      this.specs.busy = true
      this.create.error = ''
      try {
        await api('DELETE', '/api/specs/' + enc(name))
        this.specs.selected = ''
        this.specs.confirmDelete = false
        await this.loadSpecs()
      } catch (e) {
        this.create.error = e.message
      }
      this.specs.busy = false
    },

    resetSpecSave() {
      this.specSave = { open: false, name: '', busy: false, error: '', saved: '' }
    },

    openSpecSave() {
      this.resetSpecSave()
      this.specSave.open = true
      this.$nextTick(() => {
        const el = this.create.open ? this.$refs.createSpecName : this.$refs.taskSpecName
        if (el) el.focus()
      })
    },

    // Saves the create modal's YAML when the modal is open, otherwise the
    // open task's spec. Errors land in the box of whichever is on screen.
    async saveSpec() {
      const name = this.specSave.name.trim()
      if (!name || this.specSave.busy) return
      const inModal = this.create.open
      const yaml = inModal ? this.create.yaml : this.specYAML(this.task && this.task.spec)
      this.specSave.busy = true
      this.specSave.error = ''
      if (inModal) this.create.error = ''
      try {
        const s = await api('PUT', '/api/specs/' + enc(name), { yaml })
        const saved = (s && s.name) || name
        this.resetSpecSave()
        this.specSave.saved = saved
        if (inModal) {
          await this.loadSpecs()
          this.specs.selected = saved
          this.specs.confirmDelete = false
        }
      } catch (e) {
        this.specSave.busy = false
        if (inModal) this.create.error = e.message
        else this.specSave.error = e.message
      }
    },

    // ---------- terminals ----------

    async loadTerminals() {
      const name = this.taskName
      try {
        const data = await api('GET', taskPath(name) + '/terminals')
        if (name !== this.taskName) return
        this.terminals = (data && data.terminals) || []
        this.terminalsError = ''
      } catch (e) {
        if (name === this.taskName) this.terminalsError = e.message
      }
      if (name === this.taskName) this.terminalsLoaded = true
    },

    commandText(cmd) {
      if (Array.isArray(cmd)) return cmd.join(' ')
      return cmd || ''
    },

    async newTerminal(command) {
      const cmd = (command || this.newTerm.command).trim()
      if (!cmd) return
      this.newTerm.busy = true
      this.newTerm.error = ''
      try {
        const res = await api('POST', taskPath(this.taskName) + '/terminals', { command: cmd, dir: this.newTerm.dir.trim() })
        this.loadTerminals()
        if (res && res.id) location.hash = this.terminalHref(res.id)
      } catch (e) {
        this.newTerm.error = e.message
      }
      this.newTerm.busy = false
    },

    openTerminal() {
      this.closeTerminal()
      const el = this.$refs.term
      if (!el || !this.terminalId) return

      const id = this.terminalId
      const term = new Terminal({
        cursorBlink: true,
        fontSize: 13,
        fontFamily: '"JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, monospace',
        theme: { background: '#09090b', foreground: '#e4e4e7' },
        scrollback: 5000,
      })
      const fit = new FitAddon.FitAddon()
      term.loadAddon(fit)
      el.innerHTML = ''
      term.open(el)
      session = { id, term, fit, ws: null, observer: null, detached: false }

      const safeFit = () => {
        try {
          fit.fit()
        } catch (e) {
          // container not visible yet
        }
      }
      safeFit()
      term.focus()

      session.observer = new ResizeObserver(() => safeFit())
      session.observer.observe(el)

      this.termStatus = 'connecting…'
      const ws = new WebSocket(wsURL(taskPath(this.taskName) + '/terminals/' + enc(id) + '/ws'))
      ws.binaryType = 'arraybuffer'
      session.ws = ws
      const encoder = new TextEncoder()
      const sendResize = () => {
        if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'resize', rows: term.rows, cols: term.cols }))
      }

      ws.onopen = () => {
        if (!session || session.ws !== ws) return
        this.termStatus = 'attached'
        safeFit()
        sendResize()
      }
      ws.onmessage = (ev) => {
        if (typeof ev.data === 'string') term.write(ev.data)
        else term.write(new Uint8Array(ev.data))
      }
      ws.onclose = (ev) => {
        if (!session || session.ws !== ws) return
        if (session.detached) return
        const reason = ev.reason || (ev.code === 1006 ? 'connection failed or dropped (code 1006)' : 'code ' + ev.code)
        this.termStatus = 'closed: ' + reason
        this.loadTerminals()
      }
      term.onData((d) => {
        if (ws.readyState === WebSocket.OPEN) ws.send(encoder.encode(d))
      })
      term.onBinary((d) => {
        if (ws.readyState === WebSocket.OPEN) ws.send(Uint8Array.from(d, (c) => c.charCodeAt(0) & 255))
      })
      term.onResize(() => sendResize())
    },

    // Detach keeps the job running server-side; only the socket closes.
    detachTerminal() {
      if (!session || !session.ws) return
      session.detached = true
      session.ws.close(1000, 'detached')
      this.termStatus = 'detached, still running'
    },

    reattachTerminal() {
      this.openTerminal()
    },

    closeTerminal() {
      if (!session) return
      session.detached = true
      if (session.observer) session.observer.disconnect()
      if (session.ws && session.ws.readyState <= WebSocket.OPEN) session.ws.close(1000, 'detached')
      session.term.dispose()
      session = null
      this.termStatus = ''
    },

    get termAttached() {
      return this.termStatus === 'attached'
    },

    // ---------- messages ----------

    async loadChannels() {
      try {
        const data = await api('GET', '/api/msg/channels')
        this.channels = ((data && data.channels) || []).map(normalizeChannel).filter((c) => c.channel)
        this.channelsError = ''
      } catch (e) {
        this.channelsError = e.message
      }
      this.channelsLoaded = true
    },

    goToChannel() {
      let c = this.gotoChannel.trim()
      if (!c) return
      if (!c.startsWith('#')) c = '#' + c
      this.gotoChannel = ''
      location.hash = channelHash(c)
    },

    stopChat() {
      chatGen++
      if (chatAbort) chatAbort.abort()
      chatAbort = null
      this.chat = { channel: '', messages: [], cursor: 0, loading: false, error: '', draft: '', replyTo: null, posting: false, postError: '' }
    },

    async openChat(channel) {
      this.stopChat()
      const gen = chatGen
      const ctrl = new AbortController()
      chatAbort = ctrl
      this.chat.channel = channel
      this.chat.loading = true

      const q = (after, extra) => '?channel=' + enc(channel) + '&after=' + after + extra
      const live = () => gen === chatGen

      // Catch up from the start, then long-poll for new messages.
      try {
        for (let page = 0; page < 50 && live(); page++) {
          const data = await api('GET', '/api/msg/read' + q(this.chat.cursor, '&limit=' + READ_LIMIT), undefined, ctrl.signal)
          if (!live()) return
          const got = this.appendMessages(data)
          if (got < READ_LIMIT) break
        }
        this.chat.error = ''
      } catch (e) {
        if (!live()) return
        this.chat.error = e.message
      }
      if (!live()) return
      this.chat.loading = false
      this.scrollChat(true)

      while (live()) {
        try {
          const data = await api('GET', '/api/msg/wait' + q(this.chat.cursor, '&timeout=' + WAIT_TIMEOUT), undefined, ctrl.signal)
          if (!live()) return
          this.chat.error = ''
          if (this.appendMessages(data)) this.scrollChat(false)
        } catch (e) {
          if (!live()) return
          this.chat.error = e.message + ' (retrying)'
          await new Promise((r) => setTimeout(r, 3000))
        }
      }
    },

    appendMessages(data) {
      const list = (data && data.messages) || []
      const seen = new Set(this.chat.messages.map((m) => m.id))
      const fresh = list.filter((m) => !seen.has(m.id))
      if (fresh.length) this.chat.messages = this.chat.messages.concat(fresh).slice(-KEEP_MESSAGES)
      let cursor = this.chat.cursor
      if (data && typeof data.cursor === 'number') cursor = Math.max(cursor, data.cursor)
      if (list.length) cursor = Math.max(cursor, list[list.length - 1].id || 0)
      this.chat.cursor = cursor
      return list.length
    },

    scrollChat(force) {
      this.$nextTick(() => {
        const el = this.$refs.stream
        if (!el) return
        const near = el.scrollHeight - el.scrollTop - el.clientHeight < 120
        if (force || near) el.scrollTop = el.scrollHeight
      })
    },

    messageById(id) {
      return this.chat.messages.find((m) => m.id === id)
    },

    setName(name) {
      const clean = name.trim().replace(/^@/, '')
      if (!clean || clean === this.me.name) return
      const old = this.me.id
      if (old) api('POST', '/api/msg/leave', { participant_id: old }).catch(() => {})
      this.me = { name: clean, id: '' }
      localStorage.setItem(NAME_KEY, clean)
      localStorage.removeItem(PARTICIPANT_KEY)
    },

    async join() {
      const p = await api('POST', '/api/msg/join', { name: this.me.name })
      const id = p && (p.id || p.participant_id)
      if (!id) throw new Error('join returned no participant id')
      this.me.id = id
      localStorage.setItem(PARTICIPANT_KEY, id)
    },

    async post() {
      const body = this.chat.draft
      if (!body.trim() || !this.chat.channel) return
      this.chat.posting = true
      this.chat.postError = ''
      const send = () => {
        const req = { participant_id: this.me.id, channel: this.chat.channel, body }
        if (this.chat.replyTo) req.reply_to = this.chat.replyTo
        return api('POST', '/api/msg/post', req)
      }
      try {
        if (!this.me.id) await this.join()
        try {
          await send()
        } catch (e) {
          const notJoined = (e.status === 404 || e.status === 400) && /participant|join/i.test(e.message)
          if (!notJoined) throw e
          await this.join()
          await send()
        }
        this.chat.draft = ''
        this.chat.replyTo = null
      } catch (e) {
        this.chat.postError = e.message
      }
      this.chat.posting = false
    },

    postKey(ev) {
      if (ev.key === 'Enter' && !ev.shiftKey && !ev.isComposing) {
        ev.preventDefault()
        this.post()
      }
    },

    // ---------- misc ----------

    toggleTheme() {
      document.documentElement.classList.toggle('dark')
    },

    ago(ts) {
      const t = Date.parse(ts)
      if (Number.isNaN(t)) return ts || ''
      const s = Math.floor((Date.now() - t) / 1000)
      if (s < 60) return 'just now'
      if (s < 3600) return Math.floor(s / 60) + 'm ago'
      if (s < 86400) return Math.floor(s / 3600) + 'h ago'
      return Math.floor(s / 86400) + 'd ago'
    },

    clock(ts) {
      const t = new Date(ts)
      if (Number.isNaN(t.getTime())) return ''
      return t.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    },

    fullTime(ts) {
      const t = new Date(ts)
      if (Number.isNaN(t.getTime())) return ts || ''
      return t.toLocaleString()
    },
  }
}
