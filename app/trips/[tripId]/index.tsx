import { View, Text, StyleSheet, Pressable, ScrollView, ActivityIndicator, Share, Alert } from "react-native";
import { useLocalSearchParams, useRouter, Stack } from "expo-router";
import { MessageCircle, Calendar, Settings, Play, CheckCircle, FileText, CalendarDays, Clock, AlertTriangle, Share2, X, AlertCircle, RefreshCw, Send, Eye, Users, Crown } from "lucide-react-native";
import { useTranslation } from "react-i18next";
import { useState, useEffect } from "react";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { useTrip, useUpdateTrip } from "@/lib/hooks/useTrips";
import { useItinerary } from "@/lib/hooks/useItinerary";
import { useWeather } from "@/lib/hooks/useWeather";
import { ProUpgrade } from "@/components/checkout/ProUpgrade";
import { useTrialStatus } from "@/lib/hooks/useTrialStatus";
import { useDestinationGuide } from "@/lib/hooks/useDestinationGuide";
import { WeatherCard } from "@/components/weather/WeatherCard";
import { ItineraryTimeline } from "@/components/itinerary/ItineraryTimeline";
import { ItineraryMap } from "@/components/map/ItineraryMap";
import { exportItineraryPDF } from "@/lib/export/pdf-export";
import { exportItineraryICal } from "@/lib/export/calendar-export";
import { TripStatus } from "@gen/toqui/v1/trip_pb";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";
import { useTheme } from "@/lib/theme";
import { ShareNudgeBanner } from "@/components/share/ShareNudgeBanner";
import { useCollaborators } from "@/lib/hooks/useCollaborators";
import { MemberAvatar } from "@/components/collaborators/MemberAvatar";
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
  const { collaborators } = useCollaborators(tripId!);
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
  const [shareNudgeDismissed, setShareNudgeDismissed] = useState(false);
  const [shareViewCount, setShareViewCount] = useState<number | null>(null);
  const [proBannerDismissed, setProBannerDismissed] = useState(false);
  const [hasChatVisit, setHasChatVisit] = useState(false);

  // Extract first available coordinates from itinerary for weather lookup
  const firstLocation = itinerary?.days
    ?.flatMap((d) => d.items)
    .find((item) => item.location)?.location ?? null;
  const { weather, isClimate } = useWeather(
    firstLocation?.latitude ?? null,
    firstLocation?.longitude ?? null,
    trip?.startDate || null,
    trip?.endDate || null,
  );

  const dismissalKey = `toqui_planning_dismissed_${tripId}`;
  const shareNudgeKey = `toqui_share_nudge_dismissed_${tripId}`;
  const proBannerKey = `toqui_pro_dismissed_${tripId}`;
  const chatVisitKey = `toqui_chat_visited_${tripId}`;

  useEffect(() => {
    AsyncStorage.getItem(dismissalKey).then((val) => {
      if (val === "true") setBannerDismissed(true);
    });
    AsyncStorage.getItem(shareNudgeKey).then((val) => {
      if (val === "true") setShareNudgeDismissed(true);
    });
    AsyncStorage.getItem(proBannerKey).then((val) => {
      if (val === "true") setProBannerDismissed(true);
    });
    AsyncStorage.getItem(chatVisitKey).then((val) => {
      if (val === "true") setHasChatVisit(true);
    });
  }, [dismissalKey, shareNudgeKey, proBannerKey, chatVisitKey]);

  // Fetch share view count if trip has been shared
  useEffect(() => {
    if (!tripId || !accessToken) return;
    authFetch(
      `${getConfig().apiUrl}/api/trips/share/stats?trip_id=${encodeURIComponent(tripId)}`,
      accessToken,
    )
      .then((res) => {
        if (res.ok) return res.json();
        return null;
      })
      .then((data: { view_count?: number } | null) => {
        if (data && typeof data.view_count === "number" && data.view_count > 0) {
          setShareViewCount(data.view_count);
        }
      })
      .catch(() => {
        // Share stats are non-critical — silently ignore errors
      });
  }, [tripId, accessToken]);

  const handleDismissBanner = () => {
    setBannerDismissed(true);
    void AsyncStorage.setItem(dismissalKey, "true");
  };

  const handleDismissShareNudge = () => {
    setShareNudgeDismissed(true);
    void AsyncStorage.setItem(shareNudgeKey, "true");
  };

  const handleDismissProBanner = () => {
    setProBannerDismissed(true);
    void AsyncStorage.setItem(proBannerKey, "true");
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
    memberStrip: {
      flexDirection: "row",
      alignItems: "center",
      gap: 12,
      backgroundColor: colors.surface,
      borderRadius: 12,
      borderWidth: 1,
      borderColor: colors.border,
      paddingVertical: 12,
      paddingHorizontal: 14,
      marginBottom: 16,
    },
    memberStripAvatars: { flexDirection: "row" },
    memberStripAvatar: { marginRight: -10 },
    memberStripText: { flex: 1, fontSize: 14, color: colors.textPrimary, fontWeight: "600" },
    memberStripSubtext: { fontSize: 12, color: colors.textSecondary, marginTop: 2 },
    memberStripBadge: {
      flexDirection: "row",
      alignItems: "center",
      gap: 4,
      paddingHorizontal: 8,
      paddingVertical: 4,
      borderRadius: 999,
      backgroundColor: colors.accentSoft,
    },
    memberStripBadgeText: { fontSize: 11, fontWeight: "700", color: colors.accent },
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
    emptyStateContainer: {
      alignItems: "center",
      paddingVertical: 32,
      paddingHorizontal: 16,
    },
    emptyStateIconCircle: {
      width: 80,
      height: 80,
      borderRadius: 40,
      backgroundColor: colors.accentSoft,
      justifyContent: "center",
      alignItems: "center",
      marginBottom: 20,
    },
    emptyStateHeadline: {
      fontSize: 22,
      fontWeight: "700",
      color: colors.textPrimary,
      textAlign: "center",
      marginBottom: 8,
    },
    emptyStateSubtitle: {
      fontSize: 15,
      color: colors.textSecondary,
      textAlign: "center",
      lineHeight: 22,
      marginBottom: 24,
      maxWidth: 280,
    },
    emptyStatePrimaryButton: {
      backgroundColor: colors.accent,
      borderRadius: 12,
      paddingVertical: 16,
      paddingHorizontal: 32,
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
      marginBottom: 20,
      width: "100%",
      maxWidth: 320,
      justifyContent: "center",
    },
    emptyStatePrimaryButtonText: {
      color: "#fff",
      fontSize: 17,
      fontWeight: "600",
    },
    suggestionChipsRow: {
      flexDirection: "row",
      flexWrap: "wrap",
      justifyContent: "center",
      gap: 8,
    },
    suggestionChip: {
      backgroundColor: colors.surface,
      borderWidth: 1,
      borderColor: colors.border,
      borderRadius: 20,
      paddingVertical: 8,
      paddingHorizontal: 16,
    },
    suggestionChipText: {
      fontSize: 14,
      color: colors.accent,
      fontWeight: "500",
    },
    shareStats: {
      flexDirection: "row",
      alignItems: "center",
      gap: 6,
      paddingHorizontal: 4,
      marginTop: -16,
      marginBottom: 24,
    },
    shareStatsText: { fontSize: 13, color: colors.textTertiary },
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

  const hasItinerary = !isItineraryLoading && itinerary && itinerary.days && itinerary.days.length > 0;
  const showEmptyState = !isItineraryLoading && !hasItinerary;

  const SUGGESTION_CHIPS = [
    { key: "buildItinerary", labelKey: "tripDetail.emptyState.buildItinerary" },
    { key: "findFlights", labelKey: "tripDetail.emptyState.findFlights" },
    { key: "whereToStay", labelKey: "tripDetail.emptyState.whereToStay" },
  ] as const;

  const totalDays =
    trip.startDate && trip.endDate ? countDays(trip.startDate, trip.endDate) : 0;
  const showContinuationBanner =
    isPlannable &&
    totalDays > 0 &&
    !isItineraryLoading &&
    coveredDays < totalDays * 0.7 &&
    !bannerDismissed;

  const showShareNudge = hasItinerary && !shareNudgeDismissed;

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

        {showShareNudge && (
          <ShareNudgeBanner onShare={handleShare} onDismiss={handleDismissShareNudge} />
        )}

        {(() => {
          // Always show the member strip so the entry point is discoverable.
          // Owner is implicit on the trip object — fold them into the preview
          // when they're not yet listed in the collaborators array.
          const ownerEmail = trip?.userId ? trip.userId : "owner";
          const previewMembers = collaborators.length > 0
            ? collaborators
            : [{ id: "owner", email: ownerEmail, role: "owner" as const }];
          const memberCount = collaborators.length || 1;
          const visible = previewMembers.slice(0, 4);
          const overflow = Math.max(0, previewMembers.length - visible.length);
          const isUnlocked = trip?.isUnlocked ?? false;
          return (
            <Pressable
              style={styles.memberStrip}
              onPress={() => router.push(`/trips/${tripId}/members` as never)}
              accessibilityRole="button"
              accessibilityLabel={t("collaborators.membersButton")}
            >
              <View style={styles.memberStripAvatars}>
                {visible.map((m, idx) => (
                  <View
                    key={m.id}
                    style={idx < visible.length - 1 ? styles.memberStripAvatar : undefined}
                  >
                    <MemberAvatar identity={m.email} size={32} withRing />
                  </View>
                ))}
                {overflow > 0 && (
                  <View
                    style={[
                      styles.memberStripAvatar,
                      {
                        width: 32,
                        height: 32,
                        borderRadius: 16,
                        backgroundColor: colors.surfaceTertiary,
                        alignItems: "center",
                        justifyContent: "center",
                        borderWidth: 2,
                        borderColor: colors.surface,
                      },
                    ]}
                  >
                    <Text style={{ fontSize: 11, fontWeight: "700", color: colors.textSecondary }}>
                      +{overflow}
                    </Text>
                  </View>
                )}
              </View>
              <View style={{ flex: 1 }}>
                <Text style={styles.memberStripText}>
                  {t("collaborators.memberCount", { count: memberCount })}
                </Text>
                {isUnlocked && memberCount > 1 && (
                  <Text style={styles.memberStripSubtext}>
                    {t("collaborators.proAppliesAll")}
                  </Text>
                )}
              </View>
              {isUnlocked && memberCount > 1 && (
                <View style={styles.memberStripBadge}>
                  <Crown size={11} color={colors.accent} />
                  <Text style={styles.memberStripBadgeText}>PRO</Text>
                </View>
              )}
              <Users size={18} color={colors.textTertiary} />
            </Pressable>
          );
        })()}

        <View style={styles.actions}>
          <Pressable
            style={styles.actionButton}
            onPress={() => router.push(`/trips/${tripId}/chat` as never)}
            accessibilityRole="button"
            accessibilityLabel={t("tripDetail.chat")}
          >
            <MessageCircle color={colors.accent} size={24} />
            <Text style={styles.actionText}>{t("tripDetail.chat")}</Text>
          </Pressable>

          <Pressable
            style={styles.actionButton}
            onPress={() => router.push(`/trips/${tripId}/bookings` as never)}
            accessibilityRole="button"
            accessibilityLabel={t("tripDetail.bookings")}
          >
            <Calendar color={colors.accent} size={24} />
            <Text style={styles.actionText}>{t("tripDetail.bookings")}</Text>
          </Pressable>

          <Pressable
            style={styles.actionButton}
            onPress={() => router.push(`/trips/${tripId}/settings` as never)}
            accessibilityRole="button"
            accessibilityLabel={t("tripDetail.settings")}
          >
            <Settings color={colors.accent} size={24} />
            <Text style={styles.actionText}>{t("tripDetail.settings")}</Text>
          </Pressable>

          <Pressable
            style={styles.actionButton}
            onPress={handleShare}
            disabled={isSharing}
            accessibilityRole="button"
            accessibilityLabel={t("referral.share")}
          >
            {isSharing ? (
              <ActivityIndicator size="small" color={colors.accent} />
            ) : (
              <Share2 color={colors.accent} size={24} />
            )}
            <Text style={styles.actionText}>{t("referral.share")}</Text>
          </Pressable>
        </View>

        {showEmptyState && (
          <View style={styles.emptyStateContainer} testID="empty-trip-cta">
            <View style={styles.emptyStateIconCircle}>
              <MessageCircle color={colors.accent} size={36} />
            </View>
            <Text style={styles.emptyStateHeadline}>
              {t("tripDetail.emptyState.headline")}
            </Text>
            <Text style={styles.emptyStateSubtitle}>
              {t("tripDetail.emptyState.subtitle")}
            </Text>
            <Pressable
              style={styles.emptyStatePrimaryButton}
              onPress={() => router.push(`/trips/${tripId}/chat` as never)}
              accessibilityRole="button"
              testID="empty-trip-start-planning"
            >
              <Send color="#fff" size={18} />
              <Text style={styles.emptyStatePrimaryButtonText}>
                {t("tripDetail.emptyState.startPlanning")}
              </Text>
            </Pressable>
            <View style={styles.suggestionChipsRow}>
              {SUGGESTION_CHIPS.map((chip) => (
                <Pressable
                  key={chip.key}
                  style={styles.suggestionChip}
                  onPress={() =>
                    router.push({
                      pathname: `/trips/${tripId}/chat` as never,
                      params: { suggestedPrompt: t(chip.labelKey) },
                    })
                  }
                  accessibilityRole="button"
                  testID={`suggestion-chip-${chip.key}`}
                >
                  <Text style={styles.suggestionChipText}>{t(chip.labelKey)}</Text>
                </Pressable>
              ))}
            </View>
          </View>
        )}

        {shareViewCount !== null && shareViewCount > 0 && (
          <View style={styles.shareStats as object}>
            <Eye color={colors.textTertiary} size={14} />
            <Text style={styles.shareStatsText}>
              {shareViewCount === 1 ? t("share.viewCountOne") : t("share.viewCount", { count: shareViewCount })}
            </Text>
          </View>
        )}

        {weather && weather.length > 0 && (
          <WeatherCard weather={weather} isClimate={isClimate} />
        )}

        {guide && (
          <View style={styles.guideCard}>
            <Text style={styles.guideHeader}>
              {t("tripDetail.travelGuide", { destination: guide.destination })}
            </Text>
            <Text style={styles.guideExcerpt} numberOfLines={3}>
              {guide.excerpt}
            </Text>
            {guide.persona_name ? (
              <Text style={styles.guideSection}>
                {t("tripDetail.guideAuthor", { name: guide.persona_name, specialty: guide.persona_specialty })}
              </Text>
            ) : null}
          </View>
        )}

        {showContinuationBanner && (
          <View style={styles.continuationBanner}>
            <View style={styles.continuationBannerContent}>
              <Text style={styles.continuationBannerText}>
                {t("tripDetail.planningBanner", { covered: coveredDays, total: totalDays })}
              </Text>
              <Pressable onPress={() => router.push(`/trips/${tripId}/chat` as never)}>
                <Text style={styles.continuationBannerCta}>{t("tripDetail.continuePlanning")}</Text>
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
                <Text style={styles.exportText}>{t("tripDetail.exportPdf")}</Text>
              </Pressable>
              <Pressable
                style={styles.exportButton}
                onPress={() => exportItineraryICal(trip, itinerary)}
              >
                <CalendarDays color={colors.accent} size={16} />
                <Text style={styles.exportText}>{t("tripDetail.exportCalendar")}</Text>
              </Pressable>
            </View>
            <ItineraryMap itinerary={itinerary} />
            <ItineraryTimeline itinerary={itinerary} />
          </>
        )}

        {hasChatVisit && !proBannerDismissed && (
          <ProUpgrade
            tripId={tripId!}
            compact
            onDismiss={handleDismissProBanner}
          />
        )}

        {isPlannable && (
          <Pressable
            style={styles.statusButton}
            onPress={() => updateTrip.mutate({ id: tripId!, status: TripStatus.ACTIVE })}
            disabled={updateTrip.isPending}
          >
            <Play color="#22c55e" size={18} />
            <Text style={styles.statusButtonText}>{t("tripDetail.startTrip")}</Text>
          </Pressable>
        )}

        {isActive && (
          <Pressable
            style={styles.statusButton}
            onPress={() => updateTrip.mutate({ id: tripId!, status: TripStatus.COMPLETED })}
            disabled={updateTrip.isPending}
          >
            <CheckCircle color="#3b82f6" size={18} />
            <Text style={styles.statusButtonText}>{t("tripDetail.completeTrip")}</Text>
          </Pressable>
        )}
      </ScrollView>
    </>
  );
}
