"use client";

import { createContext, useContext, useMemo } from "react";
import type { Transport } from "@connectrpc/connect";
import { Code, ConnectError } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { useAuth } from "./AuthProvider";

const TransportContext = createContext<Transport | null>(null);

export function useTransport(): Transport {
  const transport = useContext(TransportContext);
  if (!transport) {
    throw new Error("useTransport must be used within a GrpcProvider");
  }
  return transport;
}

export function GrpcProvider({ children }: { children: React.ReactNode }) {
  const { refreshAccessToken } = useAuth();

  const transport = useMemo(
    () =>
      createConnectTransport({
        baseUrl: process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090",
        // Send HttpOnly auth cookies with every request.
        // The backend's cookie-to-header middleware translates these into
        // Authorization: Bearer headers for the auth interceptor.
        fetch: (input, init) =>
          globalThis.fetch(input, { ...init, credentials: "include" }),
        interceptors: [
          (next) => async (req) => {
            try {
              return await next(req);
            } catch (err) {
              // On Unauthenticated, try refreshing the token cookie and retry once.
              if (
                err instanceof ConnectError &&
                err.code === Code.Unauthenticated
              ) {
                const refreshed = await refreshAccessToken();
                if (refreshed) {
                  // Retry — browser will send updated cookie automatically.
                  return await next(req);
                }
              }
              throw err;
            }
          },
        ],
      }),
    [refreshAccessToken],
  );

  return (
    <TransportContext.Provider value={transport}>
      {children}
    </TransportContext.Provider>
  );
}
