import { View, Text, StyleSheet, Pressable, FlatList, ActivityIndicator } from "react-native";
import { useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import { Plus, MapPin, ChevronRight } from "lucide-react-native";
import { useAuth } from "@/lib/auth";
import { useGoogleAuth } from "@/lib/google-auth";
import { useTrips } from "@/lib/hooks/useTrips";
import { TripStatus } from "@gen/toqui/v1/trip_pb";
import type { Trip } from "@gen/toqui/v1/trip_pb";

function TripCard({ trip, onPress }: { trip: Trip; onPress: () => void }) {
  const statusConfig: Record<number, { label: string; color: string }> = {
    [TripStatus.PLANNING]: { label: "planning", color: "#3b82f6" },
    [TripStatus.ACTIVE]: { label: "active", color: "#22c55e" },
    [TripStatus.COMPLETED]: { label: "completed", color: "#9ca3af" },
  };
  const { label: statusLabel, color: statusColor } = statusConfig[trip.status] ?? { label: "unknown", color: "#9ca3af" };

  return (
    <Pressable style={styles.tripCard} onPress={onPress}>
      <View style={styles.tripCardContent}>
        <View style={styles.tripCardHeader}>
          <Text style={styles.tripTitle} numberOfLines={1}>{trip.title}</Text>
          <View style={[styles.statusBadge, { backgroundColor: statusColor }]}>
            <Text style={styles.statusText}>{statusLabel}</Text>
          </View>
        </View>
        {trip.description ? (
          <Text style={styles.tripDescription} numberOfLines={2}>{trip.description}</Text>
        ) : null}
        <View style={styles.tripMeta}>
          <MapPin color="#999" size={14} />
          <Text style={styles.tripMetaText}>{trip.destinationCountry || "No destination"}</Text>
        </View>
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
  const { trips, isLoading: tripsLoading } = useTrips();

  if (authLoading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color="#e8654a" />
      </View>
    );
  }

  if (!accessToken) {
    return (
      <View style={styles.center}>
        <Text style={styles.title}>{t("common.appName")}</Text>
        <Text style={styles.subtitle}>{t("common.tagline")}</Text>
        <Pressable
          style={[styles.primaryButton, !authReady && styles.disabledButton]}
          onPress={signIn}
          disabled={!authReady}
        >
          <Text style={styles.buttonText}>{t("common.signIn")}</Text>
        </Pressable>
      </View>
    );
  }

  if (tripsLoading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color="#e8654a" />
      </View>
    );
  }

  if (!trips || trips.length === 0) {
    return (
      <View style={styles.container}>
        <View style={styles.center}>
          <Text style={styles.emptyText}>{t("trips.empty")}</Text>
          <Pressable
            style={styles.primaryButton}
            onPress={() => router.push("/trips/new" as never)}
          >
            <Plus color="#fff" size={18} />
            <Text style={styles.buttonText}>{t("trips.newTrip")}</Text>
          </Pressable>
        </View>
      </View>
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
            <Plus color="#e8654a" size={18} />
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
  title: { fontSize: 32, fontWeight: "bold", color: "#e8654a", marginBottom: 8 },
  subtitle: { fontSize: 16, color: "#666", textAlign: "center", marginBottom: 32 },
  emptyText: { fontSize: 16, color: "#666", marginBottom: 20, textAlign: "center" },
  primaryButton: {
    backgroundColor: "#e8654a",
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
    borderColor: "#e8654a",
    borderStyle: "dashed",
    marginBottom: 12,
  },
  newTripText: { color: "#e8654a", fontSize: 16, fontWeight: "600" },
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
  tripDescription: { fontSize: 14, color: "#666", marginBottom: 8 },
  tripMeta: { flexDirection: "row", alignItems: "center", gap: 4 },
  tripMetaText: { fontSize: 12, color: "#999" },
});
