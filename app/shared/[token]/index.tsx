import { useEffect } from "react";
import { View, Text, StyleSheet, ScrollView, ActivityIndicator, Pressable, Platform } from "react-native";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import { MapPin, Calendar } from "lucide-react-native";
import { useTheme } from "@/lib/theme";
import { useAuth } from "@/lib/auth";
import type { ThemeColors } from "@/lib/theme";

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

function getPendingRef(): string | null {
  if (Platform.OS === "web" && typeof window !== "undefined") {
    return sessionStorage.getItem("toqui_pending_ref");
  }
  return null;
}

interface CtaCardProps {
  colors: ThemeColors;
  heading: string;
  subtitle: string;
  buttonLabel: string;
  onPress: () => void;
  socialProof?: string;
}

function CtaCard({ colors, heading, subtitle, buttonLabel, onPress, socialProof }: CtaCardProps) {
  return (
    <View style={[styles.ctaCard, { backgroundColor: colors.surface, borderColor: colors.border }]}>
      <Text style={[styles.ctaHeading, { color: colors.textPrimary }]}>{heading}</Text>
      <Text style={[styles.ctaSubtitle, { color: colors.textSecondary }]}>{subtitle}</Text>
      <Pressable
        style={[styles.ctaButton, { backgroundColor: colors.accent }]}
        onPress={onPress}
        accessibilityRole="button"
        accessibilityLabel={buttonLabel}
      >
        <Text style={styles.ctaButtonText}>{buttonLabel} →</Text>
      </Pressable>
      {socialProof && (
        <Text style={[styles.ctaSocialProof, { color: colors.textTertiary }]}>{socialProof}</Text>
      )}
    </View>
  );
}

export default function SharedTripScreen() {
  const { token } = useLocalSearchParams<{ token: string }>();
  const { colors } = useTheme();
  const { accessToken } = useAuth();
  const router = useRouter();
  const refCode = getPendingRef();

  const { data, isLoading, error } = useQuery<SharedTripResponse>({
    queryKey: ["shared-trip", token],
    queryFn: async () => {
      const res = await fetch(`${getConfig().apiUrl}/shared/${encodeURIComponent(token!)}`);
      if (!res.ok) throw new Error(`Failed to load shared trip (${res.status})`);
      return res.json();
    },
    enabled: !!token,
  });

  const tripTitle = data?.trip?.title;

  useEffect(() => {
    if (tripTitle && typeof document !== "undefined") {
      document.title = `${tripTitle} — Toqui`;
    }
  }, [tripTitle]);

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
      {refCode && (
        <View style={[styles.refBanner, { backgroundColor: colors.accentSoft, borderColor: colors.border }]}>
          <Text style={[styles.refBannerText, { color: colors.textPrimary }]}>
            🎁 Your friend shared this trip with you. Sign up and you both get a bonus.
          </Text>
        </View>
      )}

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

      {accessToken ? (
        <CtaCard
          colors={colors}
          heading="🗺️  Like this itinerary?"
          subtitle="Start a similar trip in your account"
          buttonLabel="Plan a Similar Trip"
          onPress={() => router.push("/trips/new" as never)}
        />
      ) : (
        <CtaCard
          colors={colors}
          heading="✈️  Plan your own dream trip"
          subtitle="Toqui's AI builds personalized itineraries in minutes — free to try."
          buttonLabel="Start Planning for Free"
          onPress={() => router.push("/" as never)}
          socialProof="Join 1,000+ travelers already using Toqui"
        />
      )}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  content: { padding: 16, paddingBottom: 40 },
  center: { flex: 1, justifyContent: "center", alignItems: "center" },
  errorText: { fontSize: 16 },
  refBanner: {
    borderRadius: 10,
    padding: 12,
    marginBottom: 16,
    borderWidth: 1,
  },
  refBannerText: { fontSize: 14, lineHeight: 20 },
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
  ctaCard: {
    borderRadius: 14,
    padding: 20,
    marginTop: 24,
    borderWidth: 1,
    alignItems: "center",
  },
  ctaHeading: { fontSize: 20, fontWeight: "700", marginBottom: 8, textAlign: "center" },
  ctaSubtitle: { fontSize: 14, lineHeight: 20, textAlign: "center", marginBottom: 16 },
  ctaButton: {
    borderRadius: 10,
    paddingVertical: 14,
    paddingHorizontal: 28,
    alignItems: "center",
    width: "100%",
    marginBottom: 12,
  },
  ctaButtonText: { color: "#fff", fontSize: 16, fontWeight: "600" },
  ctaSocialProof: { fontSize: 12, textAlign: "center" },
});
