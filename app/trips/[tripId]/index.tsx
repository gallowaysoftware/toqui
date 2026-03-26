import { View, Text, StyleSheet, Pressable, ScrollView, ActivityIndicator } from "react-native";
import { useLocalSearchParams, useRouter, Stack } from "expo-router";
import { MessageCircle, Calendar, Settings, Play, CheckCircle } from "lucide-react-native";
import { useTrip, useUpdateTrip } from "@/lib/hooks/useTrips";
import { useItinerary } from "@/lib/hooks/useItinerary";
import { ItineraryTimeline } from "@/components/itinerary/ItineraryTimeline";
import { TripStatus } from "@gen/toqui/v1/trip_pb";

export default function TripDetailScreen() {
  const { tripId } = useLocalSearchParams<{ tripId: string }>();
  const { trip, isLoading } = useTrip(tripId!);
  const { itinerary } = useItinerary(tripId!);
  const updateTrip = useUpdateTrip();
  const router = useRouter();

  if (isLoading || !trip) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color="#e8654a" />
      </View>
    );
  }

  const isPlannable = trip.status === TripStatus.PLANNING;
  const isActive = trip.status === TripStatus.ACTIVE;

  return (
    <>
      <Stack.Screen options={{ title: trip.title }} />
      <ScrollView style={styles.container} contentContainerStyle={styles.content}>
        {trip.description ? (
          <Text style={styles.description}>{trip.description}</Text>
        ) : null}

        {(trip.startDate || trip.endDate) && (
          <Text style={styles.dates}>
            {trip.startDate}{trip.startDate && trip.endDate ? " → " : ""}{trip.endDate}
          </Text>
        )}

        <View style={styles.actions}>
          <Pressable
            style={styles.actionButton}
            onPress={() => router.push(`/trips/${tripId}/chat` as never)}
          >
            <MessageCircle color="#e8654a" size={24} />
            <Text style={styles.actionText}>Chat</Text>
          </Pressable>

          <Pressable
            style={styles.actionButton}
            onPress={() => router.push(`/trips/${tripId}/bookings` as never)}
          >
            <Calendar color="#e8654a" size={24} />
            <Text style={styles.actionText}>Bookings</Text>
          </Pressable>

          <Pressable
            style={styles.actionButton}
            onPress={() => router.push(`/trips/${tripId}/settings` as never)}
          >
            <Settings color="#e8654a" size={24} />
            <Text style={styles.actionText}>Settings</Text>
          </Pressable>
        </View>

        {itinerary && <ItineraryTimeline itinerary={itinerary} />}

        {isPlannable && (
          <Pressable
            style={styles.statusButton}
            onPress={() => updateTrip.mutate({ id: tripId!, status: TripStatus.ACTIVE })}
            disabled={updateTrip.isPending}
          >
            <Play color="#22c55e" size={18} />
            <Text style={styles.statusButtonText}>Start Trip</Text>
          </Pressable>
        )}

        {isActive && (
          <Pressable
            style={styles.statusButton}
            onPress={() => updateTrip.mutate({ id: tripId!, status: TripStatus.COMPLETED })}
            disabled={updateTrip.isPending}
          >
            <CheckCircle color="#3b82f6" size={18} />
            <Text style={styles.statusButtonText}>Complete Trip</Text>
          </Pressable>
        )}
      </ScrollView>
    </>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#f5f5f5" },
  content: { padding: 16 },
  center: { flex: 1, justifyContent: "center", alignItems: "center" },
  description: { fontSize: 15, color: "#666", lineHeight: 22, marginBottom: 16 },
  dates: { fontSize: 14, color: "#999", marginBottom: 20 },
  actions: { flexDirection: "row", gap: 12, marginBottom: 24 },
  actionButton: {
    flex: 1,
    backgroundColor: "#fff",
    borderRadius: 12,
    padding: 16,
    alignItems: "center",
    gap: 8,
    borderWidth: 1,
    borderColor: "#e0e0e0",
  },
  actionText: { fontSize: 14, fontWeight: "500", color: "#333" },
  statusButton: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: 8,
    padding: 14,
    backgroundColor: "#fff",
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "#e0e0e0",
  },
  statusButtonText: { fontSize: 16, fontWeight: "600", color: "#333" },
});
