/**
 * Native-only MapLibre renderer for the itinerary map (iOS + Android).
 *
 * This file is loaded lazily by `ItineraryMap.tsx` via `React.lazy` so that
 * `@maplibre/maplibre-react-native` (a heavy WebGL/native module) is only
 * evaluated when a trip with mappable locations is actually opened — not at
 * trip-detail-screen module-load time. That defers maplibre's native-bridge
 * setup and JS module evaluation off the cold-start critical path.
 *
 * Metro's web target resolves the lazy import to `ItineraryMapNative.web.tsx`
 * (a noop stub), so the maplibre package never reaches the web bundle.
 */

import { View, Text, StyleSheet } from "react-native";
import { useTheme } from "@/lib/theme";
// eslint-disable-next-line @typescript-eslint/no-require-imports
const MapLibre = require("@maplibre/maplibre-react-native");

// MapLibre is free to use — no access token required. The native module still
// exposes `setAccessToken`, so we explicitly pass null to silence its warning.
MapLibre.setAccessToken(null);

const MapView = MapLibre.MapView;
const Camera = MapLibre.Camera;
const PointAnnotation = MapLibre.PointAnnotation;

interface MarkerData {
  id: string;
  coordinate: [number, number]; // [lng, lat]
  title: string;
  color: string;
  dayNumber: number;
}

interface ItineraryMapNativeProps {
  markers: MarkerData[];
  bounds: { sw: [number, number]; ne: [number, number] } | null;
  height: number;
}

function ItineraryMapNative({ markers, bounds, height }: ItineraryMapNativeProps) {
  const { colors, isDark } = useTheme();

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

// Default export so `React.lazy(() => import("./ItineraryMapNative"))` works.
export default ItineraryMapNative;

const styles = StyleSheet.create({
  container: { borderRadius: 12, overflow: "hidden", marginBottom: 16 },
  map: { flex: 1 },
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
