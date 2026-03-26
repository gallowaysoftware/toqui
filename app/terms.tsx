import { View, Text, StyleSheet, ScrollView } from "react-native";

export default function TermsScreen() {
  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <Text style={styles.title}>Terms of Service</Text>
      <Text style={styles.text}>Terms of service content — coming soon</Text>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#fff" },
  content: { padding: 24 },
  title: { fontSize: 24, fontWeight: "bold", marginBottom: 16, color: "#333" },
  text: { fontSize: 16, color: "#666", lineHeight: 24 },
});
