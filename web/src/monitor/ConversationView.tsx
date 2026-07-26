import { useEffect, useState } from "react";
import { fetchConversations } from "../api";
import { CopyButton } from "../builder/Inspector";
import type {
  Conversation,
  ConversationMessage,
  TraceEvent,
} from "../types/api";

const POLL_MS = 5000;

function roleLabel(message: ConversationMessage) {
  if (message.Role === "tool") {
    return message.Name ? `tool · ${message.Name}` : "tool";
  }
  return message.Role;
}

function toolCallText(message: ConversationMessage) {
  const call = message.ToolCall;
  if (!call) {
    return "";
  }
  const args = call.Arguments?.trim() ?? "";
  return `${call.Name ?? "tool"}(${args})`;
}

function MessageRow({ message }: { message: ConversationMessage }) {
  const toolCall = toolCallText(message);
  return (
    <div
      className={`conversation-message conversation-message--${message.Role}`}
    >
      <span className="conversation-message__role">
        {roleLabel(message)}
      </span>
      <div className="conversation-message__body">
        {message.Content && <pre>{message.Content}</pre>}
        {toolCall && (
          <pre className="conversation-message__tool-call">
            → {toolCall}
          </pre>
        )}
        {!message.Content && !toolCall && <pre>—</pre>}
      </div>
    </div>
  );
}

function TraceRow({ event }: { event: TraceEvent }) {
  const label =
    event.kind === "reasoning"
      ? "reasoning"
      : event.kind === "command"
        ? "ran"
        : event.kind === "tool"
          ? "tool"
          : event.kind;
  return (
    <div
      className={`conversation-trace__event conversation-trace__event--${event.kind}`}
    >
      <span className="conversation-trace__kind">{label}</span>
      <div className="conversation-trace__body">
        <pre>{event.content}</pre>
        {event.detail && (
          <details>
            <summary>output</summary>
            <pre>{event.detail}</pre>
          </details>
        )}
      </div>
    </div>
  );
}

function ConversationBlock({
  conversation,
}: {
  conversation: Conversation;
}) {
  const messages = conversation.messages ?? [];
  const trace = conversation.trace ?? [];
  const tokens =
    conversation.input_tokens + conversation.output_tokens;
  return (
    <section className="conversation-block">
      <header className="conversation-block__head">
        <b>{conversation.agent_name || "agent"}</b>
        <span className="mono">
          {conversation.harness} · {conversation.model}
        </span>
        <span className="conversation-block__meta mono">
          {conversation.status}
          {tokens > 0 && ` · ${tokens.toLocaleString()} tok`}
        </span>
      </header>

      {conversation.harness_error &&
        conversation.status !== "succeeded" && (
          <div className="builder-inspector__error">
            <strong>Harness error</strong>
            {conversation.harness_error}
          </div>
        )}

      {conversation.instructions && (
        <div className="conversation-message conversation-message--system">
          <span className="conversation-message__role">
            instructions
          </span>
          <div className="conversation-message__body">
            <pre>{conversation.instructions}</pre>
          </div>
        </div>
      )}

      {messages.length === 0 && trace.length === 0 && (
        <div className="conversation-empty">
          No messages recorded for this conversation.
        </div>
      )}
      {messages.map((message, index) => (
        <MessageRow key={index} message={message} />
      ))}

      {trace.length > 0 && (
        <div className="conversation-trace">
          <div className="conversation-trace__head">
            Trace
            <span>{trace.length} steps</span>
          </div>
          {trace.map((event, index) => (
            <TraceRow key={index} event={event} />
          ))}
        </div>
      )}
    </section>
  );
}

/**
 * The LLM conversation(s) inside one node execution. Rendered in place
 * of the workflow canvas; polls while open so live runs stream in.
 */
export function ConversationView({
  nodeName,
  nodeExecutionId,
  onBack,
}: {
  nodeName: string;
  nodeExecutionId: string;
  onBack: () => void;
}) {
  const [conversations, setConversations] = useState<Conversation[]>(
    [],
  );
  const [ready, setReady] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    const controller = new AbortController();
    setReady(false);
    setError("");
    setConversations([]);

    async function load() {
      try {
        const next = await fetchConversations(
          nodeExecutionId,
          controller.signal,
        );
        if (active) {
          setConversations(next);
          setError("");
        }
      } catch (caught) {
        if (active) {
          setError(
            caught instanceof Error ? caught.message : String(caught),
          );
        }
      } finally {
        if (active) {
          setReady(true);
        }
      }
    }

    void load();
    const interval = window.setInterval(() => void load(), POLL_MS);
    return () => {
      active = false;
      controller.abort();
      window.clearInterval(interval);
    };
  }, [nodeExecutionId]);

  const copyValue = conversations
    .flatMap((conversation) => conversation.messages ?? [])
    .map(
      (message) =>
        `${message.Role}: ${message.Content ?? toolCallText(message)}`,
    )
    .join("\n\n");

  return (
    <main
      className="builder-canvas conversation-pane scrollnice"
      data-component="ConversationView"
    >
      <div className="conversation-pane__inner">
        <header className="conversation-pane__head">
          <button
            type="button"
            className="builder-ghost-button"
            onClick={onBack}
          >
            ← Canvas
          </button>
          <h2>{nodeName}</h2>
          <span className="conversation-pane__eyebrow">
            Conversation
          </span>
          {copyValue && <CopyButton value={copyValue} />}
        </header>

        {error && <div className="builder-error">{error}</div>}
        {!ready && (
          <div className="conversation-empty">Loading conversation…</div>
        )}
        {ready && !error && conversations.length === 0 && (
          <div className="conversation-empty">
            No conversation was recorded for this node execution.
          </div>
        )}
        {conversations.map((conversation) => (
          <ConversationBlock
            key={conversation.id}
            conversation={conversation}
          />
        ))}
      </div>
    </main>
  );
}
