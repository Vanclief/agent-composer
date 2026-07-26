import type { ReactNode } from "react";

/**
 * The column down the left of every page.
 *
 * `header` is pinned and never scrolls — filters, identity, section
 * heads. `children` is the list underneath it, and is the only part
 * that scrolls. Width comes from the page grid via --left-panel so it
 * is identical on every page.
 */
export function LeftPanel({
  header,
  children,
  className = "",
}: {
  header?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <aside
      className={`left-panel ${className}`.trim()}
      data-component="LeftPanel"
    >
      {header !== undefined && (
        <div className="left-panel__header">{header}</div>
      )}
      <div className="left-panel__body scrollnice">{children}</div>
    </aside>
  );
}
