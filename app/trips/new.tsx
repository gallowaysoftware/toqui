import { View, Text, TextInput, Pressable, StyleSheet, ScrollView, ActivityIndicator } from "react-native";
import { useState } from "react";
import { useRouter, useLocalSearchParams } from "expo-router";
import { useTranslation } from "react-i18next";
import { useCreateTrip } from "@/lib/hooks/useTrips";
import { DatePicker } from "@/components/DatePicker";
import { useTheme } from "@/lib/theme";

export default function NewTripScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { colors } = useTheme();
  const { destination } = useLocalSearchParams<{ destination?: string }>();
  const createTrip = useCreateTrip();

  const [title, setTitle] = useState(destination ?? "");
  const [description, setDescription] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
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
        router.replace(`/trips/${trip.id}` as never);
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
    errorText: { color: colors.error, fontSize: 14, textAlign: "center", marginTop: 12 },
  });

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
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
        <Text style={styles.errorText}>{dateError}</Text>
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
        <Text style={styles.errorText}>
          {createTrip.error instanceof Error ? createTrip.error.message : "Failed to create trip"}
        </Text>
      )}
    </ScrollView>
  );
}
