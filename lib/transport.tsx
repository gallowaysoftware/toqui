import { createContext, useContext, useEffect, useMemo, useRef } from "react";
import type { Transport } from "@connectrpc/connect";
import { Code, ConnectError } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { useAuth } from "./auth";
import { getConfig } from "./config";

const TransportContext = createContext<Transport | null>(null);

export function useTransport(): Transport {
  const transport = useContext(TransportContext);
  if (!transport) {
    throw new Error("useTransport must be used within a TransportProvider");
  }
  return transport;
}


export function TransportProvider({ children }: { children: React.ReactNode }) {
  const { accessToken, refreshTokens } = useAuth();

  // Use refs so the interceptor always reads the latest token
  // without recreating the transport (which would tear active streams).
  const tokenRef = useRef(accessToken);
  useEffect(() => {
    tokenRef.current = accessToken;
  }, [accessToken]);

  const refreshRef = useRef(refreshTokens);
  useEffect(() => {
    refreshRef.current = refreshTokens;
  }, [refreshTokens]);

  // Deduplication mutex: if a refresh is already in flight, all concurrent
  // 401 interceptors await the same promise instead of each firing a new
  // refresh request.
  const refreshInFlightRef = useRef<Promise<string | null> | null>(null);

  const transport = useMemo(
    () =>
      createConnectTransport({
        baseUrl: getConfig().apiUrl,
        interceptors: [
          (next) => async (req) => {
            if (tokenRef.current) {
              req.header.set("Authorization", `Bearer ${tokenRef.current}`);
            }
            try {
              return await next(req);
            } catch (err) {
              if (
                err instanceof ConnectError &&
                err.code === Code.Unauthenticated
              ) {
                // Ensure only one refresh RPC is in flight at a time.
                if (!refreshInFlightRef.current) {
                  refreshInFlightRef.current = refreshRef.current().finally(() => {
                    refreshInFlightRef.current = null;
                  });
                }
                const newToken = await refreshInFlightRef.current;
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
    [], // stable — never recreated
  );

  return (
    <TransportContext.Provider value={transport}>
      {children}
    </TransportContext.Provider>
  );
}
