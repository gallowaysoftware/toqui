import { useState } from "react";
import { View, Text, StyleSheet, Pressable, ActivityIndicator } from "react-native";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import { MapPin, AlertCircle, CheckCircle, Users } from "lucide-react-native";
import { useTheme } from "@/lib/theme";
import { useAuth } from "@/lib/auth";
import { useAcceptInvite } from "@/lib/hooks/useCollaborators";
import type { AcceptInviteResult } from "@/lib/hooks/useCollaborators";

type InviteState = "idle" | "loading" | "success" | "error";

export default function AcceptInviteScreen() {
  const { token } = useLocalSearchParams<{ token: string }>();
  const { t } = useTranslation();
  const router = useRouter();
  const { colors } = useTheme();
  const { accessToken } = useAuth();
  const { acceptInvite } = useAcceptInvite();

  const [state, setState] = useState<InviteState>("idle");
  const [errorType, setErrorType] = useState<string | null>(null);
  const [tripData, setTripData] = useState<AcceptInviteResult["trip"] | null>(null);

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surfaceSecondary, justifyContent: "center", alignItems: "center", padding: 24 },
    card: {
      backgroundColor: colors.surface,
      borderRadius: 16,
      padding: 32,
      alignItems: "center",
      maxWidth: 400,
      width: "100%",
      borderWidth: 1,
      borderColor: colors.border,
    },
    icon: { marginBottom: 16 },
    title: { fontSize: 22, fontWeight: "700", color: colors.textPrimary, marginBottom: 8, textAlign: "center" },
    subtitle: { fontSize: 15, color: colors.textSecondary, textAlign: "center", marginBottom: 24, lineHeight: 22 },
    tripInfo: {
      backgroundColor: colors.surfaceTertiary,
      borderRadius: 12,
      padding: 16,
      width: "100%",
      marginBottom: 24,
    },
    tripTitle: { fontSize: 16, fontWeight: "600", color: colors.textPrimary, marginBottom: 4 },
    tripDetail: { flexDirection: "row", alignItems: "center", gap: 6, marginTop: 4 },
    tripDetailText: { fontSize: 13, color: colors.textSecondary },
    acceptButton: {
      backgroundColor: colors.accent,
      borderRadius: 8,
      paddingVertical: 14,
      paddingHorizontal: 32,
      alignItems: "center",
      width: "100%",
    },
    acceptButtonText: { color: "#fff", fontSize: 16, fontWeight: "600" },
    disabledButton: { opacity: 0.5 },
    errorText: { fontSize: 14, color: colors.error, textAlign: "center", marginBottom: 16 },
    secondaryButton: {
      marginTop: 12,
      paddingVertical: 10,
      paddingHorizontal: 24,
    },
    secondaryButtonText: { color: colors.accent, fontSize: 15, fontWeight: "500" },
  });

  if (!token) {
    return (
      <View style={styles.container}>
        <View style={styles.card}>
          <AlertCircle color={colors.error} size={40} style={styles.icon as object} />
          <Text style={styles.title}>{t("invite.invalidTitle")}</Text>
          <Text style={styles.subtitle}>{t("invite.invalidSubtitle")}</Text>
          <Pressable style={styles.secondaryButton} onPress={() => router.replace("/(tabs)" as never)}>
            <Text style={styles.secondaryButtonText}>{t("invite.goHome")}</Text>
          </Pressable>
        </View>
      </View>
    );
  }

  if (!accessToken) {
    return (
      <View style={styles.container}>
        <View style={styles.card}>
          <Users color={colors.accent} size={40} style={styles.icon as object} />
          <Text style={styles.title}>{t("invite.title")}</Text>
          <Text style={styles.subtitle}>{t("invite.signInToAccept")}</Text>
          <Pressable style={styles.acceptButton} onPress={() => router.replace("/(tabs)" as never)}>
            <Text style={styles.acceptButtonText}>{t("invite.signIn")}</Text>
          </Pressable>
        </View>
      </View>
    );
  }

  const handleAccept = async () => {
    setState("loading");
    setErrorType(null);
    try {
      const result = await acceptInvite(token);
      setTripData(result.trip);
      setState("success");
    } catch (err) {
      const message = err instanceof Error ? err.message : "unknown";
      setErrorType(message);
      setState("error");
    }
  };

  const handleGoToTrip = () => {
    if (tripData) {
      router.replace(`/trips/${tripData.id}` as never);
    }
  };

  if (state === "success" && tripData) {
    return (
      <View style={styles.container}>
        <View style={styles.card}>
          <CheckCircle color={colors.success} size={40} style={styles.icon as object} />
          <Text style={styles.title}>{t("invite.successTitle")}</Text>
          <Text style={styles.subtitle}>{t("invite.successSubtitle", { tripTitle: tripData.title })}</Text>
          <Pressable style={styles.acceptButton} onPress={handleGoToTrip}>
            <Text style={styles.acceptButtonText}>{t("invite.viewTrip")}</Text>
          </Pressable>
        </View>
      </View>
    );
  }

  const getErrorMessage = () => {
    switch (errorType) {
      case "expired": return t("invite.errorExpired");
      case "already_accepted": return t("invite.errorAlreadyAccepted");
      default: return t("invite.errorGeneric");
    }
  };

  return (
    <View style={styles.container}>
      <View style={styles.card}>
        <Users color={colors.accent} size={40} style={styles.icon as object} />
        <Text style={styles.title}>{t("invite.title")}</Text>
        <Text style={styles.subtitle}>{t("invite.subtitle")}</Text>

        {state === "error" && (
          <Text style={styles.errorText}>{getErrorMessage()}</Text>
        )}

        <Pressable
          style={[styles.acceptButton, state === "loading" && styles.disabledButton]}
          onPress={handleAccept}
          disabled={state === "loading"}
          accessibilityLabel={t("invite.acceptButton")}
          accessibilityRole="button"
        >
          {state === "loading" ? (
            <ActivityIndicator color="#fff" size="small" />
          ) : (
            <Text style={styles.acceptButtonText}>{t("invite.acceptButton")}</Text>
          )}
        </Pressable>

        <Pressable style={styles.secondaryButton} onPress={() => router.replace("/(tabs)" as never)}>
          <Text style={styles.secondaryButtonText}>{t("invite.goHome")}</Text>
        </Pressable>
      </View>
    </View>
  );
}
