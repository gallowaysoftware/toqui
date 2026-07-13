import { useCallback, useState } from "react";
import {
  View,
  Text,
  TextInput,
  Pressable,
  StyleSheet,
  ActivityIndicator,
  KeyboardAvoidingView,
  Platform,
} from "react-native";
import { Redirect, useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import { Server } from "lucide-react-native";

import { useTheme } from "@/lib/theme";
import { useAuth } from "@/lib/auth";
import { getConfig, needsServerSetup, normalizeServerUrl, setServerUrl } from "@/lib/config";

/**
 * Bring-your-own-server setup (native). Shown on first run when the app
 * has no server configured (App Store / TestFlight builds don't bake one
 * in), and reachable later from Settings to switch servers.
 */
export default function ServerSetupScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { colors } = useTheme();
  const { accessToken, logout } = useAuth();

  const firstRun = needsServerSetup();
  const currentUrl = firstRun ? "" : getConfig().apiUrl;
  const [input, setInput] = useState(currentUrl);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Web gets its API URL from the serving host's /config.json — this
  // screen is a native-only concept. (After all hooks, per hooks rules.)
  const isWeb = Platform.OS === "web";

  const handleConnect = useCallback(async () => {
    const normalized = normalizeServerUrl(input);
    if (!normalized) {
      setError(t("serverSetup.invalidUrl"));
      return;
    }

    setChecking(true);
    setError(null);
    try {
      // Verify this is actually a reachable Toqui backend before saving.
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 8000);
      let ok = false;
      try {
        const res = await fetch(`${normalized}/healthz`, { signal: controller.signal });
        const body: unknown = res.ok ? await res.json() : null;
        ok = !!body && typeof body === "object" && (body as { status?: string }).status === "ok";
      } finally {
        clearTimeout(timeout);
      }
      if (!ok) {
        setError(t("serverSetup.unreachable"));
        return;
      }

      // Switching servers invalidates the current session — tokens from
      // one server mean nothing to another.
      if (!firstRun && accessToken) {
        await logout();
      }
      await setServerUrl(normalized);
      router.replace("/");
    } catch {
      setError(t("serverSetup.unreachable"));
    } finally {
      setChecking(false);
    }
  }, [input, firstRun, accessToken, logout, router, t]);

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surfaceSecondary },
    content: {
      flex: 1,
      justifyContent: "center",
      alignItems: "center",
      paddingHorizontal: 32,
    },
    iconContainer: {
      width: 88,
      height: 88,
      borderRadius: 44,
      backgroundColor: colors.accentSoft,
      justifyContent: "center",
      alignItems: "center",
      marginBottom: 24,
    },
    headline: {
      fontSize: 26,
      fontWeight: "bold",
      color: colors.textPrimary,
      textAlign: "center",
      marginBottom: 8,
    },
    subtitle: {
      fontSize: 15,
      color: colors.textSecondary,
      textAlign: "center",
      lineHeight: 22,
      maxWidth: 340,
      marginBottom: 28,
    },
    input: {
      backgroundColor: colors.inputBg,
      borderWidth: 1,
      borderColor: error ? colors.error : colors.inputBorder,
      borderRadius: 12,
      padding: 16,
      fontSize: 16,
      color: colors.textPrimary,
      width: "100%",
      maxWidth: 380,
      marginBottom: 12,
    },
    errorText: {
      color: colors.error,
      fontSize: 14,
      marginBottom: 12,
      maxWidth: 380,
      textAlign: "center",
    },
    switchWarning: {
      color: colors.textSecondary,
      fontSize: 13,
      marginBottom: 12,
      maxWidth: 380,
      textAlign: "center",
    },
    button: {
      backgroundColor: colors.accent,
      borderRadius: 12,
      paddingVertical: 16,
      paddingHorizontal: 32,
      width: "100%",
      maxWidth: 380,
      alignItems: "center",
      marginBottom: 12,
    },
    buttonDisabled: { opacity: 0.5 },
    buttonText: { color: "#fff", fontSize: 16, fontWeight: "600" },
    cancelButton: { paddingVertical: 12 },
    cancelText: { color: colors.textSecondary, fontSize: 15 },
  });

  if (isWeb) {
    return <Redirect href="/" />;
  }

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : undefined}
    >
      <View style={styles.content}>
        <View style={styles.iconContainer}>
          <Server color={colors.accent} size={40} />
        </View>
        <Text style={styles.headline} testID="server-setup-headline">
          {t("serverSetup.headline")}
        </Text>
        <Text style={styles.subtitle}>{t("serverSetup.subtitle")}</Text>

        <TextInput
          style={styles.input}
          value={input}
          onChangeText={(v) => {
            setInput(v);
            setError(null);
          }}
          placeholder="https://toqui.example.com"
          placeholderTextColor={colors.textTertiary}
          autoCapitalize="none"
          autoCorrect={false}
          keyboardType="url"
          testID="server-setup-input"
          editable={!checking}
        />

        {error && (
          <Text style={styles.errorText} testID="server-setup-error">
            {error}
          </Text>
        )}
        {!firstRun && accessToken ? (
          <Text style={styles.switchWarning}>{t("serverSetup.switchWarning")}</Text>
        ) : null}

        <Pressable
          style={[styles.button, (checking || !input.trim()) && styles.buttonDisabled]}
          onPress={handleConnect}
          disabled={checking || !input.trim()}
          accessibilityRole="button"
          accessibilityLabel={t("serverSetup.connect")}
          testID="server-setup-connect"
        >
          {checking ? (
            <ActivityIndicator size="small" color="#fff" />
          ) : (
            <Text style={styles.buttonText}>{t("serverSetup.connect")}</Text>
          )}
        </Pressable>

        {!firstRun && (
          <Pressable
            style={styles.cancelButton}
            onPress={() => router.back()}
            accessibilityRole="button"
            testID="server-setup-cancel"
          >
            <Text style={styles.cancelText}>{t("common.cancel")}</Text>
          </Pressable>
        )}
      </View>
    </KeyboardAvoidingView>
  );
}
