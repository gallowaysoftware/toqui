import { useState, useCallback } from "react";
import {
  View,
  Text,
  TextInput,
  Pressable,
  StyleSheet,
  ActivityIndicator,
  KeyboardAvoidingView,
  Platform,
  Linking,
  ScrollView,
} from "react-native";
import { useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import { Plane, ChevronRight } from "lucide-react-native";
import { useTheme } from "@/lib/theme";
import { useOnboarding } from "@/lib/hooks/useOnboarding";
import { useCreateTrip } from "@/lib/hooks/useTrips";
import { useAnalytics } from "@/lib/analytics";

const SAMPLE_PERSONAS = [
  {
    id: "tokyo-food",
    initial: "T",
    accentColor: "#e74c3c",
  },
  {
    id: "paris-architecture",
    initial: "P",
    accentColor: "#2563eb",
  },
  {
    id: "bali-adventure",
    initial: "B",
    accentColor: "#16a34a",
  },
  {
    id: "rome-history",
    initial: "R",
    accentColor: "#d97706",
  },
] as const;

function formatDate(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

export default function OnboardingScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { colors } = useTheme();
  const { completeOnboarding } = useOnboarding();
  const createTrip = useCreateTrip();
  const { track } = useAnalytics();
  const [destination, setDestination] = useState("");
  const [termsAccepted, setTermsAccepted] = useState(false);

  const handleStartPlanning = useCallback(async () => {
    if (!destination.trim()) return;

    await completeOnboarding();
    track("onboarding_completed", { cta: "start_planning" });

    try {
      const startDate = new Date();
      startDate.setDate(startDate.getDate() + 14);
      const endDate = new Date(startDate);
      endDate.setDate(endDate.getDate() + 6);

      const trip = await createTrip.mutateAsync({
        title: destination.trim(),
        startDate: formatDate(startDate),
        endDate: formatDate(endDate),
      });

      if (trip) {
        router.replace(`/trips/${trip.id}/chat` as never);
      }
    } catch {
      // Fall back to new trip form on error
      router.replace({
        pathname: "/trips/new" as never,
        params: { destination: destination.trim() },
      });
    }
  }, [completeOnboarding, createTrip, destination, router, track]);

  const handleBrowseIdeas = useCallback(async () => {
    await completeOnboarding();
    track("onboarding_completed", { cta: "explore_first" });
    router.replace("/(tabs)" as never);
  }, [completeOnboarding, router, track]);

  const styles = StyleSheet.create({
    container: {
      flex: 1,
      backgroundColor: colors.surfaceSecondary,
    },
    content: {
      flex: 1,
      justifyContent: "center",
      alignItems: "center",
      paddingHorizontal: 32,
    },
    iconContainer: {
      width: 100,
      height: 100,
      borderRadius: 50,
      backgroundColor: colors.accentSoft,
      justifyContent: "center",
      alignItems: "center",
      marginBottom: 24,
    },
    headline: {
      fontSize: 28,
      fontWeight: "bold",
      color: colors.textPrimary,
      textAlign: "center",
      marginBottom: 8,
    },
    valueProp: {
      fontSize: 16,
      color: colors.textSecondary,
      textAlign: "center",
      lineHeight: 24,
      maxWidth: 320,
      marginBottom: 32,
    },
    input: {
      backgroundColor: colors.inputBg,
      borderWidth: 1,
      borderColor: colors.inputBorder,
      borderRadius: 12,
      padding: 16,
      fontSize: 17,
      color: colors.textPrimary,
      width: "100%",
      maxWidth: 360,
      marginBottom: 16,
      textAlign: "center",
    },
    primaryButton: {
      backgroundColor: colors.accent,
      borderRadius: 12,
      paddingVertical: 16,
      paddingHorizontal: 32,
      width: "100%",
      maxWidth: 360,
      alignItems: "center",
      marginBottom: 16,
    },
    primaryButtonDisabled: {
      opacity: 0.5,
    },
    primaryButtonText: {
      color: "#fff",
      fontSize: 17,
      fontWeight: "600",
    },
    secondaryButton: {
      paddingVertical: 12,
      paddingHorizontal: 32,
      alignItems: "center",
    },
    secondaryButtonText: {
      color: colors.textSecondary,
      fontSize: 15,
      fontWeight: "500",
    },
    termsRow: {
      flexDirection: "row",
      alignItems: "flex-start",
      gap: 10,
      width: "100%",
      maxWidth: 360,
      marginBottom: 16,
    },
    checkbox: {
      width: 20,
      height: 20,
      borderRadius: 4,
      borderWidth: 2,
      borderColor: colors.accent,
      alignItems: "center",
      justifyContent: "center",
      marginTop: 1,
      flexShrink: 0,
    },
    checkboxChecked: {
      backgroundColor: colors.accent,
    },
    checkboxMark: {
      color: "#fff",
      fontSize: 13,
      fontWeight: "700",
      lineHeight: 14,
    },
    termsText: {
      flex: 1,
      fontSize: 13,
      color: colors.textSecondary,
      lineHeight: 19,
    },
    termsLink: {
      color: colors.accent,
      textDecorationLine: "underline",
    },
    expertsSection: {
      width: "100%",
      maxWidth: 360,
      marginBottom: 28,
    },
    expertsSectionTitle: {
      fontSize: 15,
      fontWeight: "700",
      color: colors.textPrimary,
      marginBottom: 6,
    },
    expertsSectionSubtitle: {
      fontSize: 13,
      color: colors.textSecondary,
      lineHeight: 19,
      marginBottom: 14,
    },
    personaRow: {
      flexDirection: "row",
      gap: 10,
    },
    personaCard: {
      flex: 1,
      backgroundColor: colors.surface,
      borderRadius: 12,
      padding: 10,
      alignItems: "center",
      borderWidth: 1,
      borderColor: colors.border,
    },
    personaAvatar: {
      width: 40,
      height: 40,
      borderRadius: 20,
      justifyContent: "center",
      alignItems: "center",
      marginBottom: 6,
    },
    personaAvatarText: {
      color: "#ffffff",
      fontSize: 18,
      fontWeight: "700",
    },
    personaName: {
      fontSize: 11,
      fontWeight: "600",
      color: colors.textPrimary,
      textAlign: "center",
      lineHeight: 14,
    },
  });

  const isDisabled = !destination.trim() || !termsAccepted || createTrip.isPending;

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : undefined}
    >
      <View style={styles.content}>
        <View style={styles.iconContainer}>
          <Plane color={colors.accent} size={48} />
        </View>
        <Text style={styles.headline} testID="onboarding-headline">
          {t("onboarding.welcome.headline")}
        </Text>
        <Text style={styles.valueProp}>
          {t("onboarding.welcome.valueProp")}
        </Text>

        <View style={styles.expertsSection} testID="onboarding-experts-section">
          <Text style={styles.expertsSectionTitle}>
            {t("onboarding.welcome.meetExperts")}
          </Text>
          <Text style={styles.expertsSectionSubtitle}>
            {t("onboarding.welcome.meetExpertsSubtitle")}
          </Text>
          <ScrollView horizontal showsHorizontalScrollIndicator={false}>
            <View style={styles.personaRow}>
              {SAMPLE_PERSONAS.map((persona) => (
                <View key={persona.id} style={styles.personaCard} testID={`persona-card-${persona.id}`}>
                  <View style={[styles.personaAvatar, { backgroundColor: persona.accentColor }]}>
                    <Text style={styles.personaAvatarText}>{persona.initial}</Text>
                  </View>
                  <Text style={styles.personaName} numberOfLines={2}>
                    {t(`onboarding.welcome.personas.${persona.id}`)}
                  </Text>
                </View>
              ))}
            </View>
          </ScrollView>
        </View>

        <TextInput
          style={styles.input}
          placeholder={t("onboarding.welcome.destinationPlaceholder")}
          placeholderTextColor={colors.textTertiary}
          value={destination}
          onChangeText={setDestination}
          autoFocus
          returnKeyType="go"
          onSubmitEditing={handleStartPlanning}
          accessibilityLabel="Trip destination"
          testID="onboarding-destination-input"
        />

        <Pressable
          style={styles.termsRow}
          onPress={() => setTermsAccepted((v) => !v)}
          accessibilityRole="checkbox"
          accessibilityState={{ checked: termsAccepted }}
          testID="onboarding-terms-checkbox"
        >
          <View style={[styles.checkbox, termsAccepted && styles.checkboxChecked]}>
            {termsAccepted ? <Text style={styles.checkboxMark}>✓</Text> : null}
          </View>
          <Text style={styles.termsText}>
            {"I agree to the "}
            <Text
              style={styles.termsLink}
              onPress={() => Linking.openURL("https://toqui.travel/privacy")}
            >
              Privacy Policy
            </Text>
            {" and "}
            <Text
              style={styles.termsLink}
              onPress={() => Linking.openURL("https://toqui.travel/terms")}
            >
              Terms of Service
            </Text>
          </Text>
        </Pressable>

        <Pressable
          style={[styles.primaryButton, isDisabled && styles.primaryButtonDisabled]}
          onPress={handleStartPlanning}
          disabled={isDisabled}
          accessibilityRole="button"
          accessibilityLabel={t("onboarding.welcome.startPlanning")}
          testID="onboarding-start-planning"
        >
          {createTrip.isPending ? (
            <ActivityIndicator color="#fff" size="small" />
          ) : (
            <Text style={styles.primaryButtonText}>
              {t("onboarding.welcome.startPlanning")}
            </Text>
          )}
        </Pressable>

        <Pressable
          style={[styles.secondaryButton, !termsAccepted && { opacity: 0.4 }]}
          onPress={termsAccepted ? handleBrowseIdeas : undefined}
          accessibilityRole="button"
          accessibilityLabel={t("onboarding.welcome.browseIdeas")}
          testID="onboarding-browse-ideas"
        >
          <Text style={styles.secondaryButtonText}>
            {t("onboarding.welcome.browseIdeas")}
          </Text>
        </Pressable>
      </View>
    </KeyboardAvoidingView>
  );
}
