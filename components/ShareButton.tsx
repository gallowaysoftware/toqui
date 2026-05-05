import { useState } from "react";
import {
  Pressable,
  Text,
  StyleSheet,
  Share,
  Platform,
  ActivityIndicator,
} from "react-native";
import { alertNotice } from "@/lib/confirm";
import { Share2 } from "lucide-react-native";
import { useTheme } from "@/lib/theme";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";
import { useAnalytics } from "@/lib/analytics";

interface ShareButtonProps {
  /** Trip ID to enable sharing for */
  tripId: string;
  /** Trip title, used in the share message */
  tripTitle: string;
  /** Optional destination for richer share text */
  destination?: string;
  /** Button label text */
  label?: string;
}

/**
 * ShareButton — enables trip sharing via the backend API and opens the
 * platform share sheet (Web Share API on web, native share on iOS/Android).
 *
 * The share message includes the trip title and a link to the shared trip page.
 * On web, falls back to clipboard copy if the Web Share API is not available.
 */
export function ShareButton({
  tripId,
  tripTitle,
  destination,
  label = "Share",
}: ShareButtonProps) {
  const [isSharing, setIsSharing] = useState(false);
  const { accessToken } = useAuth();
  const { colors } = useTheme();
  const { track } = useAnalytics();

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

      const shareMessage = destination
        ? `Check out my trip to ${destination} on Toqui!`
        : `Check out "${tripTitle}" on Toqui!`;

      if (Platform.OS === "web" && typeof navigator !== "undefined" && navigator.share) {
        await navigator.share({
          title: `${tripTitle} — Toqui`,
          text: shareMessage,
          url: shareUrl,
        });
      } else {
        await Share.share({
          message: `${shareMessage}\n${shareUrl}`,
          url: shareUrl,
        });
      }
      track("trip_shared", { platform: Platform.OS });
    } catch (err) {
      // User cancelled the share sheet — not an error
      if (err instanceof Error && err.message.includes("User did not share")) {
        return;
      }
      // Web Share API abort is not an error
      if (err instanceof Error && err.name === "AbortError") {
        return;
      }
      alertNotice({ title: "Error", message: "Could not share this trip. Please try again." });
    } finally {
      setIsSharing(false);
    }
  };

  const styles = StyleSheet.create({
    button: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "center",
      gap: 8,
      backgroundColor: colors.surface,
      borderRadius: 12,
      padding: 16,
      borderWidth: 1,
      borderColor: colors.border,
    },
    label: {
      fontSize: 14,
      fontWeight: "500",
      color: colors.textPrimary,
    },
  });

  return (
    <Pressable
      style={({ pressed }) => [styles.button, { opacity: pressed ? 0.8 : 1 }]}
      onPress={handleShare}
      disabled={isSharing}
      accessibilityRole="button"
      accessibilityLabel={`Share ${tripTitle}`}
    >
      {isSharing ? (
        <ActivityIndicator size="small" color={colors.accent} />
      ) : (
        <Share2 color={colors.accent} size={24} />
      )}
      <Text style={styles.label}>{label}</Text>
    </Pressable>
  );
}
