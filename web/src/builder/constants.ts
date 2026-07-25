import type { CanvasNodeKind, PortType } from "../types/workflow";

export const PORT_TYPES: Record<
  PortType,
  { color: string; soft: string; label: string }
> = {
  text: {
    color: "var(--t-text)",
    soft: "var(--t-text-soft)",
    label: "text",
  },
  json: {
    color: "var(--t-json)",
    soft: "var(--t-json-soft)",
    label: "json",
  },
  file: {
    color: "var(--t-file)",
    soft: "var(--t-file-soft)",
    label: "file",
  },
  any: {
    color: "var(--t-any)",
    soft: "var(--t-any-soft)",
    label: "any",
  },
};

export const KIND_VISUAL: Record<
  CanvasNodeKind,
  { background: string; foreground: string }
> = {
  llm: {
    background: "oklch(0.95 0.04 250)",
    foreground: "oklch(0.42 0.18 250)",
  },
  trigger: {
    background: "oklch(0.96 0.05 70)",
    foreground: "oklch(0.5 0.16 70)",
  },
  transform: {
    background: "oklch(0.95 0.04 145)",
    foreground: "oklch(0.42 0.16 145)",
  },
};

export interface LibraryItem {
  kind: CanvasNodeKind;
  name: string;
  sub: string;
}

export const NODE_LIBRARY: Array<{
  section: string;
  items: LibraryItem[];
}> = [
  {
    section: "LLM agents",
    items: [
      { kind: "llm", name: "LLM call", sub: "gpt / claude / etc" },
      { kind: "llm", name: "Tool-using agent", sub: "loop + tools" },
      { kind: "llm", name: "Classifier", sub: "route by label" },
    ],
  },
  {
    section: "Triggers",
    items: [
      { kind: "trigger", name: "Webhook", sub: "http endpoint" },
      { kind: "trigger", name: "Schedule", sub: "cron" },
      { kind: "trigger", name: "Event", sub: "pub/sub" },
    ],
  },
  {
    section: "Transforms",
    items: [
      { kind: "transform", name: "Map / fan-out", sub: "parallel" },
      { kind: "transform", name: "Filter", sub: "predicate" },
      { kind: "transform", name: "Merge", sub: "reduce" },
      { kind: "transform", name: "HTTP request", sub: "fetch" },
    ],
  },
];
