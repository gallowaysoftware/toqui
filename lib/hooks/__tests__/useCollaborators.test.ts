import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const mockAuthFetch = vi.fn();

vi.mock("@/lib/authFetch", () => ({
  authFetch: (...args: unknown[]) => mockAuthFetch(...args),
}));

vi.mock("@/lib/config", () => ({
  getConfig: () => ({ apiUrl: "http://localhost:8090" }),
}));

const mockAuth = { accessToken: "test-token" as string | null };
vi.mock("@/lib/auth", () => ({
  useAuth: () => mockAuth,
}));

import { useCollaborators, useInviteCollaborator, useRemoveCollaborator, useAcceptInvite } from "../useCollaborators";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function mockJsonResponse(data: unknown, ok = true, status?: number) {
  return {
    ok,
    status: status ?? (ok ? 200 : 500),
    json: () => Promise.resolve(data),
    text: () => Promise.resolve(JSON.stringify(data)),
  };
}

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { wrapper, queryClient };
}

// ---------------------------------------------------------------------------
// useCollaborators
// ---------------------------------------------------------------------------

describe("useCollaborators", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.accessToken = "test-token";
  });

  it("fetches collaborators when authenticated", async () => {
    const collaborators = [
      { id: "c1", email: "alice@example.com", role: "owner", invitedAt: "2025-01-01", acceptedAt: "2025-01-01", userId: "u1" },
      { id: "c2", email: "bob@example.com", role: "editor", invitedAt: "2025-01-02", acceptedAt: null, userId: null },
    ];
    mockAuthFetch.mockResolvedValue(mockJsonResponse({ collaborators }));
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useCollaborators("trip-1"), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.collaborators).toEqual(collaborators);
    expect(result.current.isError).toBe(false);
  });

  it("calls the correct API endpoint", async () => {
    mockAuthFetch.mockResolvedValue(mockJsonResponse({ collaborators: [] }));
    const { wrapper } = createWrapper();

    renderHook(() => useCollaborators("trip-42"), { wrapper });

    await waitFor(() => {
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "http://localhost:8090/api/trips/trip-42/collaborators",
        "test-token",
      );
    });
  });

  it("does NOT fetch when accessToken is null", async () => {
    mockAuth.accessToken = null;
    const { wrapper } = createWrapper();

    renderHook(() => useCollaborators("trip-1"), { wrapper });

    await new Promise((r) => setTimeout(r, 50));
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });

  it("does NOT fetch when tripId is empty", async () => {
    const { wrapper } = createWrapper();

    renderHook(() => useCollaborators(""), { wrapper });

    await new Promise((r) => setTimeout(r, 50));
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });

  it("returns empty array as default", () => {
    mockAuth.accessToken = null;
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useCollaborators("trip-1"), { wrapper });
    expect(result.current.collaborators).toEqual([]);
  });

  it("sets isError when API returns non-ok response", async () => {
    mockAuthFetch.mockResolvedValue(mockJsonResponse(null, false));
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useCollaborators("trip-1"), { wrapper });

    await waitFor(() => expect(result.current.isError).toBe(true));
  });
});

// ---------------------------------------------------------------------------
// useInviteCollaborator
// ---------------------------------------------------------------------------

describe("useInviteCollaborator", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.accessToken = "test-token";
  });

  it("calls the correct API endpoint and parses email_sent=true response", async () => {
    mockAuthFetch.mockResolvedValue(
      mockJsonResponse({
        id: "c3",
        email: "carol@example.com",
        role: "viewer",
        invited_at: "2025-02-01",
        email_sent: true,
      }),
    );
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useInviteCollaborator(), { wrapper });

    let returned: Awaited<ReturnType<typeof result.current.mutateAsync>> | undefined;
    await act(async () => {
      returned = await result.current.mutateAsync({
        tripId: "trip-1",
        email: "carol@example.com",
        role: "viewer",
      });
    });

    expect(returned?.collaborator).toEqual({
      id: "c3",
      email: "carol@example.com",
      role: "viewer",
      invitedAt: "2025-02-01",
      acceptedAt: null,
      userId: null,
    });
    expect(returned?.emailSent).toBe(true);
    expect(returned?.acceptUrl).toBeUndefined();

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "http://localhost:8090/api/trips/trip-1/invite",
      "test-token",
      {
        method: "POST",
        body: JSON.stringify({ email: "carol@example.com", role: "viewer" }),
      },
    );
  });

  it("surfaces email_sent=false with accept_url when delivery fails", async () => {
    mockAuthFetch.mockResolvedValue(
      mockJsonResponse({
        id: "c4",
        email: "dave@example.com",
        role: "editor",
        invited_at: "2025-02-02",
        email_sent: false,
        accept_url: "https://app.toqui.travel/trips/invite?token=abc123",
      }),
    );
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useInviteCollaborator(), { wrapper });

    let returned: Awaited<ReturnType<typeof result.current.mutateAsync>> | undefined;
    await act(async () => {
      returned = await result.current.mutateAsync({
        tripId: "trip-1",
        email: "dave@example.com",
        role: "editor",
      });
    });

    expect(returned?.emailSent).toBe(false);
    expect(returned?.acceptUrl).toBe("https://app.toqui.travel/trips/invite?token=abc123");
  });

  it("throws when API returns non-ok response", async () => {
    mockAuthFetch.mockResolvedValue(mockJsonResponse("User already invited", false, 409));
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useInviteCollaborator(), { wrapper });

    await expect(
      act(async () => {
        await result.current.mutateAsync({
          tripId: "trip-1",
          email: "bob@example.com",
          role: "editor",
        });
      }),
    ).rejects.toThrow();
  });
});

// ---------------------------------------------------------------------------
// useRemoveCollaborator
// ---------------------------------------------------------------------------

describe("useRemoveCollaborator", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.accessToken = "test-token";
  });

  it("calls the correct DELETE endpoint", async () => {
    mockAuthFetch.mockResolvedValue(mockJsonResponse({}, true));
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useRemoveCollaborator(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ tripId: "trip-1", email: "bob@example.com" });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "http://localhost:8090/api/trips/trip-1/collaborators/bob%40example.com",
      "test-token",
      { method: "DELETE" },
    );
  });

  it("throws when API returns non-ok response", async () => {
    mockAuthFetch.mockResolvedValue(mockJsonResponse(null, false));
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useRemoveCollaborator(), { wrapper });

    await expect(
      act(async () => {
        await result.current.mutateAsync({ tripId: "trip-1", email: "bob@example.com" });
      }),
    ).rejects.toThrow("Failed to remove collaborator");
  });
});

// ---------------------------------------------------------------------------
// useAcceptInvite
// ---------------------------------------------------------------------------

describe("useAcceptInvite", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.accessToken = "test-token";
  });

  it("calls the correct endpoint and returns trip data", async () => {
    const tripData = { trip: { id: "t1", title: "Paris Trip", description: "Fun!", destinationCountry: "FR", invitedBy: "alice@example.com" } };
    mockAuthFetch.mockResolvedValue(mockJsonResponse(tripData));

    const { result } = renderHook(() => useAcceptInvite());

    let response: unknown;
    await act(async () => {
      response = await result.current.acceptInvite("abc-token");
    });

    expect(response).toEqual(tripData);
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "http://localhost:8090/api/trips/accept-invite",
      "test-token",
      {
        method: "POST",
        body: JSON.stringify({ token: "abc-token" }),
      },
    );
  });

  it("throws 'expired' error for 410 status", async () => {
    mockAuthFetch.mockResolvedValue(mockJsonResponse("Gone", false, 410));

    const { result } = renderHook(() => useAcceptInvite());

    await expect(
      act(async () => {
        await result.current.acceptInvite("expired-token");
      }),
    ).rejects.toThrow("expired");
  });

  it("throws 'already_accepted' error for 409 status", async () => {
    mockAuthFetch.mockResolvedValue(mockJsonResponse("Conflict", false, 409));

    const { result } = renderHook(() => useAcceptInvite());

    await expect(
      act(async () => {
        await result.current.acceptInvite("used-token");
      }),
    ).rejects.toThrow("already_accepted");
  });

  it("throws generic error for other failures", async () => {
    mockAuthFetch.mockResolvedValue(mockJsonResponse("Server Error", false, 500));

    const { result } = renderHook(() => useAcceptInvite());

    await expect(
      act(async () => {
        await result.current.acceptInvite("bad-token");
      }),
    ).rejects.toThrow();
  });
});
