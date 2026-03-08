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
  const { accessToken, refreshAccessToken } = useAuth();

  const transport = useMemo(
    () =>
      createConnectTransport({
        baseUrl: process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090",
        interceptors: [
          (next) => async (req) => {
            if (accessToken) {
              req.header.set("Authorization", `Bearer ${accessToken}`);
            }
            try {
              return await next(req);
            } catch (err) {
              // On Unauthenticated, try refreshing the token and retry once
              if (err instanceof ConnectError && err.code === Code.Unauthenticated) {
                const newToken = await refreshAccessToken();
                if (newToken) {
                  req.header.set("Authorization", `Bearer ${newToken}`);
                  return await next(req);
                }
              }
              throw err;
            }
          },
        ],
      }),
    [accessToken, refreshAccessToken],
  );

  return <TransportContext.Provider value={transport}>{children}</TransportContext.Provider>;
}
