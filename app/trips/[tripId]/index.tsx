import { View, Text, StyleSheet, Pressable, ScrollView, ActivityIndicator, Share, Alert } from "react-native";
import { useLocalSearchParams, useRouter, Stack } from "expo-router";
import { MessageCircle, Calendar, Settings, Play, CheckCircle, FileText, CalendarDays, Clock, AlertTriangle, Share2, X, AlertCircle, RefreshCw } from "lucide-react-native";
import { useTranslation } from "react-i18next";
import { useState, useEffect } from "react";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { useTrip, useUpdateTrip } from "@/lib/hooks/useTrips";
import { useItinerary } from "@/lib/hooks/useItinerary";
import { ProUpgrade } from "@/components/checkout/ProUpgrade";
import { useTrialStatus } from "@/lib/hooks/useTrialStatus";
import { useDestinationGuide } from "@/lib/hooks/useDestinationGuide";
import { ItineraryTimeline } from "@/components/itinerary/ItineraryTimeline";
import { ItineraryMap } from "@/components/map/ItineraryMap";
import { exportItineraryPDF } from "@/lib/export/pdf-export";
import { exportItineraryICal } from "@/lib/export/calendar-export";
import { TripStatus } from "@gen/toqui/v1/trip_pb";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";
import { useTheme } from "@/lib/theme";
import { useQueryClient } from "@tanstack/react-query";

function formatTripDate(dateStr: string): string {
  const date = new Date(`${dateStr}T00:00:00Z`);
  return new Intl.DateTimeFormat("en-US", {
    month: "long",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  }).format(date);
}

function countDays(startDate: string, endDate: string): number {
  const start = new Date(startDate + "T00:00:00Z");
  const end = new Date(endDate + "T00:00:00Z");
  return Math.round((end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24)) + 1;
}

export default function TripDetailScreen() {
  const { t } = useTranslation();
  const { tripId } = useLocalSearchParams<{ tripId: string }>();
  const { trip, isLoading, error: tripError } = useTrip(tripId!);
  const queryClient = useQueryClient();
  const { itinerary, coveredDays, isLoading: isItineraryLoading } = useItinerary(tripId!);
  const { isTrialActive, isTrialExpired, daysRemaining, isLastDay } = useTrialStatus(tripId!);
  const updateTrip = useUpdateTrip();
  const { guide } = useDestinationGuide(trip?.destinationCountry || undefined);
  const router = useRouter();
  const { accessToken } = useAuth();
  const { colors } = useTheme();
  const [isSharing, setIsSharing] = useState(false);
  const [bannerDismissed, setBannerDismissed] = useState(false);

  const dismissalKey = `toqui_planning_dismissed_${tripId}`;

  useEffect(() => {
    AsyncStorage.getItem(dismissalKey).then((val) => {
      if (val === "true") setBannerDismissed(true);
    });
  }, [dismissalKey]);

  const handleDismissBanner = () => {
    setBannerDismissed(true);
    void AsyncStorage.setItem(dismissalKey, "true");
  };

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surfaceSecondary },
    content: { padding: 16 },
    center: { flex: 1, justifyContent: "center", alignItems: "center" },
    description: { fontSize: 15, color: colors.textSecondary, lineHeight: 22, marginBottom: 16 },
    dates: { fontSize: 14, color: colors.textTertiary, marginBottom: 20 },
    actions: { flexDirection: "row", gap: 12, marginBottom: 24 },
    actionButton: {
      flex: 1,
      backgroundColor: colors.surface,
      borderRadius: 12,
      padding: 16,
      alignItems: "center",
      gap: 8,
      borderWidth: 1,
      borderColor: colors.border,
    },
    actionText: { fontSize: 14, fontWeight: "500", color: colors.textPrimary },
    statusButton: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "center",
      gap: 8,
      padding: 14,
      backgroundColor: colors.surface,
      borderRadius: 12,
      borderWidth: 1,
      borderColor: colors.border,
    },
    statusButtonText: { fontSize: 16, fontWeight: "600", color: colors.textPrimary },
    exportRow: { flexDirection: "row", gap: 10, marginBottom: 16 },
    exportButton: {
      flex: 1,
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "center",
      gap: 6,
      padding: 10,
      backgroundColor: colors.surface,
      borderRadius: 8,
      borderWidth: 1,
      borderColor: colors.border,
    },
    exportText: { fontSize: 13, fontWeight: "500", color: colors.accent },
    trialBanner: {
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
      backgroundColor: colors.infoBg,
      borderWidth: 1,
      borderColor: colors.infoBorder,
      borderRadius: 10,
      padding: 12,
      marginBottom: 16,
    },
    trialBannerText: { fontSize: 14, color: colors.info, fontWeight: "500", flex: 1 },
    trialBannerExpired: { backgroundColor: colors.warningBg, borderColor: colors.warningBorder },
    trialBannerExpiredText: { fontSize: 14, color: colors.warning, fontWeight: "500", flex: 1 },
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
    continuationBanner: {
      backgroundColor: colors.accentSoft,
      borderLeftWidth: 3,
      borderLeftColor: colors.accent,
      borderRadius: 10,
      padding: 12,
      marginBottom: 16,
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
    },
    continuationBannerContent: { flex: 1 },
    continuationBannerText: { fontSize: 14, color: colors.textPrimary, fontWeight: "500", marginBottom: 6 },
    continuationBannerCta: { fontSize: 14, color: colors.accent, fontWeight: "600" },
    continuationBannerDismiss: { padding: 4 },
    guideCard: {
      backgroundColor: colors.surface,
      borderRadius: 12,
      borderWidth: 1,
      borderColor: colors.border,
      padding: 16,
      marginBottom: 16,
    },
    guideHeader: { fontSize: 16, fontWeight: "700", color: colors.textPrimary, marginBottom: 10 },
    guideExcerpt: { fontSize: 14, color: colors.textSecondary, lineHeight: 20, marginBottom: 12 },
    guideSection: { fontSize: 13, fontWeight: "600", color: colors.textPrimary, marginBottom: 6 },
  });

  if (isLoading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color={colors.accent} />
      </View>
    );
  }

  if (tripError || !trip) {
    return (
      <View style={styles.errorContainer}>
        <View style={styles.errorCard}>
          <AlertCircle color={colors.error} size={40} style={styles.errorIcon as object} />
          <Text style={styles.errorTitle}>{t("tripDetail.loadError")}</Text>
          <Text style={styles.errorSubtitle}>{t("tripDetail.loadErrorSubtitle")}</Text>
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

  const isPlannable = trip.status === TripStatus.PLANNING;
  const isActive = trip.status === TripStatus.ACTIVE;

  const totalDays =
    trip.startDate && trip.endDate ? countDays(trip.startDate, trip.endDate) : 0;
  const showContinuationBanner =
    isPlannable &&
    totalDays > 0 &&
    !isItineraryLoading &&
    coveredDays < totalDays * 0.7 &&
    !bannerDismissed;

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
            {trip.startDate ? formatTripDate(trip.startDate) : ""}
            {trip.startDate && trip.endDate ? " → " : ""}
            {trip.endDate ? formatTripDate(trip.endDate) : ""}
          </Text>
        )}

        {isTrialActive && (
          <View style={styles.trialBanner}>
            <Clock color={colors.info} size={16} />
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
            <AlertTriangle color={colors.warning} size={16} />
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
            <MessageCircle color={colors.accent} size={24} />
            <Text style={styles.actionText}>Chat</Text>
          </Pressable>

          <Pressable
            style={styles.actionButton}
            onPress={() => router.push(`/trips/${tripId}/bookings` as never)}
          >
            <Calendar color={colors.accent} size={24} />
            <Text style={styles.actionText}>Bookings</Text>
          </Pressable>

          <Pressable
            style={styles.actionButton}
            onPress={() => router.push(`/trips/${tripId}/settings` as never)}
          >
            <Settings color={colors.accent} size={24} />
            <Text style={styles.actionText}>Settings</Text>
          </Pressable>

          <Pressable
            style={styles.actionButton}
            onPress={handleShare}
            disabled={isSharing}
          >
            {isSharing ? (
              <ActivityIndicator size="small" color={colors.accent} />
            ) : (
              <Share2 color={colors.accent} size={24} />
            )}
            <Text style={styles.actionText}>{t("referral.share")}</Text>
          </Pressable>
        </View>

        {guide && (
          <View style={styles.guideCard}>
            <Text style={styles.guideHeader}>
              {`${guide.destination} Travel Guide`}
            </Text>
            <Text style={styles.guideExcerpt} numberOfLines={3}>
              {guide.excerpt}
            </Text>
            {guide.persona_name ? (
              <Text style={styles.guideSection}>
                {`By ${guide.persona_name} — ${guide.persona_specialty}`}
              </Text>
            ) : null}
          </View>
        )}

        {showContinuationBanner && (
          <View style={styles.continuationBanner}>
            <View style={styles.continuationBannerContent}>
              <Text style={styles.continuationBannerText}>
                {`Only ${coveredDays} of ${totalDays} days planned`}
              </Text>
              <Pressable onPress={() => router.push(`/trips/${tripId}/chat` as never)}>
                <Text style={styles.continuationBannerCta}>Continue planning →</Text>
              </Pressable>
            </View>
            <Pressable style={styles.continuationBannerDismiss} onPress={handleDismissBanner}>
              <X color={colors.textTertiary} size={16} />
            </Pressable>
          </View>
        )}

        {itinerary && trip && (
          <>
            <View style={styles.exportRow}>
              <Pressable
                style={styles.exportButton}
                onPress={() => exportItineraryPDF(trip, itinerary)}
              >
                <FileText color={colors.accent} size={16} />
                <Text style={styles.exportText}>Export PDF</Text>
              </Pressable>
              <Pressable
                style={styles.exportButton}
                onPress={() => exportItineraryICal(trip, itinerary)}
              >
                <CalendarDays color={colors.accent} size={16} />
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
