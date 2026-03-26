import { View, Text, StyleSheet } from "react-native";
import { useTranslation } from "react-i18next";

export default function WaitlistScreen() {
  const { t } = useTranslation();

  return (
    <View style={styles.container}>
      <Text style={styles.title}>{t("waitlist.title")}</Text>
      <Text style={styles.text}>{t("waitlist.description")}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: "center", alignItems: "center", padding: 24, backgroundColor: "#fff" },
  title: { fontSize: 24, fontWeight: "bold", marginBottom: 12, color: "#333", textAlign: "center" },
  text: { fontSize: 16, color: "#666", textAlign: "center", lineHeight: 24 },
});
