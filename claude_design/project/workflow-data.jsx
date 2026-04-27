// Workflow data — research agent pipeline
// Coords are in world-space (canvas pixels at 1x zoom).

const PORT_TYPES = {
  text: { color: 'var(--t-text)', soft: 'var(--t-text-soft)', label: 'text' },
  json: { color: 'var(--t-json)', soft: 'var(--t-json-soft)', label: 'json' },
  file: { color: 'var(--t-file)', soft: 'var(--t-file-soft)', label: 'file' },
  any:  { color: 'var(--t-any)',  soft: 'var(--t-any-soft)',  label: 'any'  },
};

// Initial workflow: research agent
const INITIAL_NODES = [
  {
    id: 'trigger',
    kind: 'trigger',
    name: 'On query received',
    sub: 'webhook · /research',
    x: 80, y: 220,
    config: {
      method: 'POST',
      path: '/research',
      auth: 'bearer',
    },
    inputs: [],
    outputs: [
      { id: 'q', label: 'query', type: 'text' },
    ],
    body: [
      { k: 'method', v: 'POST', mono: true },
      { k: 'path', v: '/research', mono: true },
    ],
    last: {
      output: { q: '"How does GLP-1 affect appetite long-term?"' },
      tokens: 0,
      ms: 12,
      status: 'ok',
    },
  },
  {
    id: 'plan',
    kind: 'llm',
    name: 'Plan research',
    sub: 'gpt-5 · plan_v3',
    x: 410, y: 100,
    config: {
      model: 'gpt-5',
      temperature: 0.2,
      max_tokens: 800,
      system: 'You break a research question into 3–5 search queries that cover distinct angles. Return JSON: { queries: string[] }.',
      tools: [],
    },
    inputs: [
      { id: 'q', label: 'question', type: 'text' },
    ],
    outputs: [
      { id: 'queries', label: 'queries', type: 'json' },
    ],
    body: [
      { k: 'model', v: 'gpt-5', mono: true },
      { k: 'temp', v: '0.2', mono: true },
    ],
    last: {
      input: { q: 'How does GLP-1 affect appetite long-term?' },
      output: {
        queries: [
          'GLP-1 receptor agonist long term appetite suppression mechanism',
          'semaglutide tirzepatide appetite rebound after discontinuation',
          'GLP-1 hypothalamus satiety signaling 12 month',
          'weight regain GLP-1 clinical trial follow up',
        ],
      },
      tokens: 612,
      ms: 1840,
      status: 'ok',
    },
  },
  {
    id: 'search',
    kind: 'transform',
    name: 'Web search × N',
    sub: 'fan-out · brave api',
    x: 410, y: 360,
    config: {
      operation: 'fan_out',
      provider: 'brave',
      per_query_results: 8,
      timeout_ms: 6000,
    },
    inputs: [
      { id: 'queries', label: 'queries', type: 'json' },
    ],
    outputs: [
      { id: 'results', label: 'results', type: 'json' },
    ],
    body: [
      { k: 'op', v: 'fan_out', mono: true },
      { k: 'top_k', v: '8', mono: true },
    ],
    last: {
      input: { queries: '4 items' },
      output: { results: '32 urls · dedup → 21' },
      tokens: 0,
      ms: 4210,
      status: 'ok',
    },
  },
  {
    id: 'scrape',
    kind: 'transform',
    name: 'Scrape & clean',
    sub: 'parallel · readability',
    x: 750, y: 360,
    config: {
      operation: 'map',
      concurrency: 6,
      strip: ['nav', 'footer', 'ads'],
      max_chars: 12000,
    },
    inputs: [
      { id: 'results', label: 'urls', type: 'json' },
    ],
    outputs: [
      { id: 'docs', label: 'docs', type: 'json' },
    ],
    body: [
      { k: 'op', v: 'map(scrape)', mono: true },
      { k: 'parallel', v: '6', mono: true },
    ],
    last: {
      input: { results: '21 urls' },
      output: { docs: '19 docs · 184k chars' },
      tokens: 0,
      ms: 9120,
      status: 'ok',
    },
  },
  {
    id: 'summarize',
    kind: 'llm',
    name: 'Summarize sources',
    sub: 'claude-sonnet-4.5',
    x: 1080, y: 220,
    config: {
      model: 'claude-sonnet-4.5',
      temperature: 0.1,
      max_tokens: 1200,
      system: 'For each source, extract: claim, evidence, study type, n, year. Cite with [n].',
    },
    inputs: [
      { id: 'docs', label: 'documents', type: 'json' },
      { id: 'q', label: 'context', type: 'text' },
    ],
    outputs: [
      { id: 'notes', label: 'notes', type: 'json' },
    ],
    body: [
      { k: 'model', v: 'sonnet-4.5', mono: true },
      { k: 'temp', v: '0.1', mono: true },
    ],
    last: {
      input: { docs: '19 docs', q: 'How does GLP-1...' },
      output: { notes: '19 structured · 47 claims' },
      tokens: 14820,
      ms: 11200,
      status: 'ok',
    },
  },
  {
    id: 'writer',
    kind: 'llm',
    name: 'Write report',
    sub: 'claude-opus-4.5',
    x: 1410, y: 220,
    config: {
      model: 'claude-opus-4.5',
      temperature: 0.4,
      max_tokens: 4000,
      system: 'Write a 600-word research brief with inline [n] citations and a sources list. Tone: neutral, scientific, accessible.',
    },
    inputs: [
      { id: 'notes', label: 'notes', type: 'json' },
      { id: 'q', label: 'question', type: 'text' },
    ],
    outputs: [
      { id: 'report', label: 'report', type: 'text' },
    ],
    body: [
      { k: 'model', v: 'opus-4.5', mono: true },
      { k: 'words', v: '~600', mono: true },
    ],
    last: {
      input: { notes: '47 claims', q: 'How does GLP-1...' },
      output: { report: 'Long-term, GLP-1 receptor agonists sustain appetite suppression primarily through...' },
      tokens: 6420,
      ms: 8400,
      status: 'ok',
    },
  },
];

const INITIAL_EDGES = [
  { id: 'e1', from: 'trigger', fromPort: 'q', to: 'plan', toPort: 'q' },
  { id: 'e2', from: 'plan', fromPort: 'queries', to: 'search', toPort: 'queries' },
  { id: 'e3', from: 'search', fromPort: 'results', to: 'scrape', toPort: 'results' },
  { id: 'e4', from: 'scrape', fromPort: 'docs', to: 'summarize', toPort: 'docs' },
  { id: 'e5', from: 'trigger', fromPort: 'q', to: 'summarize', toPort: 'q' },
  { id: 'e6', from: 'summarize', fromPort: 'notes', to: 'writer', toPort: 'notes' },
  { id: 'e7', from: 'trigger', fromPort: 'q', to: 'writer', toPort: 'q' },
];

const WORKFLOWS = [
  { id: 'research', name: 'Research agent', meta: 'v12', active: true },
  { id: 'support', name: 'Support triage', meta: 'v3', running: true },
  { id: 'leads', name: 'Lead enrichment', meta: 'v8' },
  { id: 'content', name: 'Content pipeline', meta: 'v2' },
  { id: 'classifier', name: 'Doc classifier', meta: 'v5' },
];

const NODE_LIBRARY = [
  { section: 'LLM agents', items: [
    { kind: 'llm', name: 'LLM call', sub: 'gpt / claude / etc', tone: 'accent' },
    { kind: 'llm', name: 'Tool-using agent', sub: 'loop + tools', tone: 'accent' },
    { kind: 'llm', name: 'Classifier', sub: 'route by label', tone: 'accent' },
  ]},
  { section: 'Triggers', items: [
    { kind: 'trigger', name: 'Webhook', sub: 'http endpoint', tone: 'amber' },
    { kind: 'trigger', name: 'Schedule', sub: 'cron', tone: 'amber' },
    { kind: 'trigger', name: 'Event', sub: 'pub/sub', tone: 'amber' },
  ]},
  { section: 'Transforms', items: [
    { kind: 'transform', name: 'Map / fan-out', sub: 'parallel', tone: 'green' },
    { kind: 'transform', name: 'Filter', sub: 'predicate', tone: 'green' },
    { kind: 'transform', name: 'Merge', sub: 'reduce', tone: 'green' },
    { kind: 'transform', name: 'HTTP request', sub: 'fetch', tone: 'green' },
  ]},
];

const KIND_VISUAL = {
  llm:       { bg: 'oklch(0.95 0.04 250)', fg: 'oklch(0.42 0.18 250)' },
  trigger:   { bg: 'oklch(0.96 0.05 70)',  fg: 'oklch(0.5 0.16 70)'  },
  transform: { bg: 'oklch(0.95 0.04 145)', fg: 'oklch(0.42 0.16 145)' },
};

// Workflow-level run history (most recent first). Each run carries a per-node snapshot.
// To keep data compact, snapshots tweak just the fields that matter (tokens/ms/status + a couple of fields).
const RUN_HISTORY = [
  {
    id: 'r-8a3f2c', when: '2 min ago', whenAbs: '14:32:08', status: 'ok',
    duration: 36800, tokens: 21854, cost: 0.218,
    query: '"How does GLP-1 affect appetite long-term?"',
    nodes: {
      trigger:   { tokens: 0, ms: 12, status: 'ok' },
      plan:      { tokens: 612, ms: 1840, status: 'ok' },
      search:    { tokens: 0, ms: 4210, status: 'ok' },
      scrape:    { tokens: 0, ms: 9120, status: 'ok' },
      summarize: { tokens: 14820, ms: 11200, status: 'ok' },
      writer:    { tokens: 6420, ms: 8400, status: 'ok' },
    },
  },
  {
    id: 'r-7c11d8', when: '8 min ago', whenAbs: '14:26:40', status: 'ok',
    duration: 34100, tokens: 20612, cost: 0.206,
    query: '"What is the optimal dose of metformin for PCOS?"',
    nodes: {
      trigger:   { tokens: 0, ms: 9, status: 'ok' },
      plan:      { tokens: 588, ms: 1720, status: 'ok' },
      search:    { tokens: 0, ms: 3980, status: 'ok' },
      scrape:    { tokens: 0, ms: 8400, status: 'ok' },
      summarize: { tokens: 13900, ms: 10800, status: 'ok' },
      writer:    { tokens: 6124, ms: 8200, status: 'ok' },
    },
  },
  {
    id: 'r-7a9241', when: '14 min ago', whenAbs: '14:20:11', status: 'ok',
    duration: 41200, tokens: 23104, cost: 0.231,
    query: '"Sleep deprivation effects on glucose tolerance"',
    nodes: {
      trigger:   { tokens: 0, ms: 14, status: 'ok' },
      plan:      { tokens: 645, ms: 2210, status: 'ok' },
      search:    { tokens: 0, ms: 4900, status: 'ok' },
      scrape:    { tokens: 0, ms: 10500, status: 'ok' },
      summarize: { tokens: 15200, ms: 12400, status: 'ok' },
      writer:    { tokens: 7259, ms: 11200, status: 'ok' },
    },
  },
  {
    id: 'r-6f01a4', when: '23 min ago', whenAbs: '14:11:02', status: 'err',
    duration: 9300, tokens: 612, cost: 0.006,
    query: '"Effects of intermittent fasting on autophagy"',
    nodes: {
      trigger:   { tokens: 0, ms: 11, status: 'ok' },
      plan:      { tokens: 612, ms: 1690, status: 'ok' },
      search:    { tokens: 0, ms: 6800, status: 'err' },
      scrape:    { tokens: 0, ms: 0, status: 'idle' },
      summarize: { tokens: 0, ms: 0, status: 'idle' },
      writer:    { tokens: 0, ms: 0, status: 'idle' },
    },
  },
  {
    id: 'r-6e8810', when: '1 hr ago', whenAbs: '13:32:55', status: 'ok',
    duration: 33400, tokens: 19880, cost: 0.198,
    query: '"Vitamin D supplementation in elderly: meta-analysis"',
    nodes: {
      trigger:   { tokens: 0, ms: 10, status: 'ok' },
      plan:      { tokens: 552, ms: 1690, status: 'ok' },
      search:    { tokens: 0, ms: 3820, status: 'ok' },
      scrape:    { tokens: 0, ms: 8200, status: 'ok' },
      summarize: { tokens: 13100, ms: 10400, status: 'ok' },
      writer:    { tokens: 6128, ms: 9300, status: 'ok' },
    },
  },
  {
    id: 'r-6d4099', when: '1 hr ago', whenAbs: '13:14:21', status: 'ok',
    duration: 35900, tokens: 21044, cost: 0.210,
    query: '"Microbiome diversity and depression: causal evidence"',
    nodes: {
      trigger:   { tokens: 0, ms: 13, status: 'ok' },
      plan:      { tokens: 601, ms: 1810, status: 'ok' },
      search:    { tokens: 0, ms: 4100, status: 'ok' },
      scrape:    { tokens: 0, ms: 8800, status: 'ok' },
      summarize: { tokens: 14000, ms: 10900, status: 'ok' },
      writer:    { tokens: 6330, ms: 10300, status: 'ok' },
    },
  },
];

Object.assign(window, { PORT_TYPES, INITIAL_NODES, INITIAL_EDGES, WORKFLOWS, NODE_LIBRARY, KIND_VISUAL, RUN_HISTORY });
