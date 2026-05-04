/**
 * Itinerary map — public entry point.
 *
 * Gates the heavy `@maplibre/maplibre-react-native` dependency behind a
 * `React.lazy` + `<Suspense>` boundary so that the maplibre module is:
 *
 *   - kept out of the web bundle entirely. Metro resolves the lazy import to
 *     `ItineraryMapNative.web.tsx` (a noop stub) on web. The parent also
 *     short-circuits to a placeholder before the <Suspense> boundary on web,
 *     so the stub chunk is never even fetched.
 *   - only evaluated on iOS/Android the first time a trip with at least one
 *     mappable location is opened, instead of when the trip detail screen
 *     module is first imported (which is the cost paid by a top-level
 *     `require`).
 *
 * On web the user sees a small "Available on iOS and Android" placeholder.
 * MapLibre GL JS is not currently wired up for web — see toqui#205.
 */

import { lazy, Suspense, useMemo } from "react";
import { View, Text, StyleSheet, Platform, ActivityIndicator } from "react-native";
import { useTheme } from "@/lib/theme";
import { getDayColor } from "./colors";
import type { Itinerary } from "@gen/toqui/v1/trip_pb";

// `React.lazy` defers this import until the component is rendered. On web,
// the parent short-circuits to the placeholder before mounting <Suspense>, so
// this dynamic import is never triggered and Metro never reaches into the
// maplibre module graph for web bundles.
const ItineraryMapNative = lazy(() => import("./ItineraryMapNative"));

interface MarkerData {
  id: string;
  coordinate: [number, number]; // [lng, lat]
  title: string;
  color: string;
  dayNumber: number;
}

function extractMarkers(itinerary: Itinerary): MarkerData[] {
  const markers: MarkerData[] = [];
  for (const day of itinerary.days) {
    const color = getDayColor(day.dayNumber);
    for (const item of day.items) {
      if (item.location && item.location.latitude !== 0 && item.location.longitude !== 0) {
        markers.push({
          id: item.id,
          coordinate: [item.location.longitude, item.location.latitude],
          title: item.title,
          color,
          dayNumber: day.dayNumber,
        });
      }
    }
  }
  return markers;
}

function computeBounds(markers: MarkerData[]): { sw: [number, number]; ne: [number, number] } | null {
  if (markers.length === 0) return null;
  let minLng = Infinity, maxLng = -Infinity, minLat = Infinity, maxLat = -Infinity;
  for (const m of markers) {
    minLng = Math.min(minLng, m.coordinate[0]);
    maxLng = Math.max(maxLng, m.coordinate[0]);
    minLat = Math.min(minLat, m.coordinate[1]);
    maxLat = Math.max(maxLat, m.coordinate[1]);
  }
  return { sw: [minLng, minLat], ne: [maxLng, maxLat] };
}

interface ItineraryMapProps {
  itinerary: Itinerary;
  height?: number;
}

export function ItineraryMap({ itinerary, height = 300 }: ItineraryMapProps) {
  const { colors } = useTheme();
  const markers = useMemo(() => extractMarkers(itinerary), [itinerary]);
  const bounds = useMemo(() => computeBounds(markers), [markers]);

  if (markers.length === 0) {
    return (
      <View style={[styles.placeholder, { height, backgroundColor: colors.surfaceTertiary }]}>
        <Text style={[styles.placeholderText, { color: colors.textTertiary }]}>
          No locations to display on map
        </Text>
      </View>
    );
  }

  // Web fallback — MapLibre GL JS web rendering is not yet wired up. We
  // short-circuit BEFORE the Suspense/lazy boundary so the maplibre module
  // is never reached by the web bundler.
  if (Platform.OS === "web") {
    return (
      <View style={[styles.placeholder, { height, backgroundColor: colors.surfaceTertiary }]}>
        <Text style={[styles.placeholderText, { color: colors.textTertiary }]}>
          Map view ({markers.length} locations)
        </Text>
        <Text style={[styles.placeholderSubtext, { color: colors.textTertiary }]}>
          Available on iOS and Android
        </Text>
      </View>
    );
  }

  // Native: lazy-load the maplibre-using renderer. The Suspense fallback
  // is sized to the final map so layout doesn't shift when the chunk loads.
  return (
    <Suspense
      fallback={
        <View style={[styles.placeholder, { height, backgroundColor: colors.surfaceTertiary }]}>
          <ActivityIndicator color={colors.textTertiary} />
        </View>
      }
    >
      <ItineraryMapNative markers={markers} bounds={bounds} height={height} />
    </Suspense>
  );
}

const styles = StyleSheet.create({
  placeholder: {
    borderRadius: 12,
    justifyContent: "center",
    alignItems: "center",
    marginBottom: 16,
  },
  placeholderText: { fontSize: 14 },
  placeholderSubtext: { fontSize: 12, marginTop: 4 },
});
