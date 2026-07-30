import { useEffect, useState } from "react";
import { fetchHarnesses } from "../api";
import type { HarnessInfo } from "../types/api";

/**
 * The harnesses installed on this machine and the models each can
 * run. pi reports its real catalog; codex/claude show curated lists.
 */
export function HarnessesPanel() {
  const [harnesses, setHarnesses] = useState<HarnessInfo[]>([]);
  const [expanded, setExpanded] = useState("");
  const [query, setQuery] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    let active = true;

    fetchHarnesses(controller.signal)
      .then((response) => {
        if (active) {
          setHarnesses(response.harnesses ?? []);
        }
      })
      .catch(() => undefined);

    return () => {
      active = false;
      controller.abort();
    };
  }, []);

  if (harnesses.length === 0) {
    return null;
  }

  return (
    <div className="harness-panel">
      <div className="harness-panel__head">Harnesses</div>
      {harnesses.map((harness) => {
        const models = harness.models ?? [];
        const isOpen = expanded === harness.id;
        const trimmed = query.trim().toLowerCase();
        const visibleModels = isOpen
          ? models.filter(
              (model) =>
                !trimmed || model.toLowerCase().includes(trimmed),
            )
          : [];
        return (
          <div key={harness.id} className="harness-panel__item">
            <button
              type="button"
              className="harness-panel__row"
              onClick={() => {
                setExpanded(isOpen ? "" : harness.id);
                setQuery("");
              }}
            >
              <span
                className={`harness-panel__dot ${
                  harness.available ? "available" : ""
                }`}
                title={
                  harness.available
                    ? `${harness.binary} is installed`
                    : `${harness.binary} was not found on PATH`
                }
              />
              <b className="mono">{harness.id}</b>
              <small>
                {harness.available
                  ? `${models.length} models`
                  : "not installed"}
              </small>
            </button>
            {isOpen && models.length > 0 && (
              <div className="harness-panel__models">
                <input
                  className="builder-input"
                  placeholder="Filter models…"
                  value={query}
                  autoFocus
                  onChange={(event) => setQuery(event.target.value)}
                />
                <div className="harness-panel__list scrollnice">
                  {visibleModels.map((model) => (
                    <div
                      key={model}
                      className="harness-panel__model mono"
                      title={model}
                    >
                      {model}
                    </div>
                  ))}
                  {visibleModels.length === 0 && (
                    <div className="harness-panel__empty">
                      No matching models
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
