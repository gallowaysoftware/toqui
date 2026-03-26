import { View, Text, TextInput, StyleSheet, Pressable, ScrollView, Alert, ActivityIndicator } from "react-native";
import { useState, useEffect } from "react";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import { useTrip, useUpdateTrip, useDeleteTrip } from "@/lib/hooks/useTrips";

export default function TripSettingsScreen() {
  const { tripId } = useLocalSearchParams<{ tripId: string }>();
  const { t } = useTranslation();
  const router = useRouter();
  const { trip, isLoading } = useTrip(tripId!);
  const updateTrip = useUpdateTrip();
  const deleteTrip = useDeleteTrip();

  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");

  useEffect(() => {
    if (trip) {
      setTitle(trip.title);
      setDescription(trip.description);
      setStartDate(trip.startDate);
      setEndDate(trip.endDate);
    }
  }, [trip]);

  if (isLoading || !trip) {
    return <View style={styles.center}><ActivityIndicator size="large" color="#e8654a" /></View>;
  }

  const handleSave = async () => {
    await updateTrip.mutateAsync({
      id: tripId!,
      title: title.trim(),
      description: description.trim(),
      startDate,
      endDate,
    });
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
            await deleteTrip.mutateAsync(tripId!);
            router.replace("/(tabs)" as never);
          },
        },
      ],
    );
  };

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <Text style={styles.label}>{t("tripSettings.editTitle")}</Text>
      <TextInput style={styles.input} value={title} onChangeText={setTitle} />

      <Text style={styles.label}>{t("tripSettings.editDescription")}</Text>
      <TextInput style={[styles.input, styles.textArea]} value={description} onChangeText={setDescription} multiline />

      <View style={styles.dateRow}>
        <View style={styles.dateField}>
          <Text style={styles.label}>{t("tripSettings.editStartDate")}</Text>
          <TextInput style={styles.input} value={startDate} onChangeText={setStartDate} placeholder="YYYY-MM-DD" placeholderTextColor="#999" />
        </View>
        <View style={styles.dateField}>
          <Text style={styles.label}>{t("tripSettings.editEndDate")}</Text>
          <TextInput style={styles.input} value={endDate} onChangeText={setEndDate} placeholder="YYYY-MM-DD" placeholderTextColor="#999" />
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

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#f5f5f5" },
  content: { padding: 16 },
  center: { flex: 1, justifyContent: "center", alignItems: "center" },
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
  saveButton: {
    backgroundColor: "#e8654a",
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
    backgroundColor: "#fff",
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "#fca5a5",
  },
  dangerTitle: { fontSize: 16, fontWeight: "600", color: "#ef4444", marginBottom: 8 },
  dangerWarning: { fontSize: 14, color: "#666", marginBottom: 16, lineHeight: 20 },
  deleteButton: {
    borderWidth: 1,
    borderColor: "#ef4444",
    borderRadius: 8,
    padding: 12,
    alignItems: "center",
  },
  deleteText: { color: "#ef4444", fontWeight: "600" },
});
