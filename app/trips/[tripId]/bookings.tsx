import { View, Text, StyleSheet, Pressable, FlatList, ActivityIndicator, TextInput, Alert } from "react-native";
import { useState, useCallback } from "react";
import { useLocalSearchParams } from "expo-router";
import { Plus, Trash2, Plane, Hotel, Car, Train, Ticket, Utensils, MoreHorizontal } from "lucide-react-native";
import { useBookings, useIngestBooking, useDeleteBooking } from "@/lib/hooks/useBookings";
import { BookingType } from "@gen/toqui/v1/booking_pb";
import type { Booking } from "@gen/toqui/v1/booking_pb";

const typeConfig: Record<number, { label: string; color: string; Icon: typeof Plane }> = {
  [BookingType.FLIGHT]: { label: "Flight", color: "#3b82f6", Icon: Plane },
  [BookingType.HOTEL]: { label: "Hotel", color: "#8b5cf6", Icon: Hotel },
  [BookingType.CAR_RENTAL]: { label: "Car Rental", color: "#f59e0b", Icon: Car },
  [BookingType.TRAIN]: { label: "Train", color: "#10b981", Icon: Train },
  [BookingType.ACTIVITY]: { label: "Activity", color: "#ec4899", Icon: Ticket },
  [BookingType.RESTAURANT]: { label: "Restaurant", color: "#ef4444", Icon: Utensils },
  [BookingType.TOUR]: { label: "Tour", color: "#06b6d4", Icon: Ticket },
  [BookingType.OTHER]: { label: "Other", color: "#6b7280", Icon: MoreHorizontal },
};

function BookingCard({ booking, onDelete }: { booking: Booking; onDelete: () => void }) {
  const config = typeConfig[booking.type] ?? typeConfig[BookingType.OTHER]!;
  const { Icon } = config;

  return (
    <View style={styles.card}>
      <View style={[styles.typeIndicator, { backgroundColor: config.color }]}>
        <Icon color="#fff" size={16} />
      </View>
      <View style={styles.cardContent}>
        <Text style={styles.cardTitle} numberOfLines={1}>{booking.title || "Untitled booking"}</Text>
        <Text style={styles.cardType}>{config.label}</Text>
        {booking.provider ? <Text style={styles.cardMeta}>{booking.provider}</Text> : null}
        {booking.confirmationCode ? <Text style={styles.cardMeta}>#{booking.confirmationCode}</Text> : null}
      </View>
      <Pressable onPress={onDelete} hitSlop={8}>
        <Trash2 color="#999" size={18} />
      </Pressable>
    </View>
  );
}

export default function BookingsScreen() {
  const { tripId } = useLocalSearchParams<{ tripId: string }>();
  const { bookings, isLoading } = useBookings(tripId!);
  const ingestBooking = useIngestBooking();
  const deleteBooking = useDeleteBooking();
  const [showAdd, setShowAdd] = useState(false);
  const [rawText, setRawText] = useState("");

  const handleIngest = useCallback(async () => {
    if (!rawText.trim()) return;
    await ingestBooking.mutateAsync({
      tripId: tripId!,
      type: BookingType.UNSPECIFIED,
      rawText: rawText.trim(),
    });
    setRawText("");
    setShowAdd(false);
  }, [rawText, tripId, ingestBooking]);

  const handleDelete = useCallback((id: string) => {
    Alert.alert("Delete Booking", "Are you sure?", [
      { text: "Cancel", style: "cancel" },
      { text: "Delete", style: "destructive", onPress: () => deleteBooking.mutate({ id, tripId: tripId! }) },
    ]);
  }, [tripId, deleteBooking]);

  if (isLoading) {
    return <View style={styles.center}><ActivityIndicator size="large" color="#e8654a" /></View>;
  }

  return (
    <View style={styles.container}>
      <FlatList
        data={bookings}
        keyExtractor={(item) => item.id}
        renderItem={({ item }) => (
          <BookingCard booking={item} onDelete={() => handleDelete(item.id)} />
        )}
        contentContainerStyle={styles.list}
        ListEmptyComponent={
          <View style={styles.empty}>
            <Text style={styles.emptyText}>No bookings yet</Text>
            <Text style={styles.emptySubtext}>Paste a confirmation email to add one</Text>
          </View>
        }
        ListHeaderComponent={
          showAdd ? (
            <View style={styles.addForm}>
              <TextInput
                style={styles.textArea}
                placeholder="Paste booking confirmation text or email..."
                placeholderTextColor="#999"
                value={rawText}
                onChangeText={setRawText}
                multiline
                numberOfLines={5}
                autoFocus
              />
              <View style={styles.addActions}>
                <Pressable style={styles.cancelButton} onPress={() => { setShowAdd(false); setRawText(""); }}>
                  <Text style={styles.cancelText}>Cancel</Text>
                </Pressable>
                <Pressable
                  style={[styles.submitButton, (!rawText.trim() || ingestBooking.isPending) && styles.disabledButton]}
                  onPress={handleIngest}
                  disabled={!rawText.trim() || ingestBooking.isPending}
                >
                  {ingestBooking.isPending ? (
                    <ActivityIndicator color="#fff" size="small" />
                  ) : (
                    <Text style={styles.submitText}>Add Booking</Text>
                  )}
                </Pressable>
              </View>
            </View>
          ) : (
            <Pressable style={styles.addButton} onPress={() => setShowAdd(true)}>
              <Plus color="#e8654a" size={18} />
              <Text style={styles.addButtonText}>Add Booking</Text>
            </Pressable>
          )
        }
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#f5f5f5" },
  center: { flex: 1, justifyContent: "center", alignItems: "center" },
  list: { padding: 16 },
  empty: { alignItems: "center", paddingTop: 40 },
  emptyText: { fontSize: 16, fontWeight: "600", color: "#666" },
  emptySubtext: { fontSize: 14, color: "#999", marginTop: 4 },
  card: {
    flexDirection: "row",
    alignItems: "center",
    backgroundColor: "#fff",
    borderRadius: 12,
    padding: 14,
    marginBottom: 10,
    borderWidth: 1,
    borderColor: "#e0e0e0",
  },
  typeIndicator: {
    width: 36,
    height: 36,
    borderRadius: 18,
    justifyContent: "center",
    alignItems: "center",
    marginRight: 12,
  },
  cardContent: { flex: 1 },
  cardTitle: { fontSize: 15, fontWeight: "600", color: "#333" },
  cardType: { fontSize: 12, color: "#666", marginTop: 2 },
  cardMeta: { fontSize: 12, color: "#999", marginTop: 1 },
  addButton: {
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
  addButtonText: { color: "#e8654a", fontSize: 16, fontWeight: "600" },
  addForm: {
    backgroundColor: "#fff",
    borderRadius: 12,
    padding: 14,
    marginBottom: 12,
    borderWidth: 1,
    borderColor: "#e0e0e0",
  },
  textArea: {
    borderWidth: 1,
    borderColor: "#ddd",
    borderRadius: 8,
    padding: 12,
    fontSize: 14,
    minHeight: 100,
    textAlignVertical: "top",
    color: "#333",
  },
  addActions: { flexDirection: "row", justifyContent: "flex-end", gap: 10, marginTop: 12 },
  cancelButton: { padding: 10 },
  cancelText: { color: "#666", fontWeight: "500" },
  submitButton: { backgroundColor: "#e8654a", borderRadius: 8, paddingVertical: 10, paddingHorizontal: 20 },
  disabledButton: { opacity: 0.5 },
  submitText: { color: "#fff", fontWeight: "600" },
});
