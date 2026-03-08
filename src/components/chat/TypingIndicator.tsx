export function TypingIndicator() {
  return (
    <div className="flex justify-start" role="status" aria-label="Assistant is typing">
      <div
        className="bg-[var(--color-assistant-bubble)] border border-[var(--color-assistant-bubble-border)] rounded-2xl rounded-bl-sm px-4 py-3 flex gap-1"
        aria-hidden="true"
      >
        <span
          className="w-2 h-2 bg-[var(--color-text-tertiary)] rounded-full animate-bounce"
          style={{ animationDelay: "0ms" }}
        />
        <span
          className="w-2 h-2 bg-[var(--color-text-tertiary)] rounded-full animate-bounce"
          style={{ animationDelay: "150ms" }}
        />
        <span
          className="w-2 h-2 bg-[var(--color-text-tertiary)] rounded-full animate-bounce"
          style={{ animationDelay: "300ms" }}
        />
      </div>
      <span className="sr-only">Assistant is typing</span>
    </div>
  );
}
