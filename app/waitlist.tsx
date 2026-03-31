import { View, Text, TextInput, StyleSheet, Pressable, ActivityIndicator } from "react-native";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useTheme } from "@/lib/theme";
import { useJoinWaitlist, useWaitlistStatus } from "@/lib/hooks/useWaitlist";

export default function WaitlistScreen() {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const [email, setEmail] = useState("");
  const [joinedEmail, setJoinedEmail] = useState<string | null>(null);
  const joinWaitlist = useJoinWaitlist();
  const { data: status } = useWaitlistStatus(joinedEmail);

  const handleJoin = async () => {
    if (!email.trim()) return;
    await joinWaitlist.mutateAsync({ email: email.trim() });
    setJoinedEmail(email.trim());
  };

  const styles = StyleSheet.create({
    container: { flex: 1, justifyContent: "center", padding: 24, backgroundColor: colors.surface },
    title: { fontSize: 24, fontWeight: "bold", textAlign: "center", color: colors.textPrimary, marginBottom: 12 },
    subtitle: { fontSize: 15, color: colors.textSecondary, textAlign: "center", lineHeight: 22, marginBottom: 32 },
    input: {
      borderWidth: 1,
      borderColor: colors.border,
      borderRadius: 8,
      padding: 14,
      fontSize: 15,
      marginBottom: 12,
      color: colors.textPrimary,
    },
    joinButton: {
      backgroundColor: colors.accent,
      borderRadius: 8,
      padding: 14,
      alignItems: "center",
    },
    disabledButton: { opacity: 0.5 },
    joinText: { color: "#fff", fontSize: 16, fontWeight: "600" },
    errorText: { color: colors.error, fontSize: 14, textAlign: "center", marginTop: 12 },
    positionCard: {
      backgroundColor: colors.surfaceSecondary,
      borderRadius: 12,
      padding: 24,
      alignItems: "center",
      marginBottom: 24,
    },
    positionLabel: { fontSize: 14, color: colors.textSecondary, marginBottom: 8 },
    positionNumber: { fontSize: 48, fontWeight: "bold", color: colors.accent },
    note: { fontSize: 14, color: colors.textTertiary, textAlign: "center", lineHeight: 20 },
  });

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
        placeholderTextColor={colors.textTertiary}
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
