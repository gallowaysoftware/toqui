/** Consistent day color palette for map markers and legend */
export const DAY_COLORS = [
  "#3B82F6", // Day 1: blue
  "#10B981", // Day 2: green
  "#F59E0B", // Day 3: amber
  "#EF4444", // Day 4: red
  "#8B5CF6", // Day 5: violet
  "#EC4899", // Day 6: pink
  "#06B6D4", // Day 7: cyan
  "#F97316", // Day 8: orange
  "#14B8A6", // Day 9: teal
  "#6366F1", // Day 10: indigo
];

/** Get the marker color for a given day number (1-indexed), wraps around */
export function getDayColor(dayNumber: number): string {
  return DAY_COLORS[(dayNumber - 1) % DAY_COLORS.length];
}
