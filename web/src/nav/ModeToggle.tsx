import { Link } from "react-router-dom";

/**
 * Monitor | Edit — the two modes of the Workflows section, docked
 * next to the brand. Monitor is the runs console; Edit is the
 * spec editor for the same workflow when one is in context.
 */
export function ModeToggle({
  mode,
  editTo,
}: {
  mode: "monitor" | "edit";
  /** Edit destination — the workflow in context, or the bare editor. */
  editTo?: string;
}) {
  return (
    <nav className="top-bar__mode" aria-label="Mode">
      <Link
        className={mode === "monitor" ? "active" : ""}
        to="/workflows"
      >
        Monitor
      </Link>
      <Link
        className={mode === "edit" ? "active" : ""}
        to={editTo || "/build"}
      >
        Edit
      </Link>
    </nav>
  );
}
