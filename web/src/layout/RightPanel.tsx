import type { ReactNode } from "react";

/**
 * The inspector column down the right of a canvas page. Fixed width via
 * --right-panel; the content inside owns its own scrolling.
 */
export function RightPanel({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <aside
      className={`right-panel ${className}`.trim()}
      data-component="RightPanel"
    >
      {children}
    </aside>
  );
}
