import Markdown from "react-markdown";
import type { ChatMessage } from "@/lib/hooks/useChat";

interface MessageBubbleProps {
  message: ChatMessage;
  isStreaming?: boolean;
  showPersonaBadge?: boolean;
}

export function MessageBubble({ message, isStreaming, showPersonaBadge }: MessageBubbleProps) {
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
            <span className="text-xs font-medium text-[var(--color-text-secondary)]">{message.personaName}</span>
          </div>
        )}
        <div
          className={`rounded-2xl px-4 py-3 ${
            isUser
              ? "bg-[var(--color-user-bubble)] text-[var(--color-user-bubble-text)] rounded-br-sm"
              : "bg-[var(--color-assistant-bubble)] border border-[var(--color-assistant-bubble-border)] text-[var(--color-assistant-bubble-text)] rounded-bl-sm"
          }`}
          style={!isUser && accentColor ? { borderLeftColor: accentColor, borderLeftWidth: 3 } : undefined}
        >
          {isUser ? (
            <p className="whitespace-pre-wrap text-sm leading-relaxed">{content}</p>
          ) : (
            <div className="text-sm leading-relaxed prose prose-sm max-w-none prose-p:my-1.5 prose-ul:my-1.5 prose-ol:my-1.5 prose-li:my-0.5 prose-headings:my-2 prose-pre:my-2 prose-a:text-[var(--color-accent)] dark:prose-invert">
              <Markdown>{content}</Markdown>
              {isStreaming && <span className="inline-block w-1.5 h-4 bg-[var(--color-text-tertiary)] ml-0.5 animate-pulse align-text-bottom" aria-hidden="true" />}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
