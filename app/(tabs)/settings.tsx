import { View, Text, StyleSheet, Pressable, ScrollView } from "react-native";
import { useTranslation } from "react-i18next";
import { useAuth } from "@/lib/auth";

export default function SettingsScreen() {
  const { t } = useTranslation();
  const { logout, accessToken } = useAuth();

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>{t("settings.account")}</Text>
        {accessToken ? (
          <Pressable style={styles.dangerButton} onPress={logout}>
            <Text style={styles.dangerButtonText}>{t("common.signOut")}</Text>
          </Pressable>
        ) : (
          <Text style={styles.text}>Not signed in</Text>
        )}
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#f5f5f5" },
  content: { padding: 16 },
  section: {
    backgroundColor: "#fff",
    borderRadius: 12,
    padding: 16,
    marginBottom: 16,
  },
  sectionTitle: { fontSize: 18, fontWeight: "600", marginBottom: 12, color: "#333" },
  text: { fontSize: 14, color: "#666" },
  dangerButton: {
    borderWidth: 1,
    borderColor: "#ef4444",
    borderRadius: 8,
    padding: 12,
    alignItems: "center",
  },
  dangerButtonText: { color: "#ef4444", fontWeight: "600" },
});
