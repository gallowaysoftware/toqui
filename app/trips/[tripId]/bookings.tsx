import { View, Text, StyleSheet, Pressable, FlatList, ActivityIndicator, TextInput } from "react-native";
import { confirmDestructive } from "@/lib/confirm";
import { useState, useCallback } from "react";
import { useLocalSearchParams } from "expo-router";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { Plus, Trash2, Plane, Hotel, Car, Train, Ticket, Utensils, MoreHorizontal, AlertCircle, RefreshCw, ClipboardList } from "lucide-react-native";
import { useBookings, useIngestBooking, useDeleteBooking } from "@/lib/hooks/useBookings";
import { BookingType } from "@gen/toqui/v1/booking_pb";
import type { Booking } from "@gen/toqui/v1/booking_pb";
import { useTheme } from "@/lib/theme";
import { useOfflineTrip } from "@/lib/offline";

const typeConfig: Record<number, { i18nKey: string; color: string; Icon: typeof Plane }> = {
  [BookingType.FLIGHT]: { i18nKey: "bookings.typeFlight", color: "#3b82f6", Icon: Plane },
  [BookingType.HOTEL]: { i18nKey: "bookings.typeHotel", color: "#8b5cf6", Icon: Hotel },
  [BookingType.CAR_RENTAL]: { i18nKey: "bookings.typeCarRental", color: "#f59e0b", Icon: Car },
  [BookingType.TRAIN]: { i18nKey: "bookings.typeTrain", color: "#10b981", Icon: Train },
  [BookingType.ACTIVITY]: { i18nKey: "bookings.typeActivity", color: "#ec4899", Icon: Ticket },
  [BookingType.RESTAURANT]: { i18nKey: "bookings.typeRestaurant", color: "#ef4444", Icon: Utensils },
  [BookingType.TOUR]: { i18nKey: "bookings.typeTour", color: "#06b6d4", Icon: Ticket },
  [BookingType.OTHER]: { i18nKey: "bookings.typeOther", color: "#6b7280", Icon: MoreHorizontal },
};

function BookingCard({ booking, onDelete }: { booking: Booking; onDelete: () => void }) {
  const { t } = useTranslation();
  const config = typeConfig[booking.type] ?? typeConfig[BookingType.OTHER]!;
  const { Icon } = config;
  const { colors } = useTheme();

  const cardStyles = StyleSheet.create({
    card: {
      flexDirection: "row",
      alignItems: "center",
      backgroundColor: colors.surface,
      borderRadius: 12,
      padding: 14,
      marginBottom: 10,
      borderWidth: 1,
      borderColor: colors.border,
    },
    typeIndicator: {
      width: 36,
      height: 36,
      borderRadius: 18,
      justifyContent: "center",
      alignItems: "center",
      marginRight: 12,
    },
    cardContent: { flex: 1 },
    cardTitle: { fontSize: 15, fontWeight: "600", color: colors.textPrimary },
    cardType: { fontSize: 12, color: colors.textSecondary, marginTop: 2 },
    cardMeta: { fontSize: 12, color: colors.textTertiary, marginTop: 1 },
  });

  return (
    <View style={cardStyles.card}>
      <View style={[cardStyles.typeIndicator, { backgroundColor: config.color }]}>
        <Icon color="#fff" size={16} />
      </View>
      <View style={cardStyles.cardContent}>
        <Text style={cardStyles.cardTitle} numberOfLines={1}>{booking.title || t("bookings.untitled")}</Text>
        <Text style={cardStyles.cardType}>{t(config.i18nKey)}</Text>
        {booking.provider ? <Text style={cardStyles.cardMeta}>{booking.provider}</Text> : null}
        {booking.confirmationCode ? <Text style={cardStyles.cardMeta}>#{booking.confirmationCode}</Text> : null}
      </View>
      <Pressable onPress={onDelete} hitSlop={8}>
        <Trash2 color={colors.textTertiary} size={18} />
      </Pressable>
    </View>
  );
}

export default function BookingsScreen() {
  const { tripId } = useLocalSearchParams<{ tripId: string }>();
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { bookings: networkBookings, isLoading: isNetworkLoading, error: bookingsError } = useBookings(tripId!);
  const ingestBooking = useIngestBooking();
  const deleteBooking = useDeleteBooking();
  const { colors } = useTheme();
  const [showAdd, setShowAdd] = useState(false);
  const [rawText, setRawText] = useState("");

  // Offline support: fall back to cached bookings when offline
  const { isOffline, bundle: offlineBundle, hasCachedData } = useOfflineTrip(tripId);
  const offlineBookings = isOffline && offlineBundle?.bookings
    ? offlineBundle.bookings.map((b) => ({ ...b } as unknown as Booking))
    : [];
  const bookings = networkBookings.length > 0 || !isOffline ? networkBookings : offlineBookings;
  const isLoading = isNetworkLoading && !(isOffline && hasCachedData);

  const handleIngest = useCallback(async () => {
    if (!rawText.trim()) return;
    await ingestBooking.mutateAsync({
      tripId: tripId!,
      type: BookingType.UNSPECIFIED,
      rawText: rawText.trim(),
    });
    setRawText("");
    setShowAdd(false);
  }, [rawText, tripId, ingestBooking]);

  const handleDelete = useCallback(async (id: string) => {
    const confirmed = await confirmDestructive({
      title: t("bookings.deleteTitle"),
      message: t("bookings.deleteConfirm"),
      confirmLabel: t("common.delete"),
      cancelLabel: t("common.cancel"),
    });
    if (!confirmed) return;
    deleteBooking.mutate({ id, tripId: tripId! });
  }, [t, tripId, deleteBooking]);

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surfaceSecondary },
    center: { flex: 1, justifyContent: "center", alignItems: "center" },
    list: { padding: 16 },
    empty: { alignItems: "center", paddingTop: 48, paddingBottom: 24 },
    emptyIcon: { marginBottom: 16 },
    emptyText: { fontSize: 18, fontWeight: "600", color: colors.textPrimary, marginBottom: 6 },
    emptySubtext: { fontSize: 14, color: colors.textSecondary },
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
    addButton: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "center",
      gap: 8,
      padding: 14,
      backgroundColor: colors.surface,
      borderRadius: 12,
      borderWidth: 1,
      borderColor: colors.accent,
      borderStyle: "dashed",
      marginBottom: 12,
    },
    addButtonText: { color: colors.accent, fontSize: 16, fontWeight: "600" },
    addForm: {
      backgroundColor: colors.surface,
      borderRadius: 12,
      padding: 14,
      marginBottom: 12,
      borderWidth: 1,
      borderColor: colors.border,
    },
    textArea: {
      borderWidth: 1,
      borderColor: colors.inputBorder,
      borderRadius: 8,
      padding: 12,
      fontSize: 14,
      minHeight: 100,
      textAlignVertical: "top",
      color: colors.textPrimary,
      backgroundColor: colors.inputBg,
    },
    addActions: { flexDirection: "row", justifyContent: "flex-end", gap: 10, marginTop: 12 },
    cancelButton: { padding: 10 },
    cancelText: { color: colors.textSecondary, fontWeight: "500" },
    submitButton: { backgroundColor: colors.accent, borderRadius: 8, paddingVertical: 10, paddingHorizontal: 20 },
    disabledButton: { opacity: 0.5 },
    submitText: { color: "#fff", fontWeight: "600" },
  });

  if (isLoading) {
    return <View style={styles.center}><ActivityIndicator size="large" color={colors.accent} /></View>;
  }

  if (bookingsError) {
    return (
      <View style={styles.errorContainer}>
        <View style={styles.errorCard}>
          <AlertCircle color={colors.error} size={40} style={styles.errorIcon as object} />
          <Text style={styles.errorTitle}>{t("bookings.loadError")}</Text>
          <Text style={styles.errorSubtitle}>{t("bookings.loadErrorSubtitle")}</Text>
          <Pressable
            style={styles.retryButton}
            onPress={() => void queryClient.invalidateQueries({ queryKey: ["bookings", tripId] })}
          >
            <RefreshCw color="#fff" size={16} />
            <Text style={styles.retryButtonText}>{t("common.retry")}</Text>
          </Pressable>
        </View>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <FlatList
        data={bookings}
        keyExtractor={(item) => item.id}
        renderItem={({ item }) => (
          <BookingCard booking={item} onDelete={() => handleDelete(item.id)} />
        )}
        contentContainerStyle={styles.list}
        ListEmptyComponent={
          <View style={styles.empty}>
            <ClipboardList color={colors.textTertiary} size={48} style={styles.emptyIcon as object} />
            <Text style={styles.emptyText}>{t("bookings.emptyTitle")}</Text>
            <Text style={styles.emptySubtext}>{t("bookings.emptySubtitle")}</Text>
          </View>
        }
        ListHeaderComponent={
          <>
          {isOffline && hasCachedData && (
            <View style={{ backgroundColor: colors.warningBg, borderRadius: 8, padding: 10, marginBottom: 12, borderWidth: 1, borderColor: colors.warningBorder }}>
              <Text style={{ fontSize: 13, color: colors.warning }} testID="bookings-offline-notice">
                You're viewing cached bookings. Adding or deleting bookings requires an internet connection.
              </Text>
            </View>
          )}
          {showAdd ? (
            <View style={styles.addForm}>
              <TextInput
                style={styles.textArea}
                placeholder={t("bookings.pastePlaceholder")}
                placeholderTextColor={colors.textTertiary}
                value={rawText}
                onChangeText={setRawText}
                multiline
                numberOfLines={5}
                autoFocus
              />
              <View style={styles.addActions}>
                <Pressable style={styles.cancelButton} onPress={() => { setShowAdd(false); setRawText(""); }}>
                  <Text style={styles.cancelText}>{t("common.cancel")}</Text>
                </Pressable>
                <Pressable
                  style={[styles.submitButton, (!rawText.trim() || ingestBooking.isPending) && styles.disabledButton]}
                  onPress={handleIngest}
                  disabled={!rawText.trim() || ingestBooking.isPending}
                >
                  {ingestBooking.isPending ? (
                    <ActivityIndicator color="#fff" size="small" />
                  ) : (
                    <Text style={styles.submitText}>{t("bookings.addBooking")}</Text>
                  )}
                </Pressable>
              </View>
            </View>
          ) : (
            <Pressable
              style={[styles.addButton, isOffline && { opacity: 0.5 }]}
              onPress={() => setShowAdd(true)}
              disabled={isOffline}
            >
              <Plus color={colors.accent} size={18} />
              <Text style={styles.addButtonText}>{t("bookings.addBooking")}</Text>
            </Pressable>
          )}
          </>
        }
      />
    </View>
  );
}
