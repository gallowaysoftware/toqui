export function TypingIndicator() {
  return (
    <div className="flex justify-start">
      <div className="bg-[var(--color-assistant-bubble)] border border-[var(--color-assistant-bubble-border)] rounded-2xl rounded-bl-sm px-4 py-3 flex gap-1">
        <span className="w-2 h-2 bg-[var(--color-text-tertiary)] rounded-full animate-bounce" style={{ animationDelay: "0ms" }} />
        <span className="w-2 h-2 bg-[var(--color-text-tertiary)] rounded-full animate-bounce" style={{ animationDelay: "150ms" }} />
        <span className="w-2 h-2 bg-[var(--color-text-tertiary)] rounded-full animate-bounce" style={{ animationDelay: "300ms" }} />
      </div>
    </div>
  );
}
