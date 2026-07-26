import { useEffect, useState } from "react";
import { useLocation } from "react-router-dom";

/** Route → the page component that renders it. */
const SCREENS: Array<[RegExp, string]> = [
  [/^\/$/, "MainMonitorPage"],
  [/^\/workflows/, "MainMonitorPage"],
  [/^\/executions\//, "WorkflowsMonitor"],
  [/^\/build/, "BuilderPage"],
  [/^\/workflow\/[^/]+\/build/, "BuilderPage"],
  [/^\/workflow\/[^/]+$/, "WorkflowRedirect"],
  [/^\/tasks\//, "LegacyTaskRedirect"],
];

function screenName(pathname: string) {
  for (const [pattern, name] of SCREENS) {
    if (pattern.test(pathname)) {
      return name;
    }
  }
  return "Unknown";
}

/**
 * Ctrl+Shift+D outlines every component that declares a
 * `data-component` name and shows which screen you are on. Development
 * aid only — it renders nothing until switched on.
 */
export function DebugMode() {
  const [on, setOn] = useState(false);
  const { pathname, search } = useLocation();

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (
        event.shiftKey &&
        (event.ctrlKey || event.metaKey) &&
        event.key.toLowerCase() === "d"
      ) {
        event.preventDefault();
        setOn((value) => !value);
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  useEffect(() => {
    const root = document.documentElement;
    if (on) {
      root.setAttribute("data-debug", "");
    } else {
      root.removeAttribute("data-debug");
    }
    return () => root.removeAttribute("data-debug");
  }, [on]);

  if (!on) {
    return null;
  }

  return (
    <div className="debug-badge">
      <b>{screenName(pathname)}</b>
      <span className="mono">
        {pathname}
        {search}
      </span>
      <button
        type="button"
        onClick={() => setOn(false)}
        aria-label="Close debug mode"
      >
        ×
      </button>
    </div>
  );
}
