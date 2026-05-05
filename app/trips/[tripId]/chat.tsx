import {
  View,
  Text,
  FlatList,
  StyleSheet,
  KeyboardAvoidingView,
  Platform,
  ActivityIndicator,
  Pressable,
  Share,
} from "react-native";
import { alertNotice } from "@/lib/confirm";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocalSearchParams, useRouter, Stack } from "expo-router";
import { useTranslation } from "react-i18next";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { MapPin, Utensils, Compass, Briefcase, Flag } from "lucide-react-native";
import { useChat } from "@/lib/hooks/useChat";
import { useUsage, formatTimeUntilReset } from "@/lib/hooks/useUsage";
import { useTrip } from "@/lib/hooks/useTrips";
import { useOfflineTrip, useOfflineSync } from "@/lib/offline";
import { useNetworkStatus } from "@/lib/hooks/useNetworkStatus";
import { MessageBubble } from "@/components/chat/MessageBubble";
import { ChatInput } from "@/components/chat/ChatInput";
import { TypingIndicator } from "@/components/chat/TypingIndicator";
import { RecommendationCard } from "@/components/chat/RecommendationCard";
import { SuggestionChips } from "@/components/chat/SuggestionChips";
import { FollowUpSuggestions } from "@/components/chat/FollowUpSuggestions";
import { SharePromptCard } from "@/components/chat/SharePromptCard";
import { PersonaIntroCard } from "@/components/chat/PersonaIntroCard";
import { UsageIndicator } from "@/components/chat/UsageIndicator";
import { FreePlanInfoBar } from "@/components/chat/FreePlanInfoBar";
import FeedbackModal from "@/components/feedback/FeedbackModal";
import type { ChatMessage, PersonaIntroData } from "@/lib/hooks/useChat";
import { useTheme } from "@/lib/theme";
import { useAnalytics } from "@/lib/analytics";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";
import { getAutoPersona } from "@/lib/data/autoPersona";

const errorReportStyles = StyleSheet.create({
  link: {
    alignSelf: "flex-start",
    paddingHorizontal: 12,
    paddingVertical: 4,
    marginBottom: 4,
  },
  linkText: {
    fontSize: 12,
    textDecorationLine: "underline",
  },
});

const CHAT_SUGGESTION_DEFS = [
  { key: "itinerary", icon: MapPin },
  { key: "restaurants", icon: Utensils },
  { key: "hiddenGems", icon: Compass },
  { key: "packing", icon: Briefcase },
] as const;

export default function ChatScreen() {
  const { t } = useTranslation();
  const { tripId, suggestedPrompt } = useLocalSearchParams<{
    tripId: string;
    suggestedPrompt?: string;
  }>();
  const router = useRouter();
  const { colors } = useTheme();
  const { track } = useAnalytics();
  const { accessToken } = useAuth();
  const { trip } = useTrip(tripId!);
  const autoPersona = useMemo(
    () => getAutoPersona(trip?.title),
    [trip?.title],
  );
  const [showExpertBanner, setShowExpertBanner] = useState(false);
  const [feedbackOpen, setFeedbackOpen] = useState(false);
  const [showSharePrompt, setShowSharePrompt] = useState(false);
  const [isSharePromptSharing, setIsSharePromptSharing] = useState(false);
  const sharePromptCheckedRef = useRef(false);
  const [showFreePlanInfo, setShowFreePlanInfo] = useState(false);

  // Offline support
  const { isConnected } = useNetworkStatus();
  const { bundle: offlineBundle, hasCachedData } = useOfflineTrip(tripId);
  const { lastSyncedAt } = useOfflineSync(tripId);
  const isOffline = !isConnected;

  const { used, limit, resetsAt, tier } = useUsage();

  // Mark that the user has visited chat for this trip (used to gate pro upsell)
  // Also show the free plan info bar on first chat visit for free-tier users
  useEffect(() => {
    if (tripId) {
      const key = `toqui_chat_visited_${tripId}`;
      void AsyncStorage.getItem(key).then((val) => {
        if (val !== "true" && tier === "free") {
          setShowFreePlanInfo(true);
        }
        void AsyncStorage.setItem(key, "true");
      });
    }
  }, [tripId, tier]);
  const {
    messages,
    isStreaming,
    isLoadingHistory,
    isLoadingMore,
    hasMoreHistory,
    historyError,
    activePersona,
    toolActivity,
    sendMessage,
    abortStream,
    loadMoreHistory,
    retryHistory,
    lastFailedMessage,
    clearLastFailedMessage,
  } = useChat(tripId, "planning", {
    onExpertLimitReached: () => setShowExpertBanner(true),
  });

  // When offline and no network messages loaded, use cached messages from bundle
  const offlineMessages: ChatMessage[] = useMemo(() => {
    if (!isOffline || messages.length > 0 || !offlineBundle?.messages) return [];
    return offlineBundle.messages
      .filter((m) => m.role === "user" || m.role === "assistant")
      .map((m) => ({
        id: m.id,
        role: m.role as "user" | "assistant",
        content: m.content,
        personaId: m.metadata?.["persona_id"] || undefined,
        personaName: m.metadata?.["persona_name"] || undefined,
        personaAvatar: m.metadata?.["persona_avatar"] || undefined,
        personaAccentColor: m.metadata?.["persona_accent_color"] || undefined,
      }));
  }, [isOffline, messages.length, offlineBundle?.messages]);

  const displayMessages = messages.length > 0 ? messages : offlineMessages;

  const flatListRef = useRef<FlatList>(null);
  const messagesRef = useRef(displayMessages);
  messagesRef.current = displayMessages;

  // Track when the AI generates an itinerary
  useEffect(() => {
    if (toolActivity?.toolName === "create_itinerary_items" && toolActivity.status === "done") {
      track("itinerary_generated");
      // Track first-ever itinerary generation (once per user)
      void AsyncStorage.getItem("toqui_first_itinerary_tracked").then((val) => {
        if (val !== "true") {
          track("first_itinerary_generated");
          void AsyncStorage.setItem("toqui_first_itinerary_tracked", "true");
        }
      });
    }
  }, [toolActivity, track]);

  const suggestedPromptSentRef = useRef(false);
  useEffect(() => {
    if (suggestedPrompt && !suggestedPromptSentRef.current && !isLoadingHistory && !historyError && messages.length === 0) {
      suggestedPromptSentRef.current = true;
      sendMessage(suggestedPrompt);
    }
  }, [suggestedPrompt, isLoadingHistory, historyError, messages.length, sendMessage]);

  // Detect when AI finishes creating itinerary items and show share prompt (once per trip)
  const sharePromptKey = `toqui_share_prompted_${tripId}`;
  useEffect(() => {
    if (
      toolActivity?.toolName === "create_itinerary_items" &&
      toolActivity.status === "done" &&
      tripId &&
      !sharePromptCheckedRef.current
    ) {
      sharePromptCheckedRef.current = true;
      void AsyncStorage.getItem(sharePromptKey).then((val) => {
        if (val !== "true") {
          setShowSharePrompt(true);
          void AsyncStorage.setItem(sharePromptKey, "true");
        }
      });
    }
  }, [toolActivity, tripId, sharePromptKey]);

  const handleShareFromChat = useCallback(async () => {
    if (!tripId) return;
    setIsSharePromptSharing(true);
    try {
      const res = await authFetch(
        `${getConfig().apiUrl}/api/trips/share`,
        accessToken,
        { method: "POST", body: JSON.stringify({ trip_id: tripId }) },
      );
      if (!res.ok) throw new Error(`Failed to enable sharing (${res.status})`);
      const data: { share_token: string } = await res.json();
      const shareUrl = `https://app.toqui.travel/shared/${data.share_token}`;
      const shareMessage = trip?.destinationCountry
        ? `Check out my trip to ${trip.destinationCountry} on Toqui!`
        : `Check out "${trip?.title ?? "my trip"}" on Toqui!`;

      if (Platform.OS === "web" && typeof navigator !== "undefined" && navigator.share) {
        await navigator.share({ title: `${trip?.title ?? "My Trip"} — Toqui`, text: shareMessage, url: shareUrl });
      } else {
        await Share.share({ message: `${shareMessage}\n${shareUrl}`, url: shareUrl });
      }
    } catch (err) {
      if (err instanceof Error && (err.message.includes("User did not share") || err.name === "AbortError")) return;
      alertNotice({ title: t("common.error") });
    } finally {
      setIsSharePromptSharing(false);
      setShowSharePrompt(false);
    }
  }, [tripId, accessToken, trip, t]);

  const suggestions = useMemo(
    () => CHAT_SUGGESTION_DEFS.map((s) => ({ ...s, label: t(`chat.suggestions.${s.key}`) })),
    [t],
  );

  const lastAssistantMessage = useMemo(() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i].role === "assistant" && !messages[i].recommendation && !messages[i].isError) {
        return messages[i].content;
      }
    }
    return null;
  }, [messages]);

  const showFollowUps =
    !isStreaming &&
    messages.length >= 2 &&
    lastAssistantMessage !== null &&
    messages[messages.length - 1]?.role === "assistant";

  const renderMessage = useCallback(({ item, index }: { item: ChatMessage; index: number }) => {
    const prev = index > 0 ? messagesRef.current[index - 1] : null;
    const sameSpeaker =
      prev !== null &&
      prev.role === item.role &&
      prev.personaName === item.personaName &&
      !item.recommendation &&
      !prev.recommendation;
    const showAvatar = !sameSpeaker;

    if (item.recommendation) {
      return <RecommendationCard recommendation={item.recommendation} />;
    }
    if (item.isError) {
      return (
        <View>
          <MessageBubble message={item} showAvatar={showAvatar} />
          <Pressable
            onPress={() => setFeedbackOpen(true)}
            style={errorReportStyles.link}
            accessibilityRole="button"
          >
            <Text style={[errorReportStyles.linkText, { color: colors.textTertiary }]}>
              {t("chat.reportIssue")}
            </Text>
          </Pressable>
        </View>
      );
    }
    return <MessageBubble message={item} showAvatar={showAvatar} />;
  }, [colors.textTertiary]);

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surfaceSecondary },
    messageList: { padding: 16, flexGrow: 1 },
    loadingContainer: { flex: 1, justifyContent: "center", alignItems: "center", paddingTop: 100 },
    emptyContainer: { alignItems: "center", paddingTop: 100, paddingBottom: 24 },
    emptyTitle: { fontSize: 20, fontWeight: "bold", color: colors.accent, marginBottom: 8 },
    emptySubtitle: { fontSize: 14, color: colors.textSecondary, textAlign: "center", paddingHorizontal: 40, marginBottom: 20 },
    errorTitle: { fontSize: 20, fontWeight: "bold", color: colors.error, marginBottom: 8 },
    retryButton: { backgroundColor: colors.accent, borderRadius: 16, paddingVertical: 8, paddingHorizontal: 24 },
    retryButtonText: { color: "#fff", fontSize: 14, fontWeight: "600" },
    expertBanner: {
      backgroundColor: colors.warningBg,
      borderWidth: 1,
      borderColor: colors.warningBorder,
      borderRadius: 10,
      paddingVertical: 12,
      paddingHorizontal: 16,
      marginHorizontal: 16,
      marginVertical: 8,
      alignItems: "center",
    },
    expertBannerText: { fontSize: 13, color: colors.textPrimary, marginBottom: 6, textAlign: "center" },
    expertBannerRow: {
      flexDirection: "row" as const,
      alignItems: "center" as const,
      gap: 12,
    },
    expertBannerButton: {
      backgroundColor: colors.accent,
      borderRadius: 16,
      paddingVertical: 6,
      paddingHorizontal: 16,
    },
    expertBannerButtonText: { color: "#fff", fontSize: 13, fontWeight: "600" },
    expertBannerAlt: {
      paddingVertical: 6,
    },
    expertBannerAltText: { fontSize: 12, color: colors.accent, fontWeight: "500" },
    loadMoreButton: {
      alignSelf: "center",
      paddingVertical: 12,
      paddingHorizontal: 16,
    },
    loadMoreText: { fontSize: 13, color: colors.textTertiary, textAlign: "center" },
    stopButton: {
      alignSelf: "center",
      marginTop: 4,
      marginBottom: 8,
      paddingVertical: 6,
      paddingHorizontal: 16,
      borderRadius: 16,
      borderWidth: 1,
      borderColor: colors.accent,
    },
    stopButtonText: { fontSize: 13, color: colors.accent, fontWeight: "600" },
    retryBanner: {
      backgroundColor: colors.errorBg,
      borderTopWidth: 1,
      borderTopColor: colors.error,
      paddingVertical: 10,
      paddingHorizontal: 16,
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
    },
    retryBannerLeft: { flex: 1, marginRight: 8 },
    retryBannerTitle: { fontSize: 13, color: colors.error, fontWeight: "600", marginBottom: 2 },
    retryBannerPreview: { fontSize: 12, color: colors.error, opacity: 0.8 },
    retryBannerActions: { flexDirection: "row", alignItems: "center", gap: 8 },
    retryBannerRetryButton: { backgroundColor: colors.error, borderRadius: 12, paddingVertical: 5, paddingHorizontal: 12 },
    retryBannerRetryButtonText: { color: "#fff", fontSize: 13, fontWeight: "600" },
    retryDismiss: { padding: 4 },
    retryDismissText: { fontSize: 16, color: colors.error },
    headerTitle: { alignItems: "center" },
    headerTitleText: { fontSize: 16, fontWeight: "600" },
    headerSubtitle: { fontSize: 12 },
    usageBar: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
      paddingHorizontal: 16,
      paddingVertical: 6,
      borderTopWidth: 1,
      borderTopColor: colors.border,
    },
    usageText: { fontSize: 12 },
    upgradeLink: { fontSize: 12, fontWeight: "600", color: colors.accent },
    offlineInputBar: {
      paddingHorizontal: 16,
      paddingVertical: 8,
      borderTopWidth: 1,
      borderTopColor: colors.warningBorder,
      backgroundColor: colors.warningBg,
    },
    offlineInputText: {
      fontSize: 12,
      textAlign: "center",
    },
  });

  // Bottom usage bar: only show the at-limit CTA for non-free tiers
  // (free users already see the proactive UsageIndicator at the top)
  const usageAtLimit = limit > 0 && used >= limit;
  const showBottomUsageBar = usageAtLimit && tier !== "free";

  return (
    <>
    <Stack.Screen
      options={{
        headerTitle: activePersona
          ? () => (
              <View style={styles.headerTitle}>
                <Text style={[styles.headerTitleText, { color: colors.textPrimary }]}>
                  {t("chat.planYourTrip")}
                </Text>
                <Text style={[styles.headerSubtitle, { color: activePersona.accentColor || colors.textSecondary }]}>
                  {t("chat.withPersona", { name: activePersona.name })}
                </Text>
              </View>
            )
          : undefined,
        headerRight: () => (
          <Pressable
            onPress={() => setFeedbackOpen(true)}
            style={{ paddingHorizontal: 12 }}
            accessibilityLabel={t("chat.reportIssue")}
            accessibilityRole="button"
          >
            <Flag size={20} color={colors.textSecondary} />
          </Pressable>
        ),
      }}
    />
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
      keyboardVerticalOffset={90}
    >
      {/* Proactive usage counter — always visible for free users */}
      {tier === "free" && limit > 0 && (
        <UsageIndicator
          used={used}
          limit={limit}
          tier={tier}
          onUpgrade={() => tripId && router.push(`/trips/${tripId}`)}
        />
      )}
      {/* Free plan info bar — shown once on first visit to a new trip chat */}
      {showFreePlanInfo && tier === "free" && (
        <FreePlanInfoBar
          onUpgrade={() => tripId && router.push(`/trips/${tripId}`)}
        />
      )}
      <FlatList
        ref={flatListRef}
        data={displayMessages}
        renderItem={renderMessage}
        keyExtractor={(item) => item.id}
        contentContainerStyle={styles.messageList}
        onContentSizeChange={() => flatListRef.current?.scrollToEnd({ animated: true })}
        ListHeaderComponent={
          hasMoreHistory ? (
            isLoadingMore ? (
              <ActivityIndicator size="small" color={colors.accent} style={styles.loadMoreButton} />
            ) : (
              <Pressable
                onPress={loadMoreHistory}
                style={styles.loadMoreButton}
                accessibilityRole="button"
                accessibilityLabel={t("chat.loadEarlierMessages")}
              >
                <Text style={styles.loadMoreText}>{t("chat.loadEarlierMessages")}</Text>
              </Pressable>
            )
          ) : null
        }
        ListEmptyComponent={
          isLoadingHistory ? (
            <View style={styles.loadingContainer}>
              <ActivityIndicator size="large" color={colors.accent} />
            </View>
          ) : historyError && !isStreaming ? (
            <View style={styles.emptyContainer}>
              <Text style={styles.errorTitle}>{t("chat.couldNotLoadMessages")}</Text>
              <Text style={styles.emptySubtitle}>{historyError}</Text>
              <Pressable style={styles.retryButton} onPress={retryHistory}>
                <Text style={styles.retryButtonText}>{t("chat.retry")}</Text>
              </Pressable>
            </View>
          ) : (
            <View style={styles.emptyContainer}>
              <PersonaIntroCard persona={autoPersona} />
              <Text style={styles.emptySubtitle}>
                {t("chat.planYourTripSubtitle")}
              </Text>
              <SuggestionChips suggestions={suggestions} onSelect={sendMessage} />
            </View>
          )
        }
        ListFooterComponent={
          <>
            {toolActivity && (
              <TypingIndicator toolName={toolActivity.toolName} />
            )}
            {isStreaming && !toolActivity ? (
              <TypingIndicator />
            ) : null}
            {isStreaming && (
              <Pressable
                style={styles.stopButton}
                onPress={abortStream}
                accessibilityLabel={t("chat.stopGenerating")}
                accessibilityRole="button"
              >
                <Text style={styles.stopButtonText}>{t("chat.stopGenerating")}</Text>
              </Pressable>
            )}
            {showSharePrompt && !isStreaming && (
              <SharePromptCard
                onShare={handleShareFromChat}
                isSharing={isSharePromptSharing}
              />
            )}
            {showFollowUps && lastAssistantMessage && (
              <FollowUpSuggestions
                lastAssistantMessage={lastAssistantMessage}
                onSelect={(text) => sendMessage(text)}
              />
            )}
          </>
        }
      />
      {showExpertBanner && tripId && (
        <View style={styles.expertBanner}>
          <Text style={styles.expertBannerText}>{t("chat.expertLimitSoft")}</Text>
          <View style={styles.expertBannerRow}>
            <Pressable
              onPress={() => router.push(`/trips/${tripId}`)}
              style={styles.expertBannerButton}
              accessibilityRole="button"
            >
              <Text style={styles.expertBannerButtonText}>{t("chat.expertLimitCta")}</Text>
            </Pressable>
            <Pressable
              onPress={() => router.push("/(tabs)/settings" as never)}
              style={styles.expertBannerAlt}
              accessibilityRole="button"
            >
              <Text style={styles.expertBannerAltText}>{t("chat.subscriptionAlt")}</Text>
            </Pressable>
          </View>
        </View>
      )}
      {lastFailedMessage && !isStreaming && (
        <View style={styles.retryBanner}>
          <View style={styles.retryBannerLeft}>
            <Text style={styles.retryBannerTitle}>{t("chat.messageFailed")}</Text>
            <Text style={styles.retryBannerPreview} numberOfLines={1}>
              {lastFailedMessage.content.length > 60
                ? `${lastFailedMessage.content.slice(0, 60)}...`
                : lastFailedMessage.content}
            </Text>
          </View>
          <View style={styles.retryBannerActions}>
            <Pressable
              style={styles.retryBannerRetryButton}
              onPress={() => {
                void sendMessage(lastFailedMessage.content, lastFailedMessage.attachments);
              }}
              accessibilityLabel="Retry sending message"
              accessibilityRole="button"
            >
              <Text style={styles.retryBannerRetryButtonText}>{t("chat.retry")}</Text>
            </Pressable>
            <Pressable
              style={styles.retryDismiss}
              onPress={clearLastFailedMessage}
              accessibilityLabel="Dismiss retry"
              accessibilityRole="button"
            >
              <Text style={styles.retryDismissText}>×</Text>
            </Pressable>
          </View>
        </View>
      )}
      {showBottomUsageBar && (
        <View style={styles.usageBar}>
          <Text style={[styles.usageText, { color: colors.error }]}>
            {t("chat.dailyLimitReached", { time: formatTimeUntilReset(resetsAt) })}
          </Text>
          {tripId && (
            <Pressable onPress={() => router.push(`/trips/${tripId}`)}>
              <Text style={styles.upgradeLink}>{t("chat.upgrade")}</Text>
            </Pressable>
          )}
        </View>
      )}
      {isOffline && (
        <View style={styles.offlineInputBar} testID="chat-offline-indicator">
          <Text style={[styles.offlineInputText, { color: colors.warning }]}>
            You're offline — sending messages is disabled
            {lastSyncedAt ? ` (last synced: ${new Date(lastSyncedAt).toLocaleTimeString()})` : ""}
          </Text>
        </View>
      )}
      <ChatInput
        onSend={(text, attachments) => {
          sendMessage(text, attachments);
          // Track first-ever message sent (once per user)
          void AsyncStorage.getItem("toqui_first_message_tracked").then((val) => {
            if (val !== "true") {
              track("first_message_sent");
              void AsyncStorage.setItem("toqui_first_message_tracked", "true");
            }
          });
        }}
        disabled={isStreaming || isOffline}
      />
    </KeyboardAvoidingView>
    <FeedbackModal visible={feedbackOpen} onClose={() => setFeedbackOpen(false)} />
    </>
  );
}
