import type { ReactNode } from "react";
import { TopbarBrand } from "../nav/Brand";

/**
 * The bar across the top of every page.
 *
 * Brand and mode switch are fixed; pages supply what they are showing
 * (`title`) and what you can do about it (`actions`). The lead block is
 * the width of the LeftPanel, so the title always starts where the main
 * content does.
 */
export function TopBar({
  title,
  actions,
}: {
  title?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <header className="top-bar" data-component="TopBar">
      <div className="top-bar__lead">
        <TopbarBrand />
      </div>
      {title !== undefined && (
        <h1 className="top-bar__title">{title}</h1>
      )}
      <div className="top-bar__spacer" />
      {actions !== undefined && (
        <div className="top-bar__actions">{actions}</div>
      )}
    </header>
  );
}
