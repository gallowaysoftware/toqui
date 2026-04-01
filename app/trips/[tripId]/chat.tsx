import {
  View,
  Text,
  FlatList,
  StyleSheet,
  KeyboardAvoidingView,
  Platform,
  ActivityIndicator,
  Pressable,
} from "react-native";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocalSearchParams, useRouter, Stack } from "expo-router";
import { useTranslation } from "react-i18next";
import { MapPin, Utensils, Compass, Briefcase, Flag } from "lucide-react-native";
import { useChat } from "@/lib/hooks/useChat";
import { useUsage, formatTimeUntilReset } from "@/lib/hooks/useUsage";
import { MessageBubble } from "@/components/chat/MessageBubble";
import { ChatInput } from "@/components/chat/ChatInput";
import { TypingIndicator } from "@/components/chat/TypingIndicator";
import { RecommendationCard } from "@/components/chat/RecommendationCard";
import { SuggestionChips } from "@/components/chat/SuggestionChips";
import { FollowUpSuggestions } from "@/components/chat/FollowUpSuggestions";
import FeedbackModal from "@/components/feedback/FeedbackModal";
import type { ChatMessage } from "@/lib/hooks/useChat";
import { useTheme } from "@/lib/theme";

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
  const [showExpertBanner, setShowExpertBanner] = useState(false);
  const [feedbackOpen, setFeedbackOpen] = useState(false);
  const { used, limit, resetsAt } = useUsage();
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

  const flatListRef = useRef<FlatList>(null);
  const messagesRef = useRef(messages);
  messagesRef.current = messages;

  const suggestedPromptSentRef = useRef(false);
  useEffect(() => {
    if (suggestedPrompt && !suggestedPromptSentRef.current && !isLoadingHistory && messages.length === 0) {
      suggestedPromptSentRef.current = true;
      sendMessage(suggestedPrompt);
    }
  }, [suggestedPrompt, isLoadingHistory, messages.length, sendMessage]);

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
      backgroundColor: colors.accentSoft,
      borderTopWidth: 1,
      borderTopColor: colors.accent,
      paddingVertical: 10,
      paddingHorizontal: 16,
      alignItems: "center",
    },
    expertBannerText: { fontSize: 13, color: colors.textPrimary, marginBottom: 6, textAlign: "center" },
    expertBannerButton: {
      backgroundColor: colors.accent,
      borderRadius: 16,
      paddingVertical: 6,
      paddingHorizontal: 16,
    },
    expertBannerButtonText: { color: "#fff", fontSize: 13, fontWeight: "600" },
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
  });

  const usageVisible = limit > 0 && used >= limit * 0.75;
  const usageAtLimit = limit > 0 && used >= limit;
  const usageNearLimit = limit > 0 && used >= limit - 1 && !usageAtLimit;
  const usageTextColor = usageAtLimit
    ? colors.error
    : usageNearLimit
      ? colors.textSecondary
      : colors.textTertiary;
  const usageLabel = usageAtLimit
    ? t("chat.dailyLimitReached", { time: formatTimeUntilReset(resetsAt) })
    : usageNearLimit
      ? t("chat.almostAtDailyLimit", { remaining: limit - used })
      : t("chat.messagesUsage", { used, limit });

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
      <FlatList
        ref={flatListRef}
        data={messages}
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
              <Text style={styles.emptyTitle}>{t("chat.planYourTrip")}</Text>
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
          <Text style={styles.expertBannerText}>{t("chat.expertLimitReached")}</Text>
          <Pressable
            onPress={() => router.push(`/trips/${tripId}`)}
            style={styles.expertBannerButton}
          >
            <Text style={styles.expertBannerButtonText}>{t("chat.expertLimitCta")}</Text>
          </Pressable>
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
      {usageVisible && (
        <View style={styles.usageBar}>
          <Text style={[styles.usageText, { color: usageTextColor }]}>{usageLabel}</Text>
          {usageAtLimit && tripId && (
            <Pressable onPress={() => router.push(`/trips/${tripId}`)}>
              <Text style={styles.upgradeLink}>{t("chat.upgrade")}</Text>
            </Pressable>
          )}
        </View>
      )}
      <ChatInput
        onSend={(text, attachments) => sendMessage(text, attachments)}
        disabled={isStreaming}
      />
    </KeyboardAvoidingView>
    <FeedbackModal visible={feedbackOpen} onClose={() => setFeedbackOpen(false)} />
    </>
  );
}
