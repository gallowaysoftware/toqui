import { View, Text, TextInput, StyleSheet, Pressable, ActivityIndicator } from "react-native";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useJoinWaitlist, useWaitlistStatus } from "@/lib/hooks/useWaitlist";

export default function WaitlistScreen() {
  const { t } = useTranslation();
  const [email, setEmail] = useState("");
  const [joinedEmail, setJoinedEmail] = useState<string | null>(null);
  const joinWaitlist = useJoinWaitlist();
  const { data: status } = useWaitlistStatus(joinedEmail);

  const handleJoin = async () => {
    if (!email.trim()) return;
    await joinWaitlist.mutateAsync({ email: email.trim() });
    setJoinedEmail(email.trim());
  };

  if (joinedEmail && status) {
    return (
      <View style={styles.container}>
        <Text style={styles.title}>{t("waitlist.joinedTitle")}</Text>
        <Text style={styles.subtitle}>{t("waitlist.joinedDescription")}</Text>
        <View style={styles.positionCard}>
          <Text style={styles.positionLabel}>{t("waitlist.positionLabel")}</Text>
          <Text style={styles.positionNumber}>#{status.position}</Text>
        </View>
        <Text style={styles.note}>{t("waitlist.notifyMessage")}</Text>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <Text style={styles.title}>{t("waitlist.title")}</Text>
      <Text style={styles.subtitle}>{t("waitlist.description")}</Text>

      <TextInput
        style={styles.input}
        placeholder={t("waitlist.emailPlaceholder")}
        placeholderTextColor="#999"
        value={email}
        onChangeText={setEmail}
        keyboardType="email-address"
        autoCapitalize="none"
        autoCorrect={false}
      />

      <Pressable
        style={[styles.joinButton, (!email.trim() || joinWaitlist.isPending) && styles.disabledButton]}
        onPress={handleJoin}
        disabled={!email.trim() || joinWaitlist.isPending}
      >
        {joinWaitlist.isPending ? (
          <ActivityIndicator color="#fff" size="small" />
        ) : (
          <Text style={styles.joinText}>{t("waitlist.joinButton")}</Text>
        )}
      </Pressable>

      {joinWaitlist.isError && (
        <Text style={styles.errorText}>
          {joinWaitlist.error.message || "Failed to join waitlist"}
        </Text>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: "center", padding: 24, backgroundColor: "#fff" },
  title: { fontSize: 24, fontWeight: "bold", textAlign: "center", color: "#333", marginBottom: 12 },
  subtitle: { fontSize: 15, color: "#666", textAlign: "center", lineHeight: 22, marginBottom: 32 },
  input: {
    borderWidth: 1,
    borderColor: "#ddd",
    borderRadius: 8,
    padding: 14,
    fontSize: 15,
    marginBottom: 12,
    color: "#333",
  },
  joinButton: {
    backgroundColor: "#e8654a",
    borderRadius: 8,
    padding: 14,
    alignItems: "center",
  },
  disabledButton: { opacity: 0.5 },
  joinText: { color: "#fff", fontSize: 16, fontWeight: "600" },
  errorText: { color: "#ef4444", fontSize: 14, textAlign: "center", marginTop: 12 },
  positionCard: {
    backgroundColor: "#f5f5f5",
    borderRadius: 12,
    padding: 24,
    alignItems: "center",
    marginBottom: 24,
  },
  positionLabel: { fontSize: 14, color: "#666", marginBottom: 8 },
  positionNumber: { fontSize: 48, fontWeight: "bold", color: "#e8654a" },
  note: { fontSize: 14, color: "#999", textAlign: "center", lineHeight: 20 },
});
