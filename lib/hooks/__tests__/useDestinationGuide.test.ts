import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";

// useDestinationGuide is the lookup that matches a trip's
// destination_country to the relevant /api/guides entry. A bug here
// either:
//   - shows the wrong country's guide (UX confusion + privacy concern
//     if the user's destination leaks into a different guide's
//     analytics)
//   - silently fails to surface a guide that exists
//
// The matching is case-insensitive (country codes like "JP" / "jp"
// shouldn't matter to the lookup); pin that.

vi.mock("@/lib/config", () => ({
  getConfig: () => ({ apiUrl: "http://api.test" }),
}));

import { useDestinationGuide } from "../useDestinationGuide";

function makeWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { wrapper };
}

function jsonOk(data: unknown) {
  return { ok: true, status: 200, json: () => Promise.resolve(data) };
}

const fixtureGuides = {
  guides: [
    {
      slug: "japan-food",
      title: "Eating Through Japan",
      persona_name: "Akari",
      persona_specialty: "izakaya",
      destination: "Tokyo",
      country: "JP",
      theme: "food",
      excerpt: "...",
      content: "...",
      cta_text: "Plan a trip",
      cta_url: "/trips/new",
    },
    {
      slug: "italy-food",
      title: "Pasta Tour",
      persona_name: "Marco",
      persona_specialty: "trattoria",
      destination: "Rome",
      country: "IT",
      theme: "food",
      excerpt: "...",
      content: "...",
      cta_text: "Plan a trip",
      cta_url: "/trips/new",
    },
  ],
  total: 2,
};

beforeEach(() => {
  vi.clearAllMocks();
  globalThis.fetch = vi.fn();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useDestinationGuide", () => {
  it("returns null + not loading when destinationCountry is undefined", () => {
    // Disabled query: don't even fetch /api/guides.
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useDestinationGuide(undefined), { wrapper });

    expect(result.current.guide).toBeNull();
    expect(result.current.isLoading).toBe(false);
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("returns null + not loading when destinationCountry is empty string", () => {
    // The hook is enabled only when destinationCountry is truthy. An
    // empty string from a Trip with unset destination falls into the
    // disabled path.
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useDestinationGuide(""), { wrapper });

    expect(result.current.guide).toBeNull();
    expect(result.current.isLoading).toBe(false);
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("matches the guide for the given country code", async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(jsonOk(fixtureGuides));

    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useDestinationGuide("JP"), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.guide?.slug).toBe("japan-food");
    expect(result.current.guide?.country).toBe("JP");
  });

  it("matches case-insensitively (lowercase input → uppercase guide)", async () => {
    // Real-world: trip.destination_country comes back as "jp" from one
    // path and "JP" from another. Pin that lookup is robust to either.
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(jsonOk(fixtureGuides));

    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useDestinationGuide("jp"), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.guide?.slug).toBe("japan-food");
  });

  it("returns null when no guide matches the country", async () => {
    // Trip to a country we don't have a guide for yet — null, NOT
    // the first guide in the list (which would be a real bug if the
    // .find() were replaced with [0]).
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(jsonOk(fixtureGuides));

    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useDestinationGuide("FR"), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.guide).toBeNull();
  });

  it("returns null when fetch fails (graceful degradation)", async () => {
    // Backend down → render the trip detail screen without a guide
    // rather than blocking on it.
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: () => Promise.resolve({}),
    });

    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useDestinationGuide("JP"), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.guide).toBeNull();
  });

  it("calls /api/guides exactly once even with multiple consumers (shared cache key)", async () => {
    // Pin that the queryKey is fixed (`destinationGuides`) regardless
    // of input — the entire guide list is fetched once and filtered
    // client-side. A future change to per-country queries would
    // multiply the request rate by the number of trip detail screens
    // open, so this contract is worth pinning.
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(jsonOk(fixtureGuides));

    const { wrapper } = makeWrapper();
    // Render two consumers in the same wrapper.
    renderHook(() => useDestinationGuide("JP"), { wrapper });
    renderHook(() => useDestinationGuide("IT"), { wrapper });

    await waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledTimes(1);
    });
    expect(globalThis.fetch).toHaveBeenCalledWith("http://api.test/api/guides");
  });
});
