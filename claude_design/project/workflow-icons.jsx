// Icons — small inline SVGs (16/14px). No emoji.

const Ico = {
  Llm: (p) => (
    <svg width={p.s||14} height={p.s||14} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <path d="M8 2.5 L13 5 L13 11 L8 13.5 L3 11 L3 5 Z"/>
      <path d="M8 2.5 V8 M8 8 L3 5 M8 8 L13 5 M8 8 V13.5"/>
    </svg>
  ),
  Trigger: (p) => (
    <svg width={p.s||14} height={p.s||14} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <path d="M9 2 L4 9 H8 L7 14 L12 7 H8 L9 2 Z"/>
    </svg>
  ),
  Transform: (p) => (
    <svg width={p.s||14} height={p.s||14} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2.5 5 H10 M10 5 L7.5 2.5 M10 5 L7.5 7.5"/>
      <path d="M13.5 11 H6 M6 11 L8.5 8.5 M6 11 L8.5 13.5"/>
    </svg>
  ),
  Play: (p) => (
    <svg width={p.s||12} height={p.s||12} viewBox="0 0 12 12" fill="currentColor"><path d="M3 2 L10 6 L3 10 Z"/></svg>
  ),
  Stop: (p) => (
    <svg width={p.s||10} height={p.s||10} viewBox="0 0 10 10" fill="currentColor"><rect x="2" y="2" width="6" height="6" rx="1"/></svg>
  ),
  Plus: (p) => (
    <svg width={p.s||14} height={p.s||14} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round"><path d="M8 3 V13 M3 8 H13"/></svg>
  ),
  Search: (p) => (
    <svg width={p.s||14} height={p.s||14} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round"><circle cx="7" cy="7" r="4.5"/><path d="M10.5 10.5 L13.5 13.5"/></svg>
  ),
  ZoomIn: (p) => (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round"><circle cx="7" cy="7" r="4.5"/><path d="M10.5 10.5 L13.5 13.5 M5 7 H9 M7 5 V9"/></svg>
  ),
  ZoomOut: (p) => (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round"><circle cx="7" cy="7" r="4.5"/><path d="M10.5 10.5 L13.5 13.5 M5 7 H9"/></svg>
  ),
  Fit: (p) => (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><path d="M3 6 V3 H6 M13 6 V3 H10 M3 10 V13 H6 M13 10 V13 H10"/></svg>
  ),
  More: (p) => (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><circle cx="3.5" cy="8" r="1.2"/><circle cx="8" cy="8" r="1.2"/><circle cx="12.5" cy="8" r="1.2"/></svg>
  ),
  Share: (p) => (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><circle cx="4" cy="8" r="2"/><circle cx="12" cy="4" r="2"/><circle cx="12" cy="12" r="2"/><path d="M5.7 7 L10.3 5 M5.7 9 L10.3 11"/></svg>
  ),
  History: (p) => (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><path d="M3 8 A5 5 0 1 0 5 4"/><path d="M3 4 V7 H6"/><path d="M8 5 V8 L10 9.5"/></svg>
  ),
  Sparkle: (p) => (
    <svg width="12" height="12" viewBox="0 0 12 12" fill="currentColor"><path d="M6 1 L7 4.5 L10.5 5.5 L7 6.5 L6 10 L5 6.5 L1.5 5.5 L5 4.5 Z"/></svg>
  ),
  Stack: (p) => (
    <svg width={p.s||16} height={p.s||16} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <path d="M8 2 L14 5 L8 8 L2 5 Z"/>
      <path d="M2 8 L8 11 L14 8"/>
      <path d="M2 11 L8 14 L14 11"/>
    </svg>
  ),
  Blocks: (p) => (
    <svg width={p.s||16} height={p.s||16} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2.5" y="2.5" width="5" height="5" rx="1"/>
      <rect x="8.5" y="2.5" width="5" height="5" rx="1"/>
      <rect x="2.5" y="8.5" width="5" height="5" rx="1"/>
      <rect x="8.5" y="8.5" width="5" height="5" rx="1"/>
    </svg>
  ),
  Bolt: (p) => (
    <svg width={p.s||16} height={p.s||16} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <path d="M9 2 L4 9 H8 L7 14 L12 7 H8 L9 2 Z"/>
    </svg>
  ),
  Cog: (p) => (
    <svg width={p.s||16} height={p.s||16} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="8" cy="8" r="2.2"/>
      <path d="M8 1.5 V3 M8 13 V14.5 M14.5 8 H13 M3 8 H1.5 M12.6 3.4 L11.5 4.5 M4.5 11.5 L3.4 12.6 M12.6 12.6 L11.5 11.5 M4.5 4.5 L3.4 3.4"/>
    </svg>
  ),
};

function KindIcon({ kind, s }) {
  if (kind === 'llm') return <Ico.Llm s={s}/>;
  if (kind === 'trigger') return <Ico.Trigger s={s}/>;
  if (kind === 'transform') return <Ico.Transform s={s}/>;
  return <Ico.Sparkle s={s}/>;
}

Object.assign(window, { Ico, KindIcon });
