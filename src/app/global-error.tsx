"use client";

import { useEffect } from "react";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("Global error:", error);
  }, [error]);

  return (
    <html lang="en">
      <head>
        <style
          dangerouslySetInnerHTML={{
            __html: `
              @media (prefers-color-scheme: dark) {
                body { background: #1A1A1A !important; color: #F5F5F5 !important; }
                .ge-secondary { color: #A0A0A0 !important; }
              }
            `,
          }}
        />
      </head>
      <body
        style={{
          margin: 0,
          fontFamily: "system-ui, sans-serif",
          background: "#FAF8F5",
          color: "#2D2D2D",
        }}
      >
        <main
          style={{
            display: "flex",
            minHeight: "100vh",
            alignItems: "center",
            justifyContent: "center",
            padding: "2rem",
          }}
        >
          <div style={{ maxWidth: "28rem", textAlign: "center" }}>
            <h1 style={{ fontSize: "1.5rem", fontWeight: "bold", marginBottom: "0.5rem" }}>
              Something went wrong
            </h1>
            <p className="ge-secondary" style={{ color: "#6B7280", marginBottom: "2rem" }}>
              An unexpected error occurred. Please try again.
            </p>
            <button
              onClick={reset}
              style={{
                borderRadius: "9999px",
                background: "#E8654A",
                padding: "0.75rem 1.5rem",
                color: "white",
                fontWeight: 500,
                border: "none",
                cursor: "pointer",
                fontSize: "1rem",
              }}
            >
              Try again
            </button>
          </div>
        </main>
      </body>
    </html>
  );
}
