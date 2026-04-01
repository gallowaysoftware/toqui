import { View, Text, TextInput, Pressable, StyleSheet, ScrollView, ActivityIndicator } from "react-native";
import { useState, useMemo } from "react";
import { useRouter, useLocalSearchParams } from "expo-router";
import { useTranslation } from "react-i18next";
import { AlertCircle, Sparkles } from "lucide-react-native";
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

  const initialTitle = template ? t(template.titleKey) : (destination ?? "");
  const initialDescription = template ? t(template.descriptionKey) : "";
  const initialStartDate = template ? formatDate(new Date()) : "";
  const initialEndDate = useMemo(() => {
    if (!template) return "";
    const end = new Date();
    end.setDate(end.getDate() + template.duration - 1);
    return formatDate(end);
  }, [template]);

  const [title, setTitle] = useState(initialTitle);
  const [description, setDescription] = useState(initialDescription);
  const [startDate, setStartDate] = useState(initialStartDate);
  const [endDate, setEndDate] = useState(initialEndDate);
  const [dateError, setDateError] = useState("");

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
          router.replace(`/trips/${trip.id}` as never);
        }
      }
    } catch {
      // TanStack Query sets createTrip.isError automatically
    }
  };

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surfaceSecondary },
    content: { padding: 16 },
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
  });

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
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
        autoFocus
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
