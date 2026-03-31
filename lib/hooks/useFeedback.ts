import { useState, useCallback } from "react";
import { Platform } from "react-native";
import Constants from "expo-constants";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";
import { useTheme } from "@/lib/theme";

export type FeedbackType = "bug" | "feature" | "general" | "chat_quality";

interface FeedbackContext {
  platform: string;
  appVersion: string;
  screen: string;
  theme: string;
  tripId?: string;
  userAgent?: string;
}

export function useFeedback() {
  const { accessToken } = useAuth();
  const { mode } = useTheme();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isSuccess, setIsSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = useCallback(
    async (
      type: FeedbackType,
      message: string,
      screen: string,
      tripId?: string,
    ) => {
      setIsSubmitting(true);
      setError(null);
      setIsSuccess(false);

      const context: FeedbackContext = {
        platform: Platform.OS,
        appVersion:
          Constants.expoConfig?.version ?? "unknown",
        screen,
        theme: mode,
        tripId,
      };

      if (Platform.OS === "web" && typeof navigator !== "undefined") {
        context.userAgent = navigator.userAgent;
      }

      try {
        const res = await authFetch(
          `${getConfig().apiUrl}/api/feedback`,
          accessToken,
          {
            method: "POST",
            body: JSON.stringify({ type, message, context }),
          },
        );
        if (!res.ok) {
          throw new Error(`Feedback submission failed: ${res.status}`);
        }
        setIsSuccess(true);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Failed to send feedback",
        );
      } finally {
        setIsSubmitting(false);
      }
    },
    [accessToken, mode],
  );

  const reset = useCallback(() => {
    setIsSuccess(false);
    setError(null);
  }, []);

  return { submit, isSubmitting, isSuccess, error, reset };
}
