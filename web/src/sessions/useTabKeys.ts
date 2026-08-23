import { useEffect, useRef, useState } from "react";
import { moveSession } from "./mock";
import type { Session } from "./types";

/**
 * Written once and shown verbatim in the tab strip, so the hint can
 * never drift from what the listener below actually binds.
 */
export const KEY_HINT = "⌃1–9 switch · ⌃B c new · ⌃B n/p next/prev";

/** tmux gives its prefix a beat before handing the keys back. */
const PREFIX_MS = 1500;

interface TabKeys {
  sessions: Session[];
  activeId: string;
  onSelect: (id: string) => void;
  onNew: () => void;
}

/** Typing surfaces own their keys — except the tmux prefix. */
function isTyping(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  const tag = target.tagName;
  return (
    tag === "INPUT" ||
    tag === "TEXTAREA" ||
    tag === "SELECT" ||
    target.isContentEditable
  );
}

/** 1..9 → 0-based tab index; null for anything else. */
function digitIndex(key: string) {
  return /^[1-9]$/.test(key) ? Number(key) - 1 : null;
}

/** Held modifiers must not spend an armed prefix. */
function isModifier(key: string) {
  return (
    key === "Shift" || key === "Control" || key === "Alt" || key === "Meta"
  );
}

/**
 * tmux muscle memory for the tab strip. The listener is registered
 * once and reads the latest props through a ref, so re-rendering
 * mid-prefix never drops the armed state or its timer.
 */
export function useTabKeys({
  sessions,
  activeId,
  onSelect,
  onNew,
}: TabKeys) {
  const [armed, setArmed] = useState(false);
  const latest = useRef<TabKeys>({ sessions, activeId, onSelect, onNew });
  const armedRef = useRef(false);
  const timer = useRef(0);

  useEffect(() => {
    latest.current = { sessions, activeId, onSelect, onNew };
  });

  useEffect(() => {
    function disarm() {
      window.clearTimeout(timer.current);
      armedRef.current = false;
      setArmed(false);
    }

    function arm() {
      window.clearTimeout(timer.current);
      armedRef.current = true;
      setArmed(true);
      timer.current = window.setTimeout(disarm, PREFIX_MS);
    }

    function select(index: number) {
      const session = latest.current.sessions[index];
      if (session) {
        latest.current.onSelect(session.id);
      }
    }

    function step(delta: 1 | -1) {
      const { sessions: list, activeId: current } = latest.current;
      if (list.length === 0) {
        return;
      }
      const index = list.findIndex((session) => session.id === current);
      const next = list[(index + delta + list.length) % list.length];
      if (next) {
        latest.current.onSelect(next.id);
      }
    }

    function onKeyDown(event: KeyboardEvent) {
      // While the prefix is armed every key is ours, including keys
      // typed into the chat box — tabbing out mid-thought is the point.
      if (armedRef.current) {
        if (isModifier(event.key)) {
          return;
        }
        event.preventDefault();
        disarm();
        const index = digitIndex(event.key);
        if (index !== null) {
          select(index);
        } else if (event.key === "n") {
          step(1);
        } else if (event.key === "p") {
          step(-1);
        } else if (event.key === "c") {
          latest.current.onNew();
        }
        return;
      }

      // Cmd+key belongs to the browser; only plain Ctrl is bindable.
      const ctrl = event.ctrlKey && !event.metaKey && !event.altKey;

      if (ctrl && !event.shiftKey && event.key.toLowerCase() === "b") {
        event.preventDefault();
        arm();
        return;
      }

      const index =
        ctrl && !event.shiftKey ? digitIndex(event.key) : null;
      if (index !== null) {
        event.preventDefault();
        select(index);
        return;
      }

      if (isTyping(event.target)) {
        return;
      }

      if (
        ctrl &&
        event.shiftKey &&
        (event.key === "ArrowLeft" || event.key === "ArrowRight")
      ) {
        event.preventDefault();
        moveSession(
          latest.current.activeId,
          event.key === "ArrowLeft" ? -1 : 1,
        );
      }
    }

    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
      window.clearTimeout(timer.current);
    };
  }, []);

  return { armed };
}
