"use client";

import { useEffect } from "react";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("Unhandled error:", error);
  }, [error]);

  return (
    <main id="main-content" className="flex min-h-screen flex-col items-center justify-center p-8">
      <div className="max-w-md text-center">
        <h1 className="text-2xl font-bold text-[var(--color-text-primary)] mb-2">
          Something went wrong
        </h1>
        <p className="text-[var(--color-text-secondary)] mb-8">
          An unexpected error occurred. Please try again.
        </p>
        <button
          onClick={reset}
          className="rounded-full bg-[var(--color-accent)] px-6 py-3 text-white font-medium hover:bg-[var(--color-accent-hover)] transition-colors"
        >
          Try again
        </button>
      </div>
    </main>
  );
}
