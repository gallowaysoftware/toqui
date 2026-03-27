import { View, Text, StyleSheet, ScrollView, ActivityIndicator, Pressable, Linking } from "react-native";
import { useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import { MapPin, Calendar } from "lucide-react-native";
import { useTheme } from "@/lib/theme";

import { getConfig } from "@/lib/config";

interface SharedTripInfo {
  title: string;
  description?: string;
  destination_country?: string;
  status: string;
  start_date?: string;
  end_date?: string;
}

interface SharedItineraryItem {
  title: string;
  type?: string;
  description?: string;
}

interface SharedItineraryDay {
  day_number: number;
  items: SharedItineraryItem[];
}

interface SharedTripResponse {
  trip: SharedTripInfo;
  itinerary: SharedItineraryDay[];
}

const dayColors = [
  "#3B82F6", "#10B981", "#F59E0B", "#EF4444", "#8B5CF6",
  "#EC4899", "#06B6D4", "#F97316", "#14B8A6", "#6366F1",
];

export default function SharedTripScreen() {
  const { token } = useLocalSearchParams<{ token: string }>();
  const { colors } = useTheme();

  const { data, isLoading, error } = useQuery<SharedTripResponse>({
    queryKey: ["shared-trip", token],
    queryFn: async () => {
      const res = await fetch(`${getConfig().apiUrl}/shared/${encodeURIComponent(token!)}`);
      if (!res.ok) throw new Error(`Failed to load shared trip (${res.status})`);
      return res.json();
    },
    enabled: !!token,
  });

  if (isLoading) {
    return <View style={styles.center}><ActivityIndicator size="large" color={colors.accent} /></View>;
  }

  if (error || !data) {
    return (
      <View style={[styles.center, { backgroundColor: colors.surface }]}>
        <Text style={[styles.errorText, { color: colors.error }]}>
          {error instanceof Error ? error.message : "Trip not found"}
        </Text>
      </View>
    );
  }

  const { trip, itinerary } = data;

  return (
    <ScrollView style={[styles.container, { backgroundColor: colors.surfaceSecondary }]} contentContainerStyle={styles.content}>
      <View style={[styles.header, { backgroundColor: colors.surface, borderColor: colors.border }]}>
        <Text style={[styles.title, { color: colors.textPrimary }]}>{trip.title}</Text>
        {trip.description && (
          <Text style={[styles.description, { color: colors.textSecondary }]}>{trip.description}</Text>
        )}
        <View style={styles.metaRow}>
          {trip.destination_country && (
            <View style={styles.metaChip}>
              <MapPin color={colors.textTertiary} size={14} />
              <Text style={[styles.metaText, { color: colors.textTertiary }]}>{trip.destination_country}</Text>
            </View>
          )}
          {trip.start_date && (
            <View style={styles.metaChip}>
              <Calendar color={colors.textTertiary} size={14} />
              <Text style={[styles.metaText, { color: colors.textTertiary }]}>
                {trip.start_date}{trip.end_date ? ` — ${trip.end_date}` : ""}
              </Text>
            </View>
          )}
        </View>
      </View>

      {itinerary.length > 0 ? (
        itinerary
          .slice()
          .sort((a, b) => a.day_number - b.day_number)
          .map((day, i) => {
            const color = dayColors[i % dayColors.length]!;
            return (
              <View key={day.day_number} style={styles.daySection}>
                <View style={[styles.dayHeader, { borderLeftColor: color }]}>
                  <View style={[styles.dayBadge, { backgroundColor: color }]}>
                    <Text style={styles.dayBadgeText}>Day {day.day_number}</Text>
                  </View>
                </View>
                {day.items.map((item, j) => (
                  <View key={j} style={[styles.itemCard, { backgroundColor: colors.surface, borderColor: colors.border }]}>
                    <Text style={[styles.itemTitle, { color: colors.textPrimary }]}>{item.title}</Text>
                    {item.description && (
                      <Text style={[styles.itemDesc, { color: colors.textSecondary }]}>{item.description}</Text>
                    )}
                    {item.type && (
                      <Text style={[styles.itemType, { color }]}>{item.type}</Text>
                    )}
                  </View>
                ))}
              </View>
            );
          })
      ) : (
        <Text style={[styles.emptyText, { color: colors.textTertiary }]}>No itinerary items yet</Text>
      )}

      <Pressable
        style={[styles.ctaButton, { backgroundColor: colors.accent }]}
        onPress={() => Linking.openURL("https://toqui.travel")}
      >
        <Text style={styles.ctaText}>Plan your own trip with Toqui</Text>
      </Pressable>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  content: { padding: 16, paddingBottom: 40 },
  center: { flex: 1, justifyContent: "center", alignItems: "center" },
  errorText: { fontSize: 16 },
  header: { borderRadius: 12, padding: 16, marginBottom: 20, borderWidth: 1 },
  title: { fontSize: 24, fontWeight: "bold", marginBottom: 8 },
  description: { fontSize: 15, lineHeight: 22, marginBottom: 12 },
  metaRow: { flexDirection: "row", flexWrap: "wrap", gap: 12 },
  metaChip: { flexDirection: "row", alignItems: "center", gap: 4 },
  metaText: { fontSize: 13 },
  daySection: { marginBottom: 16 },
  dayHeader: { borderLeftWidth: 4, paddingLeft: 10, marginBottom: 8 },
  dayBadge: { paddingHorizontal: 10, paddingVertical: 4, borderRadius: 12, alignSelf: "flex-start" },
  dayBadgeText: { color: "#fff", fontSize: 12, fontWeight: "700" },
  itemCard: { borderRadius: 10, padding: 12, marginBottom: 6, marginLeft: 14, borderWidth: 1 },
  itemTitle: { fontSize: 14, fontWeight: "600" },
  itemDesc: { fontSize: 13, marginTop: 4, lineHeight: 18 },
  itemType: { fontSize: 11, fontWeight: "500", marginTop: 4, textTransform: "capitalize" },
  emptyText: { fontSize: 14, textAlign: "center", paddingTop: 20 },
  ctaButton: { borderRadius: 12, padding: 16, alignItems: "center", marginTop: 24 },
  ctaText: { color: "#fff", fontSize: 16, fontWeight: "600" },
});
