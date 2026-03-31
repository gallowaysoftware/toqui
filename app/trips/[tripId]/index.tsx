import { View, Text, StyleSheet, Pressable, ScrollView, ActivityIndicator, Share, Alert } from "react-native";
import { useLocalSearchParams, useRouter, Stack } from "expo-router";
import { MessageCircle, Calendar, Settings, Play, CheckCircle, FileText, CalendarDays, Clock, AlertTriangle, Share2 } from "lucide-react-native";
import { useTranslation } from "react-i18next";
import { useState } from "react";
import { useTrip, useUpdateTrip } from "@/lib/hooks/useTrips";
import { useItinerary } from "@/lib/hooks/useItinerary";
import { ProUpgrade } from "@/components/checkout/ProUpgrade";
import { useTrialStatus } from "@/lib/hooks/useTrialStatus";
import { ItineraryTimeline } from "@/components/itinerary/ItineraryTimeline";
import { ItineraryMap } from "@/components/map/ItineraryMap";
import { exportItineraryPDF } from "@/lib/export/pdf-export";
import { exportItineraryICal } from "@/lib/export/calendar-export";
import { TripStatus } from "@gen/toqui/v1/trip_pb";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";

export default function TripDetailScreen() {
  const { t } = useTranslation();
  const { tripId } = useLocalSearchParams<{ tripId: string }>();
  const { trip, isLoading } = useTrip(tripId!);
  const { itinerary } = useItinerary(tripId!);
  const { isTrialActive, isTrialExpired, daysRemaining, isLastDay } = useTrialStatus(tripId!);
  const updateTrip = useUpdateTrip();
  const router = useRouter();
  const { accessToken } = useAuth();
  const [isSharing, setIsSharing] = useState(false);

  if (isLoading || !trip) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color="#BF4028" />
      </View>
    );
  }

  const isPlannable = trip.status === TripStatus.PLANNING;
  const isActive = trip.status === TripStatus.ACTIVE;

  const handleShare = async () => {
    setIsSharing(true);
    try {
      const res = await authFetch(
        `${getConfig().apiUrl}/api/trips/share`,
        accessToken,
        { method: "POST", body: JSON.stringify({ trip_id: tripId }) },
      );
      if (!res.ok) {
        throw new Error(`Failed to enable sharing (${res.status})`);
      }
      const data: { share_token: string } = await res.json();
      const shareUrl = `https://app.toqui.travel/shared/${data.share_token}`;
      await Share.share({
        message: `${trip.title} — ${shareUrl}`,
        url: shareUrl,
      });
    } catch (err) {
      if (err instanceof Error && err.message.includes("User did not share")) {
        // User cancelled the share sheet — not an error
        return;
      }
      Alert.alert(t("common.error"));
    } finally {
      setIsSharing(false);
    }
  };

  return (
    <>
      <Stack.Screen options={{ title: trip.title }} />
      <ScrollView style={styles.container} contentContainerStyle={styles.content}>
        {trip.description ? (
          <Text style={styles.description}>{trip.description}</Text>
        ) : null}

        {(trip.startDate || trip.endDate) && (
          <Text style={styles.dates}>
            {trip.startDate}{trip.startDate && trip.endDate ? " → " : ""}{trip.endDate}
          </Text>
        )}

        {isTrialActive && (
          <View style={styles.trialBanner}>
            <Clock color="#2563eb" size={16} />
            <Text style={styles.trialBannerText}>
              {t("trial.active")}
              {" — "}
              {isLastDay
                ? t("trial.hoursRemaining")
                : t("trial.daysRemaining", { days: daysRemaining })}
            </Text>
          </View>
        )}

        {isTrialExpired && (
          <View style={[styles.trialBanner, styles.trialBannerExpired]}>
            <AlertTriangle color="#b45309" size={16} />
            <Text style={styles.trialBannerExpiredText}>
              {t("trial.expired")} — {t("trial.upgradePrompt")}
            </Text>
          </View>
        )}

        <View style={styles.actions}>
          <Pressable
            style={styles.actionButton}
            onPress={() => router.push(`/trips/${tripId}/chat` as never)}
          >
            <MessageCircle color="#BF4028" size={24} />
            <Text style={styles.actionText}>Chat</Text>
          </Pressable>

          <Pressable
            style={styles.actionButton}
            onPress={() => router.push(`/trips/${tripId}/bookings` as never)}
          >
            <Calendar color="#BF4028" size={24} />
            <Text style={styles.actionText}>Bookings</Text>
          </Pressable>

          <Pressable
            style={styles.actionButton}
            onPress={() => router.push(`/trips/${tripId}/settings` as never)}
          >
            <Settings color="#BF4028" size={24} />
            <Text style={styles.actionText}>Settings</Text>
          </Pressable>

          <Pressable
            style={styles.actionButton}
            onPress={handleShare}
            disabled={isSharing}
          >
            {isSharing ? (
              <ActivityIndicator size="small" color="#BF4028" />
            ) : (
              <Share2 color="#BF4028" size={24} />
            )}
            <Text style={styles.actionText}>{t("referral.share")}</Text>
          </Pressable>
        </View>

        {itinerary && trip && (
          <>
            <View style={styles.exportRow}>
              <Pressable
                style={styles.exportButton}
                onPress={() => exportItineraryPDF(trip, itinerary)}
              >
                <FileText color="#BF4028" size={16} />
                <Text style={styles.exportText}>Export PDF</Text>
              </Pressable>
              <Pressable
                style={styles.exportButton}
                onPress={() => exportItineraryICal(trip, itinerary)}
              >
                <CalendarDays color="#BF4028" size={16} />
                <Text style={styles.exportText}>Export Calendar</Text>
              </Pressable>
            </View>
            <ItineraryMap itinerary={itinerary} />
            <ItineraryTimeline itinerary={itinerary} />
          </>
        )}

        <ProUpgrade tripId={tripId!} />

        {isPlannable && (
          <Pressable
            style={styles.statusButton}
            onPress={() => updateTrip.mutate({ id: tripId!, status: TripStatus.ACTIVE })}
            disabled={updateTrip.isPending}
          >
            <Play color="#22c55e" size={18} />
            <Text style={styles.statusButtonText}>Start Trip</Text>
          </Pressable>
        )}

        {isActive && (
          <Pressable
            style={styles.statusButton}
            onPress={() => updateTrip.mutate({ id: tripId!, status: TripStatus.COMPLETED })}
            disabled={updateTrip.isPending}
          >
            <CheckCircle color="#3b82f6" size={18} />
            <Text style={styles.statusButtonText}>Complete Trip</Text>
          </Pressable>
        )}
      </ScrollView>
    </>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#f5f5f5" },
  content: { padding: 16 },
  center: { flex: 1, justifyContent: "center", alignItems: "center" },
  description: { fontSize: 15, color: "#666", lineHeight: 22, marginBottom: 16 },
  dates: { fontSize: 14, color: "#999", marginBottom: 20 },
  actions: { flexDirection: "row", gap: 12, marginBottom: 24 },
  actionButton: {
    flex: 1,
    backgroundColor: "#fff",
    borderRadius: 12,
    padding: 16,
    alignItems: "center",
    gap: 8,
    borderWidth: 1,
    borderColor: "#e0e0e0",
  },
  actionText: { fontSize: 14, fontWeight: "500", color: "#333" },
  statusButton: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: 8,
    padding: 14,
    backgroundColor: "#fff",
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "#e0e0e0",
  },
  statusButtonText: { fontSize: 16, fontWeight: "600", color: "#333" },
  exportRow: { flexDirection: "row", gap: 10, marginBottom: 16 },
  exportButton: {
    flex: 1,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: 6,
    padding: 10,
    backgroundColor: "#fff",
    borderRadius: 8,
    borderWidth: 1,
    borderColor: "#e0e0e0",
  },
  exportText: { fontSize: 13, fontWeight: "500", color: "#BF4028" },
  trialBanner: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    backgroundColor: "#eff6ff",
    borderWidth: 1,
    borderColor: "#bfdbfe",
    borderRadius: 10,
    padding: 12,
    marginBottom: 16,
  },
  trialBannerText: { fontSize: 14, color: "#1e40af", fontWeight: "500", flex: 1 },
  trialBannerExpired: { backgroundColor: "#fffbeb", borderColor: "#fde68a" },
  trialBannerExpiredText: { fontSize: 14, color: "#92400e", fontWeight: "500", flex: 1 },
});
