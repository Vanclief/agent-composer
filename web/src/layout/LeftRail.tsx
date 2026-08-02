import type { ReactNode } from "react";
import { Link } from "react-router-dom";

export interface RailItem {
  key: string;
  label: string;
  icon: ReactNode;
  /** Set when the item is a different page rather than a panel swap. */
  to?: string;
  /** Shown but inert — a surface that is temporarily switched off. */
  disabled?: boolean;
}

/**
 * The narrow icon rail on the far left, outside the LeftPanel. Switches
 * what the panel is showing — tasks vs workflows, workflows vs nodes.
 */
export function LeftRail({
  items,
  active,
  onSelect,
  footer,
}: {
  items: RailItem[];
  active: string;
  onSelect?: (key: string) => void;
  /** Pinned to the bottom of the rail (settings). */
  footer?: ReactNode;
}) {
  return (
    <nav className="left-rail" aria-label="Panel" data-component="LeftRail">
      {items.map((item) => {
        const className = item.key === active ? "active" : "";
        const current = item.key === active ? "page" : undefined;
        const body = (
          <>
            <span className="left-rail__icon">{item.icon}</span>
            <span className="left-rail__label">{item.label}</span>
          </>
        );
        if (item.disabled) {
          return (
            <button
              key={item.key}
              type="button"
              className="left-rail__disabled"
              title={`${item.label} — temporarily disabled`}
              disabled
            >
              {body}
            </button>
          );
        }
        return item.to ? (
          <Link
            key={item.key}
            to={item.to}
            className={className}
            title={item.label}
            aria-current={current}
          >
            {body}
          </Link>
        ) : (
          <button
            key={item.key}
            type="button"
            className={className}
            title={item.label}
            aria-current={current}
            onClick={() => onSelect?.(item.key)}
          >
            {body}
          </button>
        );
      })}
      {footer && <div className="left-rail__footer">{footer}</div>}
    </nav>
  );
}
