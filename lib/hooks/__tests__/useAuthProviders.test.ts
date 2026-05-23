import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";

// ---------- mocks ----------

const mockGetAuthProviders = vi.fn();

vi.mock("@connectrpc/connect-web", () => ({
  createConnectTransport: vi.fn(() => "mock-transport"),
}));

vi.mock("@connectrpc/connect", () => ({
  createClient: vi.fn(() => ({
    getAuthProviders: mockGetAuthProviders,
  })),
}));

vi.mock("@gen/toqui/v1/auth_pb", () => ({
  AuthService: "MockAuthService",
}));

vi.mock("@/lib/config", () => ({
  getConfig: () => ({ apiUrl: "http://localhost:8090", googleClientId: "" }),
}));

import { useAuthProviders } from "../useAuthProviders";

// ---------- helpers ----------

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { wrapper, queryClient };
}

// ---------- tests ----------

describe("useAuthProviders", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns googleOauth: true when the server reports Google as configured", async () => {
    mockGetAuthProviders.mockResolvedValue({ emailPassword: true, googleOauth: true });
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useAuthProviders(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.data).toEqual({ emailPassword: true, googleOauth: true });
    expect(mockGetAuthProviders).toHaveBeenCalledWith({});
  });

  it("returns googleOauth: false when the server reports Google as disabled", async () => {
    mockGetAuthProviders.mockResolvedValue({ emailPassword: true, googleOauth: false });
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useAuthProviders(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.data).toEqual({ emailPassword: true, googleOauth: false });
  });

  it("starts in loading state before the RPC resolves", () => {
    let resolveFn: (v: { emailPassword: boolean; googleOauth: boolean }) => void = () => {};
    mockGetAuthProviders.mockReturnValue(
      new Promise((r) => {
        resolveFn = r;
      }),
    );
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useAuthProviders(), { wrapper });

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
    // Resolve to clean up the pending promise so vitest doesn't warn
    resolveFn({ emailPassword: true, googleOauth: false });
  });
});
