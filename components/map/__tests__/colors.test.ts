import { describe, it, expect } from "vitest";
import { DAY_COLORS, getDayColor } from "../colors";

describe("DAY_COLORS", () => {
  it("contains exactly 10 colors", () => {
    expect(DAY_COLORS).toHaveLength(10);
  });

  it("every entry is a valid hex color", () => {
    for (const color of DAY_COLORS) {
      expect(color).toMatch(/^#[0-9A-Fa-f]{6}$/);
    }
  });

  it("contains no duplicate colors", () => {
    const unique = new Set(DAY_COLORS.map((c) => c.toLowerCase()));
    expect(unique.size).toBe(DAY_COLORS.length);
  });
});

describe("getDayColor", () => {
  it("returns the first color for day 1 (1-indexed)", () => {
    expect(getDayColor(1)).toBe(DAY_COLORS[0]);
  });

  it("returns the last color for day 10", () => {
    expect(getDayColor(10)).toBe(DAY_COLORS[9]);
  });

  it("wraps around: day 11 equals day 1", () => {
    expect(getDayColor(11)).toBe(getDayColor(1));
  });

  it("wraps around: day 20 equals day 10", () => {
    expect(getDayColor(20)).toBe(getDayColor(10));
  });

  it("wraps correctly for high day numbers (day 101)", () => {
    // day 101 -> (101-1) % 10 = 0 -> DAY_COLORS[0]
    expect(getDayColor(101)).toBe(DAY_COLORS[0]);
  });

  it("returns a valid hex string for every day in a 30-day trip", () => {
    for (let day = 1; day <= 30; day++) {
      const color = getDayColor(day);
      expect(color).toMatch(/^#[0-9A-Fa-f]{6}$/);
    }
  });

  it("cycles through all 10 colors before repeating", () => {
    const first10 = Array.from({ length: 10 }, (_, i) => getDayColor(i + 1));
    // All 10 should be distinct
    expect(new Set(first10).size).toBe(10);
    // Next 10 should be the same sequence
    const next10 = Array.from({ length: 10 }, (_, i) => getDayColor(i + 11));
    expect(next10).toEqual(first10);
  });
});
