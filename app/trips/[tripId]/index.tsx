import { View, Text, StyleSheet, Pressable } from "react-native";
import { useLocalSearchParams, useRouter } from "expo-router";
import { MessageCircle, Calendar, Settings } from "lucide-react-native";

export default function TripDetailScreen() {
  const { tripId } = useLocalSearchParams<{ tripId: string }>();
  const router = useRouter();

  return (
    <View style={styles.container}>
      <Text style={styles.title}>Trip Detail</Text>
      <Text style={styles.subtitle}>ID: {tripId}</Text>

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
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, padding: 16, backgroundColor: "#f5f5f5" },
  title: { fontSize: 24, fontWeight: "bold", color: "#333", marginBottom: 4 },
  subtitle: { fontSize: 14, color: "#666", marginBottom: 24 },
  actions: { flexDirection: "row", gap: 12 },
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
});
