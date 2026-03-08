import { describe, it, expect, beforeEach } from "vitest";
import { useLocationStore } from "./location-store";

describe("useLocationStore", () => {
  beforeEach(() => {
    // Reset store to initial state between tests
    useLocationStore.setState({
      latitude: null,
      longitude: null,
      accuracy: null,
      lastUpdated: null,
      isTracking: false,
    });
  });

  it("has correct initial state", () => {
    const state = useLocationStore.getState();
    expect(state.latitude).toBeNull();
    expect(state.longitude).toBeNull();
    expect(state.accuracy).toBeNull();
    expect(state.lastUpdated).toBeNull();
    expect(state.isTracking).toBe(false);
  });

  it("sets location with coordinates and accuracy", () => {
    const before = new Date();
    useLocationStore.getState().setLocation(43.6532, -79.3832, 10);
    const after = new Date();

    const state = useLocationStore.getState();
    expect(state.latitude).toBe(43.6532);
    expect(state.longitude).toBe(-79.3832);
    expect(state.accuracy).toBe(10);
    expect(state.lastUpdated).not.toBeNull();
    expect(state.lastUpdated!.getTime()).toBeGreaterThanOrEqual(before.getTime());
    expect(state.lastUpdated!.getTime()).toBeLessThanOrEqual(after.getTime());
  });

  it("updates location when called multiple times", () => {
    useLocationStore.getState().setLocation(43.6532, -79.3832, 10);
    useLocationStore.getState().setLocation(35.6762, 139.6503, 5);

    const state = useLocationStore.getState();
    expect(state.latitude).toBe(35.6762);
    expect(state.longitude).toBe(139.6503);
    expect(state.accuracy).toBe(5);
  });

  it("sets tracking state to true", () => {
    useLocationStore.getState().setTracking(true);
    expect(useLocationStore.getState().isTracking).toBe(true);
  });

  it("sets tracking state to false", () => {
    useLocationStore.getState().setTracking(true);
    useLocationStore.getState().setTracking(false);
    expect(useLocationStore.getState().isTracking).toBe(false);
  });

  it("does not affect tracking when setting location", () => {
    useLocationStore.getState().setTracking(true);
    useLocationStore.getState().setLocation(43.6532, -79.3832, 10);

    // Tracking should remain true since setLocation doesn't modify it
    expect(useLocationStore.getState().isTracking).toBe(true);
  });
});
