import { View, Text, TextInput, StyleSheet, Pressable, ScrollView, Alert, ActivityIndicator } from "react-native";
import { useState, useEffect } from "react";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { AlertCircle, RefreshCw, CheckCircle } from "lucide-react-native";
import { useTrip, useUpdateTrip, useDeleteTrip } from "@/lib/hooks/useTrips";
import { DatePicker } from "@/components/DatePicker";
import { useTheme } from "@/lib/theme";

export default function TripSettingsScreen() {
  const { tripId } = useLocalSearchParams<{ tripId: string }>();
  const { t } = useTranslation();
  const router = useRouter();
  const { colors } = useTheme();
  const queryClient = useQueryClient();
  const { trip, isLoading, error: tripError } = useTrip(tripId!);
  const updateTrip = useUpdateTrip();
  const deleteTrip = useDeleteTrip();

  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  useEffect(() => {
    if (trip) {
      setTitle(trip.title);
      setDescription(trip.description);
      setStartDate(trip.startDate);
      setEndDate(trip.endDate);
    }
  }, [trip]);

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surfaceSecondary },
    content: { padding: 16 },
    center: { flex: 1, justifyContent: "center", alignItems: "center" },
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
    saveButton: {
      backgroundColor: colors.accent,
      borderRadius: 8,
      padding: 14,
      alignItems: "center",
      marginTop: 24,
    },
    disabledButton: { opacity: 0.5 },
    saveText: { color: "#fff", fontSize: 16, fontWeight: "600" },
    dangerZone: {
      marginTop: 40,
      padding: 16,
      backgroundColor: colors.surface,
      borderRadius: 12,
      borderWidth: 1,
      borderColor: colors.error,
    },
    dangerTitle: { fontSize: 16, fontWeight: "600", color: colors.error, marginBottom: 8 },
    dangerWarning: { fontSize: 14, color: colors.textSecondary, marginBottom: 16, lineHeight: 20 },
    deleteButton: {
      borderWidth: 1,
      borderColor: colors.error,
      borderRadius: 8,
      padding: 12,
      alignItems: "center",
    },
    deleteText: { color: colors.error, fontWeight: "600" },
    feedbackBanner: {
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
      borderRadius: 10,
      padding: 12,
      marginTop: 12,
    },
    successBanner: { backgroundColor: colors.successBg },
    errorBanner: { backgroundColor: colors.errorBg },
    feedbackText: { fontSize: 14, fontWeight: "500", flex: 1 },
    successText: { color: colors.success },
    errorText: { color: colors.error },
    errorContainer: {
      flex: 1,
      justifyContent: "center",
      alignItems: "center",
      padding: 24,
      backgroundColor: colors.surfaceSecondary,
    },
    errorCard: {
      backgroundColor: colors.errorBg,
      borderRadius: 16,
      padding: 24,
      alignItems: "center",
      maxWidth: 320,
      width: "100%",
    },
    errorIcon: { marginBottom: 12 },
    errorTitle: { fontSize: 18, fontWeight: "600", color: colors.textPrimary, marginBottom: 6, textAlign: "center" },
    errorSubtitle: { fontSize: 14, color: colors.textSecondary, textAlign: "center", marginBottom: 20 },
    retryButton: {
      backgroundColor: colors.accent,
      borderRadius: 8,
      paddingVertical: 12,
      paddingHorizontal: 28,
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
    },
    retryButtonText: { color: "#fff", fontSize: 15, fontWeight: "600" },
  });

  if (isLoading) {
    return <View style={styles.center}><ActivityIndicator size="large" color={colors.accent} /></View>;
  }

  if (tripError || !trip) {
    return (
      <View style={styles.errorContainer}>
        <View style={styles.errorCard}>
          <AlertCircle color={colors.error} size={40} style={styles.errorIcon as object} />
          <Text style={styles.errorTitle}>{t("tripSettings.loadError")}</Text>
          <Text style={styles.errorSubtitle}>{t("tripSettings.loadErrorSubtitle")}</Text>
          <Pressable
            style={styles.retryButton}
            onPress={() => void queryClient.invalidateQueries({ queryKey: ["trip", tripId] })}
          >
            <RefreshCw color="#fff" size={16} />
            <Text style={styles.retryButtonText}>{t("common.retry")}</Text>
          </Pressable>
        </View>
      </View>
    );
  }

  const handleSave = async () => {
    setSaveSuccess(false);
    setSaveError(null);
    try {
      await updateTrip.mutateAsync({
        id: tripId!,
        title: title.trim(),
        description: description.trim(),
        startDate,
        endDate,
      });
      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 3000);
    } catch {
      setSaveError(t("tripSettings.saveError"));
    }
  };

  const handleDelete = () => {
    Alert.alert(
      t("tripSettings.deleteTrip"),
      t("tripSettings.deleteWarning"),
      [
        { text: t("common.cancel"), style: "cancel" },
        {
          text: t("tripSettings.deleteConfirm"),
          style: "destructive",
          onPress: async () => {
            try {
              await deleteTrip.mutateAsync(tripId!);
              router.replace("/(tabs)" as never);
            } catch {
              Alert.alert(t("common.error"), t("tripSettings.deleteError"));
            }
          },
        },
      ],
    );
  };

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <Text style={styles.label}>{t("tripSettings.editTitle")}</Text>
      <TextInput style={styles.input} value={title} onChangeText={setTitle} accessibilityLabel="Trip title" />

      <Text style={styles.label}>{t("tripSettings.editDescription")}</Text>
      <TextInput style={[styles.input, styles.textArea]} value={description} onChangeText={setDescription} multiline accessibilityLabel="Trip description" />

      <View style={styles.dateRow}>
        <View style={styles.dateField}>
          <DatePicker
            label={t("tripSettings.editStartDate")}
            value={startDate}
            onChange={setStartDate}
            placeholder="YYYY-MM-DD"
          />
        </View>
        <View style={styles.dateField}>
          <DatePicker
            label={t("tripSettings.editEndDate")}
            value={endDate}
            onChange={setEndDate}
            placeholder="YYYY-MM-DD"
          />
        </View>
      </View>

      <Pressable
        style={[styles.saveButton, updateTrip.isPending && styles.disabledButton]}
        onPress={handleSave}
        disabled={updateTrip.isPending}
      >
        <Text style={styles.saveText}>
          {updateTrip.isPending ? t("tripSettings.saving") : t("tripSettings.save")}
        </Text>
      </Pressable>

      {saveSuccess && (
        <View style={[styles.feedbackBanner, styles.successBanner]}>
          <CheckCircle color={colors.success} size={16} />
          <Text style={[styles.feedbackText, styles.successText]}>{t("tripSettings.saved")}</Text>
        </View>
      )}

      {saveError && (
        <View style={[styles.feedbackBanner, styles.errorBanner]}>
          <AlertCircle color={colors.error} size={16} />
          <Text style={[styles.feedbackText, styles.errorText]}>{saveError}</Text>
        </View>
      )}

      <View style={styles.dangerZone}>
        <Text style={styles.dangerTitle}>{t("tripSettings.deleteTrip")}</Text>
        <Text style={styles.dangerWarning}>{t("tripSettings.deleteWarning")}</Text>
        <Pressable
          style={[styles.deleteButton, deleteTrip.isPending && styles.disabledButton]}
          onPress={handleDelete}
          disabled={deleteTrip.isPending}
        >
          <Text style={styles.deleteText}>
            {deleteTrip.isPending ? t("tripSettings.deleting") : t("tripSettings.deleteConfirm")}
          </Text>
        </Pressable>
      </View>
    </ScrollView>
  );
}
