import { View, Text, StyleSheet, Pressable, FlatList, ActivityIndicator, ScrollView } from "react-native";
import { useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import { Plus, MapPin, ChevronRight, Crown, Plane, AlertCircle } from "lucide-react-native";
import { useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth";
import { useGoogleAuth } from "@/lib/google-auth";
import { useTrips } from "@/lib/hooks/useTrips";
import { TripStatus } from "@gen/toqui/v1/trip_pb";
import type { Trip } from "@gen/toqui/v1/trip_pb";

const DESTINATIONS = [
  { key: "tokyo", flag: "\u{1F1EF}\u{1F1F5}", title: "Tokyo" },
  { key: "paris", flag: "\u{1F1EB}\u{1F1F7}", title: "Paris" },
  { key: "rome", flag: "\u{1F1EE}\u{1F1F9}", title: "Rome" },
  { key: "bali", flag: "\u{1F1EE}\u{1F1E9}", title: "Bali" },
  { key: "newYork", flag: "\u{1F1FA}\u{1F1F8}", title: "New York" },
] as const;

function TripCard({ trip, onPress }: { trip: Trip; onPress: () => void }) {
  const { t } = useTranslation();
  const statusConfig: Record<number, { labelKey: string; color: string }> = {
    [TripStatus.PLANNING]: { labelKey: "trips.status.planning", color: "#3b82f6" },
    [TripStatus.ACTIVE]: { labelKey: "trips.status.active", color: "#22c55e" },
    [TripStatus.COMPLETED]: { labelKey: "trips.status.completed", color: "#9ca3af" },
  };
  const { labelKey, color: statusColor } = statusConfig[trip.status] ?? { labelKey: "trips.status.planning", color: "#9ca3af" };
  const statusLabel = t(labelKey);

  return (
    <Pressable style={styles.tripCard} onPress={onPress}>
      <View style={styles.tripCardContent}>
        <View style={styles.tripCardHeader}>
          <Text style={styles.tripTitle} numberOfLines={1}>{trip.title}</Text>
          <View style={[styles.statusBadge, { backgroundColor: statusColor }]}>
            <Text style={styles.statusText}>{statusLabel}</Text>
          </View>
          {trip.isUnlocked && (
            <View style={styles.proBadge}>
              <Crown color="#fff" size={10} />
              <Text style={styles.proBadgeText}>{t("trips.proBadge")}</Text>
            </View>
          )}
        </View>
        {trip.description ? (
          <Text style={styles.tripDescription} numberOfLines={2}>{trip.description}</Text>
        ) : null}
        {trip.destinationCountry ? (
          <View style={styles.tripMeta}>
            <MapPin color="#999" size={14} />
            <Text style={styles.tripMetaText}>{trip.destinationCountry}</Text>
          </View>
        ) : null}
      </View>
      <ChevronRight color="#ccc" size={20} />
    </Pressable>
  );
}

export default function TripsScreen() {
  const { t } = useTranslation();
  const { accessToken, isLoading: authLoading } = useAuth();
  const router = useRouter();
  const { signIn, isReady: authReady } = useGoogleAuth();
  const queryClient = useQueryClient();
  const { trips, isLoading: tripsLoading, isError: tripsError } = useTrips();

  if (authLoading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color="#BF4028" />
      </View>
    );
  }

  if (!accessToken) {
    return (
      <ScrollView style={styles.container} contentContainerStyle={styles.signInContent}>
        <Plane color="#BF4028" size={48} style={styles.welcomeIcon} />
        <Text style={styles.signInTitle}>{t("common.appName")}</Text>
        <Text style={styles.signInTagline}>{t("common.tagline")}</Text>

        <View style={styles.valueProps}>
          <Text style={styles.valueProp}>{t("home.valueProps.experts")}</Text>
          <Text style={styles.valueProp}>{t("home.valueProps.itineraries")}</Text>
          <Text style={styles.valueProp}>{t("home.valueProps.export")}</Text>
          <Text style={styles.valueProp}>{t("home.valueProps.free")}</Text>
        </View>

        <Pressable
          style={[styles.primaryButton, !authReady && styles.disabledButton]}
          onPress={signIn}
          disabled={!authReady}
        >
          <Text style={styles.buttonText}>{t("common.getStarted")}</Text>
        </Pressable>
        <Text style={styles.signInNote}>{t("home.signInNote")}</Text>
      </ScrollView>
    );
  }

  if (tripsLoading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color="#BF4028" />
      </View>
    );
  }

  if (tripsError) {
    return (
      <View style={styles.center}>
        <AlertCircle color="#BF4028" size={40} style={styles.errorIcon} />
        <Text style={styles.errorTitle}>{t("trips.loadError")}</Text>
        <Text style={styles.errorSubtitle}>{t("trips.loadErrorSubtitle")}</Text>
        <Pressable
          style={styles.retryButton}
          onPress={() => void queryClient.invalidateQueries({ queryKey: ["trips"] })}
        >
          <Text style={styles.retryButtonText}>{t("common.retry")}</Text>
        </Pressable>
      </View>
    );
  }

  if (!trips || trips.length === 0) {
    return (
      <ScrollView style={styles.container} contentContainerStyle={styles.welcomeContent}>
        <Plane color="#BF4028" size={40} style={styles.welcomeIcon} />
        <Text style={styles.welcomeTitle}>{t("home.welcomeTitle")}</Text>
        <Text style={styles.welcomeSubtitle}>{t("home.welcomeSubtitle")}</Text>

        <View style={styles.destinationList}>
          {DESTINATIONS.map((dest) => (
            <Pressable
              key={dest.key}
              style={styles.destinationCard}
              onPress={() =>
                router.push({
                  pathname: "/trips/new" as never,
                  params: { destination: dest.title },
                })
              }
            >
              <Text style={styles.destinationFlag}>{dest.flag}</Text>
              <View style={styles.destinationInfo}>
                <Text style={styles.destinationName}>{dest.title}</Text>
                <Text style={styles.destinationHook}>
                  {t(`home.destinations.${dest.key}`)}
                </Text>
              </View>
              <ChevronRight color="#ccc" size={18} />
            </Pressable>
          ))}
        </View>

        <Pressable
          style={styles.primaryButton}
          onPress={() => router.push("/trips/new" as never)}
        >
          <Plus color="#fff" size={18} />
          <Text style={styles.buttonText}>{t("trips.newTrip")}</Text>
        </Pressable>
      </ScrollView>
    );
  }

  return (
    <View style={styles.container}>
      <FlatList
        data={trips}
        keyExtractor={(item) => item.id}
        renderItem={({ item }) => (
          <TripCard
            trip={item}
            onPress={() => router.push(`/trips/${item.id}` as never)}
          />
        )}
        contentContainerStyle={styles.listContent}
        ListHeaderComponent={
          <Pressable
            style={styles.newTripButton}
            onPress={() => router.push("/trips/new" as never)}
          >
            <Plus color="#BF4028" size={18} />
            <Text style={styles.newTripText}>{t("trips.newTrip")}</Text>
          </Pressable>
        }
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#f5f5f5" },
  center: { flex: 1, justifyContent: "center", alignItems: "center", padding: 24 },
  signInContent: { flexGrow: 1, justifyContent: "center", padding: 24, alignItems: "center" },
  signInTitle: { fontSize: 36, fontWeight: "bold", color: "#BF4028", marginBottom: 4 },
  signInTagline: { fontSize: 17, color: "#666", textAlign: "center", marginBottom: 28 },
  valueProps: { marginBottom: 32, width: "100%" },
  valueProp: { fontSize: 14, color: "#555", textAlign: "center", lineHeight: 24, marginBottom: 6 },
  signInNote: { fontSize: 12, color: "#999", textAlign: "center", marginTop: 12 },
  title: { fontSize: 32, fontWeight: "bold", color: "#BF4028", marginBottom: 8 },
  subtitle: { fontSize: 16, color: "#666", textAlign: "center", marginBottom: 32 },
  welcomeContent: { padding: 24, alignItems: "center" },
  welcomeIcon: { marginTop: 32, marginBottom: 16 },
  welcomeTitle: { fontSize: 24, fontWeight: "bold", color: "#333", marginBottom: 6, textAlign: "center" },
  welcomeSubtitle: { fontSize: 15, color: "#666", textAlign: "center", marginBottom: 28 },
  destinationList: { width: "100%", marginBottom: 24 },
  destinationCard: {
    flexDirection: "row",
    alignItems: "center",
    backgroundColor: "#fff",
    borderRadius: 12,
    padding: 14,
    marginBottom: 10,
    borderWidth: 1,
    borderColor: "#e0e0e0",
  },
  destinationFlag: { fontSize: 28, marginRight: 12 },
  destinationInfo: { flex: 1 },
  destinationName: { fontSize: 16, fontWeight: "600", color: "#333" },
  destinationHook: { fontSize: 13, color: "#888", marginTop: 2 },
  primaryButton: {
    backgroundColor: "#BF4028",
    borderRadius: 8,
    paddingVertical: 14,
    paddingHorizontal: 24,
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  disabledButton: { opacity: 0.5 },
  buttonText: { color: "#fff", fontSize: 16, fontWeight: "600" },
  listContent: { padding: 16 },
  newTripButton: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: 8,
    padding: 14,
    backgroundColor: "#fff",
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "#BF4028",
    borderStyle: "dashed",
    marginBottom: 12,
  },
  newTripText: { color: "#BF4028", fontSize: 16, fontWeight: "600" },
  tripCard: {
    backgroundColor: "#fff",
    borderRadius: 12,
    padding: 16,
    marginBottom: 12,
    flexDirection: "row",
    alignItems: "center",
    borderWidth: 1,
    borderColor: "#e0e0e0",
  },
  tripCardContent: { flex: 1 },
  tripCardHeader: { flexDirection: "row", alignItems: "center", gap: 8, marginBottom: 4 },
  tripTitle: { fontSize: 16, fontWeight: "600", color: "#333", flex: 1 },
  statusBadge: { paddingHorizontal: 8, paddingVertical: 2, borderRadius: 10 },
  statusText: { fontSize: 11, fontWeight: "600", color: "#fff", textTransform: "capitalize" },
  proBadge: {
    flexDirection: "row",
    alignItems: "center",
    gap: 3,
    backgroundColor: "#BF4028",
    paddingHorizontal: 7,
    paddingVertical: 2,
    borderRadius: 10,
  },
  proBadgeText: { fontSize: 11, fontWeight: "700", color: "#fff" },
  tripDescription: { fontSize: 14, color: "#666", marginBottom: 8 },
  tripMeta: { flexDirection: "row", alignItems: "center", gap: 4 },
  tripMetaText: { fontSize: 12, color: "#999" },
  errorIcon: { marginBottom: 16 },
  errorTitle: { fontSize: 18, fontWeight: "600", color: "#333", marginBottom: 6, textAlign: "center" },
  errorSubtitle: { fontSize: 14, color: "#888", textAlign: "center", marginBottom: 24 },
  retryButton: {
    backgroundColor: "#BF4028",
    borderRadius: 8,
    paddingVertical: 12,
    paddingHorizontal: 28,
  },
  retryButtonText: { color: "#fff", fontSize: 15, fontWeight: "600" },
});
