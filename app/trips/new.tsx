import { View, Text, StyleSheet } from "react-native";

export default function NewTripScreen() {
  return (
    <View style={styles.container}>
      <Text style={styles.text}>Create new trip — coming soon</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: "center", alignItems: "center", backgroundColor: "#f5f5f5" },
  text: { fontSize: 16, color: "#666" },
});
