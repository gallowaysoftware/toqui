import { View, Text, StyleSheet } from "react-native";
import { useLocalSearchParams } from "expo-router";

export default function TripSettingsScreen() {
  const { tripId } = useLocalSearchParams<{ tripId: string }>();

  return (
    <View style={styles.container}>
      <Text style={styles.text}>Settings for trip {tripId} — coming soon</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: "center", alignItems: "center", backgroundColor: "#f5f5f5" },
  text: { fontSize: 16, color: "#666", textAlign: "center", padding: 24 },
});
