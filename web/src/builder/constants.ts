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
    background: "var(--t-text-soft)",
    foreground: "var(--t-text)",
  },
  trigger: {
    background: "var(--st-warn-soft)",
    foreground: "var(--st-warn-ink)",
  },
  transform: {
    background: "var(--t-json-soft)",
    foreground: "var(--t-json)",
  },
};
