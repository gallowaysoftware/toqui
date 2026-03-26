import { View, Text, StyleSheet, Pressable } from "react-native";
import { useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react-native";
import { useAuth } from "@/lib/auth";

export default function TripsScreen() {
  const { t } = useTranslation();
  const { accessToken, isLoading } = useAuth();
  const router = useRouter();

  if (isLoading) {
    return (
      <View style={styles.center}>
        <Text style={styles.loading}>{t("common.loading")}</Text>
      </View>
    );
  }

  if (!accessToken) {
    return (
      <View style={styles.center}>
        <Text style={styles.title}>{t("common.appName")}</Text>
        <Text style={styles.subtitle}>{t("common.tagline")}</Text>
        <Pressable
          style={styles.primaryButton}
          onPress={() => {
            // TODO: implement Google OAuth via expo-auth-session
          }}
        >
          <Text style={styles.buttonText}>{t("common.signIn")}</Text>
        </Pressable>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <View style={styles.center}>
        <Text style={styles.emptyText}>{t("trips.empty")}</Text>
        <Pressable
          style={styles.primaryButton}
          onPress={() => router.push("/trips/new" as never)}
        >
          <Plus color="#fff" size={18} />
          <Text style={styles.buttonText}>{t("trips.newTrip")}</Text>
        </Pressable>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#f5f5f5" },
  center: { flex: 1, justifyContent: "center", alignItems: "center", padding: 24 },
  title: { fontSize: 32, fontWeight: "bold", color: "#e8654a", marginBottom: 8 },
  subtitle: { fontSize: 16, color: "#666", textAlign: "center", marginBottom: 32 },
  loading: { fontSize: 16, color: "#666" },
  emptyText: { fontSize: 16, color: "#666", marginBottom: 20, textAlign: "center" },
  primaryButton: {
    backgroundColor: "#e8654a",
    borderRadius: 8,
    paddingVertical: 14,
    paddingHorizontal: 24,
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  buttonText: { color: "#fff", fontSize: 16, fontWeight: "600" },
});
