import { View, Text, StyleSheet, Platform } from "react-native";
import { useMemo } from "react";
import { useTheme } from "@/lib/theme";
import { getDayColor } from "./colors";
import type { Itinerary, ItineraryDay, ItineraryItem } from "@gen/toqui/v1/trip_pb";

// MapLibre is only available on native platforms.
// On web, we show a placeholder (MapLibre GL JS could be added separately).
let MapView: React.ComponentType<any> | null = null;
let Camera: React.ComponentType<any> | null = null;
let PointAnnotation: React.ComponentType<any> | null = null;

if (Platform.OS !== "web") {
  try {
    const MapLibre = require("@maplibre/maplibre-react-native");
    MapLibre.setAccessToken(null); // MapLibre is free, no token needed
    MapView = MapLibre.MapView;
    Camera = MapLibre.Camera;
    PointAnnotation = MapLibre.PointAnnotation;
  } catch {
    // MapLibre not available — show placeholder
  }
}

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
  const { colors, isDark } = useTheme();
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

  // Web fallback — MapLibre GL JS would need a separate implementation
  if (Platform.OS === "web" || !MapView || !Camera || !PointAnnotation) {
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

  const styleUrl = isDark
    ? "https://basemaps.cartocdn.com/gl/dark-matter-gl-style/style.json"
    : "https://basemaps.cartocdn.com/gl/positron-gl-style/style.json";

  return (
    <View style={[styles.container, { height }]}>
      <MapView
        style={styles.map}
        styleURL={styleUrl}
        attributionEnabled={false}
        logoEnabled={false}
      >
        {bounds && (
          <Camera
            bounds={{
              sw: bounds.sw,
              ne: bounds.ne,
              paddingLeft: 40,
              paddingRight: 40,
              paddingTop: 40,
              paddingBottom: 40,
            }}
            animationDuration={0}
          />
        )}
        {markers.map((marker) => (
          <PointAnnotation
            key={marker.id}
            id={marker.id}
            coordinate={marker.coordinate}
            title={marker.title}
          >
            <View style={[styles.marker, { backgroundColor: marker.color, borderColor: colors.surface }]}>
              <Text style={[styles.markerText, { color: colors.surface }]}>{marker.dayNumber}</Text>
            </View>
          </PointAnnotation>
        ))}
      </MapView>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { borderRadius: 12, overflow: "hidden", marginBottom: 16 },
  map: { flex: 1 },
  placeholder: {
    borderRadius: 12,
    justifyContent: "center",
    alignItems: "center",
    marginBottom: 16,
  },
  placeholderText: { fontSize: 14 },
  placeholderSubtext: { fontSize: 12, marginTop: 4 },
  marker: {
    width: 28,
    height: 28,
    borderRadius: 14,
    justifyContent: "center",
    alignItems: "center",
    borderWidth: 2,
  },
  markerText: { fontSize: 12, fontWeight: "700" },
});
