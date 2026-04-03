import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock @sentry/react-native before importing the module under test
const mockInit = vi.fn();
const mockCaptureException = vi.fn();
const mockWithScope = vi.fn((cb: (scope: unknown) => void) => {
  const scope = { setExtra: vi.fn() };
  cb(scope);
  return scope;
});

vi.mock("@sentry/react-native", () => ({
  init: (...args: unknown[]) => mockInit(...args),
  captureException: (...args: unknown[]) => mockCaptureException(...args),
  withScope: (cb: (scope: unknown) => void) => mockWithScope(cb),
}));

// Mock config — default to empty DSN (Sentry disabled)
let mockSentryDsn = "";
vi.mock("../config", () => ({
  getConfig: () => ({
    apiUrl: "http://localhost:8090",
    googleClientId: "",
    posthogKey: "",
    sentryDsn: mockSentryDsn,
  }),
}));

import { beforeSend, initSentry, captureException } from "../sentry";
import type { ErrorEvent } from "@sentry/react-native";

// Helper to create a minimal ErrorEvent (the Sentry type requires `type`)
function makeEvent(overrides: Omit<ErrorEvent, "type">): ErrorEvent {
  return { type: undefined, ...overrides } as ErrorEvent;
}

describe("beforeSend (PII stripping)", () => {
  it("removes user email, username, and ip_address", () => {
    const event = makeEvent({
      user: {
        id: "user-123",
        email: "alice@example.com",
        username: "alice",
        ip_address: "1.2.3.4",
      },
    });

    const result = beforeSend(event);

    expect(result).not.toBeNull();
    expect(result!.user).toBeDefined();
    expect(result!.user!.id).toBe("user-123");
    expect(result!.user!.email).toBeUndefined();
    expect(result!.user!.username).toBeUndefined();
    expect(result!.user!.ip_address).toBeUndefined();
  });

  it("strips breadcrumb data to prevent travel info leaks", () => {
    const event = makeEvent({
      breadcrumbs: [
        {
          category: "navigation",
          message: "trip screen",
          data: { destination: "Paris", dates: "Jan 1-5" },
        },
        {
          category: "ui.click",
          message: "button",
          data: { element: "send" },
        },
      ],
    });

    const result = beforeSend(event);

    expect(result).not.toBeNull();
    expect(result!.breadcrumbs).toHaveLength(2);
    for (const breadcrumb of result!.breadcrumbs!) {
      expect(breadcrumb.data).toBeUndefined();
      // Category and message are preserved
      expect(breadcrumb.category).toBeDefined();
    }
  });

  it("passes through events without user or breadcrumbs unchanged", () => {
    const event = makeEvent({
      exception: {
        values: [{ type: "Error", value: "something broke" }],
      },
    });

    const result = beforeSend(event);

    expect(result).toBe(event);
  });

  it("handles event with empty user object", () => {
    const event = makeEvent({ user: {} });
    const result = beforeSend(event);
    expect(result).not.toBeNull();
    expect(result!.user).toEqual({});
  });

  it("handles event with empty breadcrumbs array", () => {
    const event = makeEvent({ breadcrumbs: [] });
    const result = beforeSend(event);
    expect(result!.breadcrumbs).toEqual([]);
  });
});

describe("initSentry", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("does not initialise Sentry when DSN is empty", () => {
    mockSentryDsn = "";
    initSentry();
    expect(mockInit).not.toHaveBeenCalled();
  });

  it("initialises Sentry when DSN is provided", () => {
    mockSentryDsn = "https://examplePublicKey@o0.ingest.sentry.io/0";
    initSentry();
    expect(mockInit).toHaveBeenCalledWith(
      expect.objectContaining({
        dsn: "https://examplePublicKey@o0.ingest.sentry.io/0",
        sendDefaultPii: false,
        tracesSampleRate: 0.1,
      }),
    );
  });

  it("sets beforeSend hook for PII stripping", () => {
    mockSentryDsn = "https://examplePublicKey@o0.ingest.sentry.io/0";
    initSentry();
    const initCall = mockInit.mock.calls[0][0];
    expect(initCall.beforeSend).toBe(beforeSend);
  });
});

describe("captureException", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls Sentry.captureException for plain errors", () => {
    const error = new Error("test error");
    captureException(error);
    expect(mockCaptureException).toHaveBeenCalledWith(error);
  });

  it("adds context via withScope when context is provided", () => {
    const error = new Error("auth failed");
    captureException(error, { source: "token_refresh" });
    expect(mockWithScope).toHaveBeenCalled();
    expect(mockCaptureException).toHaveBeenCalledWith(error);
  });
});
