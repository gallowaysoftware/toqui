import { Text, StyleSheet, ScrollView } from "react-native";
import { useTheme } from "@/lib/theme";

export default function PrivacyScreen() {
  const { colors } = useTheme();

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surface },
    content: { padding: 24 },
    title: { fontSize: 24, fontWeight: "bold", marginBottom: 16, color: colors.textPrimary },
    text: { fontSize: 16, color: colors.textSecondary, lineHeight: 24 },
  });

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <Text style={styles.title}>Privacy Policy</Text>
      <Text style={styles.text}>Privacy policy content — coming soon</Text>
    </ScrollView>
  );
}
