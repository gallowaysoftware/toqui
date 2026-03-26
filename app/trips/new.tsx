import { View, Text, TextInput, Pressable, StyleSheet, ScrollView, ActivityIndicator } from "react-native";
import { useState } from "react";
import { useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import { useCreateTrip } from "@/lib/hooks/useTrips";

export default function NewTripScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const createTrip = useCreateTrip();

  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");

  const handleCreate = async () => {
    if (!title.trim()) return;
    const trip = await createTrip.mutateAsync({
      title: title.trim(),
      description: description.trim() || undefined,
      startDate: startDate || undefined,
      endDate: endDate || undefined,
    });
    if (trip) {
      router.replace(`/trips/${trip.id}/chat` as never);
    }
  };

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <Text style={styles.label}>{t("tripCreate.whereLabel")}</Text>
      <TextInput
        style={styles.input}
        placeholder={t("tripCreate.wherePlaceholder")}
        placeholderTextColor="#999"
        value={title}
        onChangeText={setTitle}
        autoFocus
      />

      <Text style={styles.label}>{t("tripCreate.descriptionLabel")}</Text>
      <TextInput
        style={[styles.input, styles.textArea]}
        placeholder={t("tripCreate.descriptionPlaceholder")}
        placeholderTextColor="#999"
        value={description}
        onChangeText={setDescription}
        multiline
        numberOfLines={3}
      />

      <View style={styles.dateRow}>
        <View style={styles.dateField}>
          <Text style={styles.label}>{t("tripCreate.startDate")}</Text>
          <TextInput
            style={styles.input}
            placeholder="YYYY-MM-DD"
            placeholderTextColor="#999"
            value={startDate}
            onChangeText={setStartDate}
          />
        </View>
        <View style={styles.dateField}>
          <Text style={styles.label}>{t("tripCreate.endDate")}</Text>
          <TextInput
            style={styles.input}
            placeholder="YYYY-MM-DD"
            placeholderTextColor="#999"
            value={endDate}
            onChangeText={setEndDate}
          />
        </View>
      </View>

      <Pressable
        style={[styles.submitButton, (!title.trim() || createTrip.isPending) && styles.disabledButton]}
        onPress={handleCreate}
        disabled={!title.trim() || createTrip.isPending}
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

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#f5f5f5" },
  content: { padding: 16 },
  label: { fontSize: 14, fontWeight: "600", color: "#333", marginBottom: 6, marginTop: 16 },
  input: {
    backgroundColor: "#fff",
    borderWidth: 1,
    borderColor: "#ddd",
    borderRadius: 8,
    padding: 12,
    fontSize: 15,
    color: "#333",
  },
  textArea: { minHeight: 80, textAlignVertical: "top" },
  dateRow: { flexDirection: "row", gap: 12 },
  dateField: { flex: 1 },
  submitButton: {
    backgroundColor: "#e8654a",
    borderRadius: 8,
    padding: 16,
    alignItems: "center",
    marginTop: 24,
  },
  disabledButton: { opacity: 0.5 },
  submitText: { color: "#fff", fontSize: 16, fontWeight: "600" },
  errorText: { color: "#ef4444", fontSize: 14, textAlign: "center", marginTop: 12 },
});
