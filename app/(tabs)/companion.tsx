import {
  View,
  Text,
  FlatList,
  StyleSheet,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  ActivityIndicator,
} from "react-native";
import { useCallback, useMemo, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { MapPin, Utensils, Compass, Globe, Navigation } from "lucide-react-native";
import { useChat } from "@/lib/hooks/useChat";
import { useAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import { useLocation } from "@/lib/hooks/useLocation";
import { MessageBubble } from "@/components/chat/MessageBubble";
import { ChatInput } from "@/components/chat/ChatInput";
import { TypingIndicator } from "@/components/chat/TypingIndicator";
import { SuggestionChips } from "@/components/chat/SuggestionChips";
import { FollowUpSuggestions } from "@/components/chat/FollowUpSuggestions";
import { LocationPermission } from "@/components/LocationPermission";
import FeedbackModal from "@/components/feedback/FeedbackModal";
import type { ChatMessage } from "@/lib/hooks/useChat";

const COMPANION_SUGGESTION_DEFS = [
  { key: "nearby", icon: MapPin },
  { key: "eat", icon: Utensils },
  { key: "navigate", icon: Compass },
  { key: "translate", icon: Globe },
] as const;

const LOCATION_SUGGESTION_DEFS = [
  { key: "whatsNearby", icon: MapPin },
  { key: "findRestaurants", icon: Utensils },
  { key: "navigateNext", icon: Navigation },
] as const;

export default function CompanionScreen() {
  const { t } = useTranslation();
  const { accessToken, isLoading: authLoading } = useAuth();
  const { colors } = useTheme();
  const [feedbackOpen, setFeedbackOpen] = useState(false);
  const {
    messages,
    isStreaming,
    toolActivity,
    sendMessage,
    abortStream,
  } = useChat(undefined, "companion");

  const {
    location,
    isTracking,
    permissionState,
    startTracking,
    stopTracking,
  } = useLocation();

  const flatListRef = useRef<FlatList>(null);

  // Stop tracking when the user navigates away from this screen
  useEffect(() => {
    return () => {
      stopTracking();
    };
  }, [stopTracking]);

  // Wrap sendMessage to prepend location context when available
  const sendWithLocation = useCallback(
    (content: string, attachments?: { filename: string; mediaType: string; data: Uint8Array }[]) => {
      if (location) {
        const locationPrefix = `[User location: ${location.latitude.toFixed(6)}, ${location.longitude.toFixed(6)}]\n`;
        sendMessage(locationPrefix + content, attachments);
      } else {
        sendMessage(content, attachments);
      }
    },
    [location, sendMessage],
  );

  const suggestions = useMemo(() => {
    if (location) {
      return LOCATION_SUGGESTION_DEFS.map((s) => ({
        ...s,
        label: t(`companion.locationSuggestions.${s.key}`),
      }));
    }
    return COMPANION_SUGGESTION_DEFS.map((s) => ({
      ...s,
      label: t(`companion.suggestions.${s.key}`),
    }));
  }, [t, location]);

  const lastAssistantMessage = useMemo(() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i].role === "assistant" && !messages[i].isError) {
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

  const renderMessage = useCallback(({ item }: { item: ChatMessage }) => {
    return <MessageBubble message={item} />;
  }, []);

  const showPermissionBanner =
    permissionState === "prompt" && !isTracking && accessToken;

  const showDeniedBanner =
    permissionState === "denied" && !isTracking && accessToken;

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surfaceSecondary },
    center: { flex: 1, justifyContent: "center", alignItems: "center" },
    emptyText: { fontSize: 16, color: colors.textSecondary },
    messageList: { padding: 16, flexGrow: 1 },
    emptyContainer: { flex: 1, justifyContent: "center", alignItems: "center", paddingTop: 100 },
    emptyTitle: { fontSize: 20, fontWeight: "bold", color: colors.accent, marginBottom: 8 },
    emptySubtitle: { fontSize: 14, color: colors.textSecondary, textAlign: "center", paddingHorizontal: 40, marginBottom: 20 },
    feedbackLink: { marginTop: 8, paddingVertical: 4, paddingHorizontal: 8 },
    feedbackLinkText: { fontSize: 12, color: colors.textTertiary, textDecorationLine: "underline" },
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
    locationIndicator: {
      flexDirection: "row",
      alignItems: "center",
      gap: 4,
      alignSelf: "center",
      paddingVertical: 4,
      paddingHorizontal: 10,
      marginTop: 8,
      borderRadius: 12,
      backgroundColor: colors.successBg,
    },
    locationIndicatorText: {
      fontSize: 11,
      color: colors.success,
      fontWeight: "500",
    },
    deniedBanner: {
      flexDirection: "row",
      alignItems: "center",
      gap: 6,
      paddingVertical: 8,
      paddingHorizontal: 14,
      marginHorizontal: 16,
      marginTop: 8,
      borderRadius: 10,
      backgroundColor: colors.warningBg,
      borderWidth: 1,
      borderColor: colors.warningBorder,
    },
    deniedBannerText: {
      fontSize: 12,
      color: colors.warning,
      flex: 1,
    },
  });

  if (authLoading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color={colors.accent} />
      </View>
    );
  }

  if (!accessToken) {
    return (
      <>
        <View style={styles.center}>
          <Text style={styles.emptyText}>{t("companion.signInRequired")}</Text>
          <Pressable
            onPress={() => setFeedbackOpen(true)}
            style={styles.feedbackLink}
            accessibilityRole="button"
          >
            <Text style={styles.feedbackLinkText}>{t("companion.feedbackLink")}</Text>
          </Pressable>
        </View>
        <FeedbackModal visible={feedbackOpen} onClose={() => setFeedbackOpen(false)} />
      </>
    );
  }

  return (
    <>
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
      keyboardVerticalOffset={90}
    >
      {/* Location active indicator */}
      {isTracking && location && (
        <Pressable
          style={styles.locationIndicator}
          onPress={stopTracking}
          accessibilityRole="button"
          accessibilityLabel={t("companion.locationActive")}
        >
          <MapPin color={colors.success} size={12} />
          <Text style={styles.locationIndicatorText}>
            {t("companion.locationActive")}
          </Text>
        </Pressable>
      )}

      {/* Permission prompt banner */}
      {showPermissionBanner && (
        <LocationPermission onEnable={startTracking} />
      )}

      {/* Denied banner */}
      {showDeniedBanner && (
        <View style={styles.deniedBanner}>
          <MapPin color={colors.warning} size={14} />
          <Text style={styles.deniedBannerText}>
            {t("companion.locationDenied")}
          </Text>
        </View>
      )}

      <FlatList
        ref={flatListRef}
        data={messages}
        renderItem={renderMessage}
        keyExtractor={(item) => item.id}
        contentContainerStyle={styles.messageList}
        onContentSizeChange={() => flatListRef.current?.scrollToEnd({ animated: true })}
        ListEmptyComponent={
          <View style={styles.emptyContainer}>
            <Text style={styles.emptyTitle}>{t("companion.title")}</Text>
            <Text style={styles.emptySubtitle}>
              {t("companion.subtitle")}
            </Text>
            <SuggestionChips suggestions={suggestions} onSelect={sendWithLocation} />
            <Pressable
              onPress={() => setFeedbackOpen(true)}
              style={styles.feedbackLink}
              accessibilityRole="button"
            >
              <Text style={styles.feedbackLinkText}>{t("companion.feedbackLink")}</Text>
            </Pressable>
          </View>
        }
        ListFooterComponent={
          <>
            {toolActivity && <TypingIndicator toolName={toolActivity.toolName} />}
            {isStreaming && !toolActivity ? <TypingIndicator /> : null}
            {isStreaming && (
              <Pressable
                style={styles.stopButton}
                onPress={abortStream}
                accessibilityLabel={t("companion.stopGenerating")}
                accessibilityRole="button"
              >
                <Text style={styles.stopButtonText}>{t("companion.stopGenerating")}</Text>
              </Pressable>
            )}
            {showFollowUps && lastAssistantMessage && (
              <FollowUpSuggestions
                lastAssistantMessage={lastAssistantMessage}
                onSelect={(text) => sendWithLocation(text)}
                mode="companion"
                hasLocation={!!location}
              />
            )}
          </>
        }
      />
      <ChatInput onSend={sendWithLocation} disabled={isStreaming} placeholder={t("companion.placeholder")} />
    </KeyboardAvoidingView>
    <FeedbackModal visible={feedbackOpen} onClose={() => setFeedbackOpen(false)} />
    </>
  );
}
