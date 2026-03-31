import { useState, useEffect, useCallback, useRef } from "react";
import {
  View,
  Text,
  StyleSheet,
  Pressable,
  Platform,
  ActivityIndicator,
  Linking,
} from "react-native";
import { useTranslation } from "react-i18next";
import { CheckCircle, Star, Mail, BookOpen, ExternalLink } from "lucide-react-native";
import { useCheckout } from "@/lib/hooks/useCheckout";

interface ProUpgradeProps {
  tripId: string;
  onUnlocked?: () => void;
}

declare global {
  interface Window {
    appendHelcimPayIframe?: (token: string) => void;
    helcimPaySuccess?: () => void;
  }
}

function loadHelcimJS(): Promise<void> {
  return new Promise((resolve, reject) => {
    if (document.getElementById("helcim-pay-js")) {
      resolve();
      return;
    }
    const script = document.createElement("script");
    script.id = "helcim-pay-js";
    script.src = "https://secure.helcim.com/helcim-pay/services/start.js";
    script.onload = () => resolve();
    script.onerror = () => reject(new Error("Failed to load Helcim.js"));
    document.head.appendChild(script);
  });
}

export function ProUpgrade({ tripId, onUnlocked }: ProUpgradeProps) {
  const { t } = useTranslation();
  const { initCheckout, validatePayment, checkStatus, isLoading, error } = useCheckout(tripId);
  const [unlocked, setUnlocked] = useState<boolean | null>(null);
  const [checkingStatus, setCheckingStatus] = useState(true);
  const statusChecked = useRef(false);

  useEffect(() => {
    if (statusChecked.current) return;
    statusChecked.current = true;
    let cancelled = false;
    checkStatus()
      .then((status) => {
        if (!cancelled) setUnlocked(status.unlocked);
      })
      .catch(() => {
        if (!cancelled) setUnlocked(false);
      })
      .finally(() => {
        if (!cancelled) setCheckingStatus(false);
      });
    return () => {
      cancelled = true;
    };
  }, [checkStatus]);

  const handleCheckout = useCallback(async () => {
    if (Platform.OS !== "web") return;

    try {
      const checkout = await initCheckout();
      await loadHelcimJS();

      window.helcimPaySuccess = async () => {
        try {
          const responseElement = document.getElementById("helcimPayJsIdentityToken") as HTMLInputElement | null;
          const hashElement = document.getElementById("helcimPayJsHash") as HTMLInputElement | null;
          const response = responseElement?.value ?? "";
          const hash = hashElement?.value ?? "";

          const result = await validatePayment(response, hash);
          if (result.unlocked) {
            setUnlocked(true);
            onUnlocked?.();
          }
        } catch {
          // useCheckout hook captures the error for display
        }
      };

      if (window.appendHelcimPayIframe) {
        window.appendHelcimPayIframe(checkout.checkoutToken);
      }
    } catch {
      // Error is already captured in the hook
    }
  }, [initCheckout, validatePayment, onUnlocked]);

  if (checkingStatus) {
    return (
      <View style={styles.container}>
        <ActivityIndicator size="small" color="#BF4028" />
      </View>
    );
  }

  if (unlocked) {
    return (
      <View style={styles.successContainer}>
        <CheckCircle color="#22c55e" size={28} />
        <Text style={styles.successTitle}>{t("checkout.success")}</Text>
        <Text style={styles.successDescription}>
          {t("checkout.successDescription")}
        </Text>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Star color="#BF4028" size={22} />
        <Text style={styles.title}>{t("checkout.title")}</Text>
      </View>

      <Text style={styles.price}>{t("checkout.price")}</Text>
      <Text style={styles.priceDescription}>
        {t("checkout.priceDescription")}
      </Text>

      <View style={styles.benefits}>
        <View style={styles.benefitRow}>
          <BookOpen color="#666" size={16} />
          <Text style={styles.benefitText}>{t("checkout.benefits.experts")}</Text>
        </View>
        <View style={styles.benefitRow}>
          <CheckCircle color="#666" size={16} />
          <Text style={styles.benefitText}>{t("checkout.benefits.bookings")}</Text>
        </View>
        <View style={styles.benefitRow}>
          <Mail color="#666" size={16} />
          <Text style={styles.benefitText}>{t("checkout.benefits.email")}</Text>
        </View>
      </View>

      {error && <Text style={styles.error}>{t("checkout.error")}</Text>}

      {Platform.OS === "web" ? (
        <Pressable
          style={[styles.unlockButton, isLoading && styles.unlockButtonDisabled]}
          onPress={handleCheckout}
          disabled={isLoading}
        >
          {isLoading ? (
            <ActivityIndicator size="small" color="#fff" />
          ) : (
            <Text style={styles.unlockButtonText}>
              {t("checkout.unlockButton")}
            </Text>
          )}
        </Pressable>
      ) : (
        <View style={styles.webOnly}>
          <Text style={styles.webOnlyText}>{t("checkout.webOnly")}</Text>
          <Pressable
            style={styles.webOnlyLink}
            onPress={() => Linking.openURL("https://toqui.app")}
          >
            <ExternalLink color="#BF4028" size={14} />
            <Text style={styles.webOnlyLinkText}>
              {t("checkout.webOnlyLink")}
            </Text>
          </Pressable>
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    backgroundColor: "#fff",
    borderRadius: 12,
    padding: 20,
    marginBottom: 20,
    borderWidth: 1,
    borderColor: "#BF4028",
  },
  header: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    marginBottom: 12,
  },
  title: {
    fontSize: 18,
    fontWeight: "700",
    color: "#333",
  },
  price: {
    fontSize: 24,
    fontWeight: "700",
    color: "#BF4028",
    marginBottom: 2,
  },
  priceDescription: {
    fontSize: 13,
    color: "#999",
    marginBottom: 16,
  },
  benefits: {
    gap: 10,
    marginBottom: 20,
  },
  benefitRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 10,
  },
  benefitText: {
    fontSize: 14,
    color: "#444",
    flex: 1,
  },
  unlockButton: {
    backgroundColor: "#BF4028",
    borderRadius: 10,
    paddingVertical: 14,
    alignItems: "center",
  },
  unlockButtonDisabled: {
    opacity: 0.6,
  },
  unlockButtonText: {
    color: "#fff",
    fontSize: 16,
    fontWeight: "600",
  },
  error: {
    color: "#ef4444",
    fontSize: 13,
    marginBottom: 12,
    textAlign: "center",
  },
  webOnly: {
    alignItems: "center",
    gap: 8,
  },
  webOnlyText: {
    fontSize: 14,
    color: "#666",
    textAlign: "center",
  },
  webOnlyLink: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
  },
  webOnlyLinkText: {
    fontSize: 14,
    color: "#BF4028",
    fontWeight: "500",
  },
  successContainer: {
    backgroundColor: "#f0fdf4",
    borderRadius: 12,
    padding: 20,
    marginBottom: 20,
    alignItems: "center",
    gap: 8,
    borderWidth: 1,
    borderColor: "#bbf7d0",
  },
  successTitle: {
    fontSize: 18,
    fontWeight: "700",
    color: "#22c55e",
  },
  successDescription: {
    fontSize: 14,
    color: "#666",
    textAlign: "center",
  },
});
