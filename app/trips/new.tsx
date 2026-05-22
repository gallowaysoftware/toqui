import { View, Text, TextInput, Pressable, StyleSheet, ScrollView, ActivityIndicator } from "react-native";
import { useState, useMemo, useEffect } from "react";
import { useRouter, useLocalSearchParams } from "expo-router";
import { useTranslation } from "react-i18next";
import { AlertCircle, Sparkles, ChevronDown, ChevronUp } from "lucide-react-native";
import { useCreateTrip } from "@/lib/hooks/useTrips";
import { DatePicker } from "@/components/DatePicker";
import { useTheme } from "@/lib/theme";
import { getTemplateById } from "@/lib/data/tripTemplates";

function formatDate(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

export default function NewTripScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { colors } = useTheme();
  const { destination, template: templateId } = useLocalSearchParams<{
    destination?: string;
    template?: string;
  }>();
  const createTrip = useCreateTrip();

  const template = useMemo(
    () => (templateId ? getTemplateById(templateId) : undefined),
    [templateId],
  );

  // When coming from a template, auto-create and go straight to chat
  useEffect(() => {
    if (template) {
      const end = new Date();
      end.setDate(end.getDate() + template.duration - 1);
      void createTrip
        .mutateAsync({
          title: t(template.titleKey),
          description: t(template.descriptionKey),
          startDate: formatDate(new Date()),
          endDate: formatDate(end),
        })
        .then((trip) => {
          if (trip) {
            router.replace({
              pathname: `/trips/${trip.id}/chat` as never,
              params: { suggestedPrompt: t(template.suggestedPromptKey) },
            });
          }
        })
        .catch(() => {
          // Fall through to form if auto-create fails
        });
    }
    // Only run on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const initialTitle = destination ?? "";

  const [quickDestination, setQuickDestination] = useState(initialTitle);
  const [showAdvanced, setShowAdvanced] = useState(false);

  const [title, setTitle] = useState(initialTitle);
  const [description, setDescription] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [dateError, setDateError] = useState("");

  // Quick create handler — creates trip with defaults and navigates to chat
  const handleQuickCreate = async () => {
    if (!quickDestination.trim()) return;

    try {
      const start = new Date();
      start.setDate(start.getDate() + 14);
      const end = new Date(start);
      end.setDate(end.getDate() + 6);

      const trip = await createTrip.mutateAsync({
        title: quickDestination.trim(),
        startDate: formatDate(start),
        endDate: formatDate(end),
      });
      if (trip) {
        router.replace(`/trips/${trip.id}/chat` as never);
      }
    } catch {
      // TanStack Query sets createTrip.isError automatically
    }
  };

  // Advanced form handler — original behavior
  const handleCreate = async () => {
    if (!title.trim()) return;

    if (startDate && endDate && endDate < startDate) {
      setDateError(t("tripCreate.dateRangeError"));
      return;
    }
    setDateError("");

    try {
      const trip = await createTrip.mutateAsync({
        title: title.trim(),
        description: description.trim() || undefined,
        startDate: startDate || undefined,
        endDate: endDate || undefined,
      });
      if (trip) {
        if (template) {
          router.replace({
            pathname: `/trips/${trip.id}/chat` as never,
            params: { suggestedPrompt: t(template.suggestedPromptKey) },
          });
        } else {
          router.replace(`/trips/${trip.id}/chat` as never);
        }
      }
    } catch {
      // TanStack Query sets createTrip.isError automatically
    }
  };

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surfaceSecondary },
    content: { padding: 16 },
    // Quick create styles
    quickSection: {
      backgroundColor: colors.surface,
      borderRadius: 16,
      padding: 20,
      marginBottom: 20,
      borderWidth: 1,
      borderColor: colors.border,
    },
    quickInput: {
      backgroundColor: colors.inputBg,
      borderWidth: 1,
      borderColor: colors.inputBorder,
      borderRadius: 12,
      padding: 16,
      fontSize: 17,
      color: colors.textPrimary,
      marginBottom: 12,
      textAlign: "center",
    },
    quickButton: {
      backgroundColor: colors.accent,
      borderRadius: 12,
      paddingVertical: 16,
      alignItems: "center",
    },
    quickButtonDisabled: { opacity: 0.5 },
    quickButtonText: { color: "#fff", fontSize: 17, fontWeight: "600" },
    // Divider with advanced toggle
    advancedToggle: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "center",
      gap: 6,
      paddingVertical: 12,
      marginBottom: 8,
    },
    advancedToggleText: {
      fontSize: 14,
      fontWeight: "500",
      color: colors.textTertiary,
    },
    // Advanced form styles (existing)
    label: { fontSize: 14, fontWeight: "600", color: colors.textPrimary, marginBottom: 6, marginTop: 16 },
    input: {
      backgroundColor: colors.inputBg,
      borderWidth: 1,
      borderColor: colors.inputBorder,
      borderRadius: 8,
      padding: 12,
      fontSize: 15,
      color: colors.textPrimary,
    },
    textArea: { minHeight: 80, textAlignVertical: "top" },
    dateRow: { flexDirection: "row", gap: 12 },
    dateField: { flex: 1 },
    submitButton: {
      backgroundColor: colors.accent,
      borderRadius: 8,
      padding: 16,
      alignItems: "center",
      marginTop: 24,
    },
    disabledButton: { opacity: 0.5 },
    submitText: { color: "#fff", fontSize: 16, fontWeight: "600" },
    errorCard: {
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
      backgroundColor: colors.errorBg,
      borderRadius: 10,
      padding: 12,
      marginTop: 12,
    },
    errorText: { color: colors.error, fontSize: 14, flex: 1 },
    templateBadge: {
      flexDirection: "row" as const,
      alignItems: "center" as const,
      gap: 6,
      backgroundColor: colors.accentSoft,
      borderRadius: 8,
      padding: 10,
      marginBottom: 8,
    },
    templateBadgeText: { color: colors.accent, fontSize: 13, fontWeight: "600" as const },
    // Loading overlay for template auto-create
    loadingContainer: {
      flex: 1,
      justifyContent: "center" as const,
      alignItems: "center" as const,
      backgroundColor: colors.surfaceSecondary,
    },
    loadingText: {
      fontSize: 16,
      color: colors.textSecondary,
      marginTop: 12,
    },
  });

  // Show loading screen while template auto-create is in progress
  if (template && createTrip.isPending) {
    return (
      <View style={styles.loadingContainer}>
        <ActivityIndicator size="large" color={colors.accent} />
        <Text style={styles.loadingText}>{t("tripCreate.submitting")}</Text>
      </View>
    );
  }

  // If template was provided but failed, fall through to the form
  // (template variable is still set but isPending is false and isError might be true)

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {/* Quick Create Section */}
      {!template && (
        <View style={styles.quickSection}>
          <TextInput
            style={styles.quickInput}
            placeholder={t("tripCreate.quickCreate.destinationPlaceholder")}
            placeholderTextColor={colors.textTertiary}
            value={quickDestination}
            onChangeText={setQuickDestination}
            autoFocus
            returnKeyType="go"
            onSubmitEditing={handleQuickCreate}
            accessibilityLabel="Trip destination"
            testID="quick-create-input"
          />
          <Pressable
            style={[
              styles.quickButton,
              (!quickDestination.trim() || createTrip.isPending) && styles.quickButtonDisabled,
            ]}
            onPress={handleQuickCreate}
            disabled={!quickDestination.trim() || createTrip.isPending}
            testID="quick-create-button"
          >
            {createTrip.isPending ? (
              <ActivityIndicator color="#fff" size="small" />
            ) : (
              <Text style={styles.quickButtonText}>
                {t("tripCreate.quickCreate.startPlanning")}
              </Text>
            )}
          </Pressable>
        </View>
      )}

      {/* Advanced Options Toggle */}
      {!template && (
        <Pressable
          style={styles.advancedToggle}
          onPress={() => setShowAdvanced(!showAdvanced)}
          testID="advanced-toggle"
        >
          <Text style={styles.advancedToggleText}>
            {showAdvanced
              ? t("tripCreate.quickCreate.hideAdvanced")
              : t("tripCreate.quickCreate.advancedOptions")}
          </Text>
          {showAdvanced ? (
            <ChevronUp color={colors.textTertiary} size={16} />
          ) : (
            <ChevronDown color={colors.textTertiary} size={16} />
          )}
        </Pressable>
      )}

      {/* Advanced Form (collapsed by default, always shown for templates) */}
      {(showAdvanced || template) && (
        <>
          {template && (
            <View style={styles.templateBadge}>
              <Sparkles color={colors.accent} size={14} />
              <Text style={styles.templateBadgeText}>{t("templates.fromTemplate")}</Text>
            </View>
          )}
          <Text style={styles.label}>{t("tripCreate.whereLabel")}</Text>
          <TextInput
            style={styles.input}
            placeholder={t("tripCreate.wherePlaceholder")}
            placeholderTextColor={colors.textTertiary}
            value={title}
            onChangeText={setTitle}
            autoFocus={!!template}
            accessibilityLabel="Trip title"
          />

          <Text style={styles.label}>{t("tripCreate.descriptionLabel")}</Text>
          <TextInput
            style={[styles.input, styles.textArea]}
            placeholder={t("tripCreate.descriptionPlaceholder")}
            placeholderTextColor={colors.textTertiary}
            value={description}
            onChangeText={setDescription}
            multiline
            numberOfLines={3}
            accessibilityLabel="Trip description"
          />

          <View style={styles.dateRow}>
            <View style={styles.dateField}>
              <DatePicker
                label={t("tripCreate.startDate")}
                value={startDate}
                onChange={(v) => { setStartDate(v); setDateError(""); }}
                placeholder="YYYY-MM-DD"
              />
            </View>
            <View style={styles.dateField}>
              <DatePicker
                label={t("tripCreate.endDate")}
                value={endDate}
                onChange={(v) => { setEndDate(v); setDateError(""); }}
                placeholder="YYYY-MM-DD"
              />
            </View>
          </View>

          {dateError ? (
            <View style={styles.errorCard}>
              <AlertCircle color={colors.error} size={16} />
              <Text style={styles.errorText}>{dateError}</Text>
            </View>
          ) : null}

          <Pressable
            style={[styles.submitButton, (!title.trim() || createTrip.isPending || !!dateError) && styles.disabledButton]}
            onPress={handleCreate}
            disabled={!title.trim() || createTrip.isPending || !!dateError}
          >
            {createTrip.isPending ? (
              <ActivityIndicator color="#fff" size="small" />
            ) : (
              <Text style={styles.submitText}>{t("tripCreate.submit")}</Text>
            )}
          </Pressable>
        </>
      )}

      {createTrip.isError && (
        <View style={styles.errorCard}>
          <AlertCircle color={colors.error} size={16} />
          <Text style={styles.errorText}>
            {createTrip.error instanceof Error ? createTrip.error.message : t("tripCreate.createError")}
          </Text>
        </View>
      )}
    </ScrollView>
  );
}
