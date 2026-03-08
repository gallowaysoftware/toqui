import { describe, it, expect, beforeEach, vi, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useUsage } from "./useUsage";

describe("useUsage", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-03-06T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("starts with 0 used and 30 remaining", () => {
    const { result } = renderHook(() => useUsage());
    expect(result.current.used).toBe(0);
    expect(result.current.remaining).toBe(30);
    expect(result.current.limit).toBe(30);
    expect(result.current.isAtLimit).toBe(false);
    expect(result.current.isWarning).toBe(false);
  });

  it("increments used count on recordMessage", () => {
    const { result } = renderHook(() => useUsage());

    act(() => {
      result.current.recordMessage();
    });

    expect(result.current.used).toBe(1);
    expect(result.current.remaining).toBe(29);
  });

  it("persists usage to localStorage", () => {
    const { result } = renderHook(() => useUsage());

    act(() => {
      result.current.recordMessage();
    });

    const stored = JSON.parse(localStorage.getItem("toqui_daily_usage")!);
    expect(stored.count).toBe(1);
    expect(stored.date).toBe("2026-03-06");
  });

  it("loads existing usage from localStorage", () => {
    localStorage.setItem("toqui_daily_usage", JSON.stringify({ date: "2026-03-06", count: 15 }));

    const { result } = renderHook(() => useUsage());
    expect(result.current.used).toBe(15);
    expect(result.current.remaining).toBe(15);
  });

  it("resets count when date changes", () => {
    localStorage.setItem("toqui_daily_usage", JSON.stringify({ date: "2026-03-05", count: 25 }));

    const { result } = renderHook(() => useUsage());
    expect(result.current.used).toBe(0);
    expect(result.current.remaining).toBe(30);
  });

  it("shows warning when remaining <= 5", () => {
    localStorage.setItem("toqui_daily_usage", JSON.stringify({ date: "2026-03-06", count: 25 }));

    const { result } = renderHook(() => useUsage());
    expect(result.current.isWarning).toBe(true);
    expect(result.current.remaining).toBe(5);
    expect(result.current.isAtLimit).toBe(false);
  });

  it("shows at limit when remaining is 0", () => {
    localStorage.setItem("toqui_daily_usage", JSON.stringify({ date: "2026-03-06", count: 30 }));

    const { result } = renderHook(() => useUsage());
    expect(result.current.isAtLimit).toBe(true);
    expect(result.current.isWarning).toBe(false);
    expect(result.current.remaining).toBe(0);
  });

  it("markExhausted sets used to limit", () => {
    const { result } = renderHook(() => useUsage());

    act(() => {
      result.current.markExhausted();
    });

    expect(result.current.used).toBe(30);
    expect(result.current.isAtLimit).toBe(true);
    expect(result.current.remaining).toBe(0);
  });

  it("remaining never goes below 0", () => {
    localStorage.setItem("toqui_daily_usage", JSON.stringify({ date: "2026-03-06", count: 35 }));

    const { result } = renderHook(() => useUsage());
    expect(result.current.remaining).toBe(0);
    expect(result.current.isAtLimit).toBe(true);
  });

  it("handles corrupt localStorage gracefully", () => {
    localStorage.setItem("toqui_daily_usage", "not-json");

    const { result } = renderHook(() => useUsage());
    expect(result.current.used).toBe(0);
    expect(result.current.remaining).toBe(30);
  });
});
