import { memo } from "react";
import Markdown, { type Components } from "react-markdown";
import type { ChatMessage } from "@/lib/hooks/useChat";

const ALLOWED_ELEMENTS = [
  "p",
  "strong",
  "em",
  "ul",
  "ol",
  "li",
  "a",
  "code",
  "pre",
  "h1",
  "h2",
  "h3",
  "h4",
  "blockquote",
  "br",
  "hr",
  "table",
  "thead",
  "tbody",
  "tr",
  "th",
  "td",
];

const markdownComponents: Components = {
  a: ({ children, href, ...rest }) => (
    <a href={href} target="_blank" rel="noopener noreferrer" {...rest}>
      {children}
    </a>
  ),
};

interface MessageBubbleProps {
  message: ChatMessage;
  isStreaming?: boolean;
  showPersonaBadge?: boolean;
}

export const MessageBubble = memo(function MessageBubble({ message, isStreaming, showPersonaBadge }: MessageBubbleProps) {
  const { role, content } = message;

  if (role === "system") {
    return (
      <div className="flex justify-center" role="status">
        <p className="text-xs text-[var(--color-text-secondary)] bg-[var(--color-surface-tertiary)] rounded-full px-4 py-1.5 max-w-[80%] text-center">
          {content}
        </p>
      </div>
    );
  }

  const isUser = role === "user";
  const accentColor = message.personaAccentColor;

  return (
    <div className={`flex ${isUser ? "justify-end" : "justify-start"}`}>
      <div className="max-w-[80%]">
        {!isUser && showPersonaBadge && message.personaName && (
          <div className="flex items-center gap-1.5 mb-1 ml-1">
            {message.personaAvatar && (
              <div
                className="w-5 h-5 rounded-full bg-[var(--color-surface-tertiary)] flex items-center justify-center text-[10px] font-bold text-white"
                style={accentColor ? { backgroundColor: accentColor } : undefined}
                aria-hidden="true"
              >
                {message.personaName[0]}
              </div>
            )}
            <span className="text-xs font-medium text-[var(--color-text-secondary)]">
              {message.personaName}
            </span>
          </div>
        )}
        <div
          className={`rounded-2xl px-4 py-3 ${
            isUser
              ? "bg-[var(--color-user-bubble)] text-[var(--color-user-bubble-text)] rounded-br-sm"
              : "bg-[var(--color-assistant-bubble)] border border-[var(--color-assistant-bubble-border)] text-[var(--color-assistant-bubble-text)] rounded-bl-sm"
          }${message.isError ? " border-l-2 border-l-[var(--color-error)]" : ""}`}
          style={
            !isUser && accentColor && !message.isError
              ? { borderLeftColor: accentColor, borderLeftWidth: 3 }
              : undefined
          }
        >
          {isUser ? (
            <p className="whitespace-pre-wrap text-sm leading-relaxed">{content}</p>
          ) : (
            <div className="text-sm leading-relaxed prose prose-sm max-w-none prose-p:my-1.5 prose-ul:my-1.5 prose-ol:my-1.5 prose-li:my-0.5 prose-headings:my-2 prose-pre:my-2 prose-a:text-[var(--color-accent)] dark:prose-invert">
              <Markdown allowedElements={ALLOWED_ELEMENTS} unwrapDisallowed components={markdownComponents}>{content}</Markdown>
              {isStreaming && (
                <span
                  className="inline-block w-1.5 h-4 bg-[var(--color-text-tertiary)] ml-0.5 animate-pulse align-text-bottom"
                  aria-hidden="true"
                />
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
});
