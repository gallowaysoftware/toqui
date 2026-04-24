import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
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

// ---------------------------------------------------------------------------
// Consent-required signal
//
// Backend PR #374 (merged b94db9d) adds a ConsentInterceptor that refuses
// non-exempt RPCs with `FailedPrecondition("consent_required")` when the
// authenticated user hasn't recorded BOTH `terms` and `privacy_policy`
// consents. The transport interceptor recognises that specific sentinel
// and flips a context flag so the `ConsentGate` component (wired into
// `app/_layout.tsx`) can pop the consent modal.
//
// Deliberate design choices:
//   - Match on BOTH `CodeFailedPrecondition` AND the literal substring
//     `consent_required`. Relying on the message alone would collide with
//     any future FailedPrecondition error that happens to contain that
//     phrase; relying on the code alone would over-trigger on unrelated
//     preconditions (age-gate errors, for example).
//   - The flag is a one-way latch: once set, it stays true until the user
//     accepts consent via `acknowledgeConsent()`. This prevents race
//     conditions where a second in-flight RPC re-flips the flag after the
//     user has already clicked "I agree".
//   - We do NOT swallow the error. The RPC still rejects — the consuming
//     hook/query handles the error normally. The gate just gets drawn on
//     top. After the user accepts and the consent is recorded server-side,
//     subsequent RPCs succeed and the UI recovers on its own.
// ---------------------------------------------------------------------------

export const CONSENT_REQUIRED_SENTINEL = "consent_required";

/** Exported for tests. Matches the sentinel used by `internal/auth/consent_interceptor.go`. */
export function isConsentRequiredError(err: unknown): boolean {
  if (!(err instanceof ConnectError)) return false;
  if (err.code !== Code.FailedPrecondition) return false;
  // ConnectError.message prefixes "[failed_precondition] " — check both
  // the raw message and `rawMessage` (the server's unprefixed string).
  const raw = err.rawMessage ?? "";
  return (
    raw.includes(CONSENT_REQUIRED_SENTINEL) ||
    err.message.includes(CONSENT_REQUIRED_SENTINEL)
  );
}

interface ConsentSignalState {
  /** True when a recent RPC failed with `consent_required`. */
  consentRequired: boolean;
  /** Clear the flag after the user has recorded consent. */
  acknowledgeConsent: () => void;
}

const ConsentSignalContext = createContext<ConsentSignalState | null>(null);

export function useConsentSignal(): ConsentSignalState {
  const ctx = useContext(ConsentSignalContext);
  if (!ctx) {
    throw new Error(
      "useConsentSignal must be used within a TransportProvider",
    );
  }
  return ctx;
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

  // Consent-required one-way latch. We keep a ref mirror of the state so
  // the interceptor (which closes over stable refs, not state) can read
  // the current value without recreating the transport.
  const [consentRequired, setConsentRequired] = useState(false);
  const consentRequiredRef = useRef(false);
  useEffect(() => {
    consentRequiredRef.current = consentRequired;
  }, [consentRequired]);

  const signalConsentRequired = useCallback(() => {
    if (!consentRequiredRef.current) {
      consentRequiredRef.current = true;
      setConsentRequired(true);
    }
  }, []);

  const acknowledgeConsent = useCallback(() => {
    consentRequiredRef.current = false;
    setConsentRequired(false);
  }, []);

  const signalConsentRef = useRef(signalConsentRequired);
  useEffect(() => {
    signalConsentRef.current = signalConsentRequired;
  }, [signalConsentRequired]);

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
              // Consent gate: flip the flag and re-throw. The React tree
              // watches `consentRequired` via context and pops the modal;
              // the calling hook still sees the error so its normal loading/
              // error states behave correctly.
              if (isConsentRequiredError(err)) {
                signalConsentRef.current();
                throw err;
              }

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

  const consentSignal = useMemo<ConsentSignalState>(
    () => ({ consentRequired, acknowledgeConsent }),
    [consentRequired, acknowledgeConsent],
  );

  return (
    <TransportContext.Provider value={transport}>
      <ConsentSignalContext.Provider value={consentSignal}>
        {children}
      </ConsentSignalContext.Provider>
    </TransportContext.Provider>
  );
}

