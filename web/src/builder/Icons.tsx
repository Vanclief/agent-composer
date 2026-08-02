interface IconProps {
  size?: number;
}

export function LlmIcon({ size = 14 }: IconProps) {
  // A compute chip — the node that burns tokens.
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <rect x="4" y="4" width="8" height="8" rx="1.5" />
      <rect x="6.7" y="6.7" width="2.6" height="2.6" />
      <path d="M6 4 V1.8 M10 4 V1.8 M6 14.2 V12 M10 14.2 V12" />
      <path d="M4 6 H1.8 M4 10 H1.8 M14.2 6 H12 M14.2 10 H12" />
    </svg>
  );
}

export function TriggerIcon({ size = 14 }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M9 2 L4 9 H8 L7 14 L12 7 H8 L9 2 Z" />
    </svg>
  );
}

export function TransformIcon({ size = 14 }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M2.5 5 H10 M10 5 L7.5 2.5 M10 5 L7.5 7.5" />
      <path d="M13.5 11 H6 M6 11 L8.5 8.5 M6 11 L8.5 13.5" />
    </svg>
  );
}

export function PlayIcon({ size = 12 }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 12 12"
      fill="currentColor"
      aria-hidden="true"
    >
      <path d="M3 2 L10 6 L3 10 Z" />
    </svg>
  );
}

export function StopIcon({ size = 10 }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 10 10"
      fill="currentColor"
      aria-hidden="true"
    >
      <rect x="2" y="2" width="6" height="6" rx="1" />
    </svg>
  );
}

export function HistoryIcon({ size = 14 }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M3 8 A5 5 0 1 0 5 4" />
      <path d="M3 4 V7 H6" />
      <path d="M8 5 V8 L10 9.5" />
    </svg>
  );
}

export function StackIcon({ size = 16 }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M8 2 L14 5 L8 8 L2 5 Z" />
      <path d="M2 8 L8 11 L14 8" />
      <path d="M2 11 L8 14 L14 11" />
    </svg>
  );
}

export function BlocksIcon({ size = 16 }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <rect x="2.5" y="2.5" width="5" height="5" rx="1" />
      <rect x="8.5" y="2.5" width="5" height="5" rx="1" />
      <rect x="2.5" y="8.5" width="5" height="5" rx="1" />
      <rect x="8.5" y="8.5" width="5" height="5" rx="1" />
    </svg>
  );
}

export function BoltIcon({ size = 16 }: IconProps) {
  return <TriggerIcon size={size} />;
}

export function CogIcon({ size = 16 }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="8" cy="8" r="2.2" />
      <path d="M8 1.5 V3 M8 13 V14.5 M14.5 8 H13 M3 8 H1.5 M12.6 3.4 L11.5 4.5 M4.5 11.5 L3.4 12.6 M12.6 12.6 L11.5 11.5 M4.5 4.5 L3.4 3.4" />
    </svg>
  );
}

export function ChatIcon({ size = 13 }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M13.5 8a5.5 5.5 0 0 1-8.1 4.8L2.5 13.5l.8-2.7A5.5 5.5 0 1 1 13.5 8Z" />
      <path d="M5.6 7h4.8 M5.6 9.4h3" />
    </svg>
  );
}

export function FolderIcon({ size = 14 }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 14 14"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.4"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M1.5 3.5 A1 1 0 0 1 2.5 2.5 H5.5 L6.8 3.9 H11.5 A1 1 0 0 1 12.5 4.9 V10.5 A1 1 0 0 1 11.5 11.5 H2.5 A1 1 0 0 1 1.5 10.5 Z" />
    </svg>
  );
}

export function BranchIcon({ size = 14 }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 14 14"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.4"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="3.5" cy="3.5" r="1.6" />
      <circle cx="3.5" cy="10.5" r="1.6" />
      <circle cx="10.5" cy="3.5" r="1.6" />
      <path d="M3.5 5.1v3.8" />
      <path d="M10.5 5.1a5.4 5.4 0 0 1-5.4 5.4" />
    </svg>
  );
}

export function BoxArrowInIcon({ size = 14 }: IconProps) {
  // An arrow entering a box — the workflow's inputs.
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M8.5 3 H13 V13 H8.5" />
      <path d="M2 8 H9.5" />
      <path d="M7 5.5 L9.5 8 L7 10.5" />
    </svg>
  );
}

export function BoxArrowOutIcon({ size = 14 }: IconProps) {
  // An arrow leaving a box — the workflow's outputs.
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M7.5 3 H3 V13 H7.5" />
      <path d="M6.5 8 H14" />
      <path d="M11.5 5.5 L14 8 L11.5 10.5" />
    </svg>
  );
}

export function KindIcon({
  kind,
  size,
}: IconProps & { kind: string }) {
  if (kind === "llm") {
    return <LlmIcon size={size} />;
  }
  if (kind === "input") {
    return <BoxArrowInIcon size={size} />;
  }
  if (kind === "output") {
    return <BoxArrowOutIcon size={size} />;
  }
  if (kind === "trigger") {
    return <TriggerIcon size={size} />;
  }
  return <TransformIcon size={size} />;
}
