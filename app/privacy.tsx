import { View, Text, StyleSheet, ScrollView } from "react-native";

export default function PrivacyScreen() {
  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <Text style={styles.title}>Privacy Policy</Text>
      <Text style={styles.text}>Privacy policy content — coming soon</Text>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#fff" },
  content: { padding: 24 },
  title: { fontSize: 24, fontWeight: "bold", marginBottom: 16, color: "#333" },
  text: { fontSize: 16, color: "#666", lineHeight: 24 },
});
