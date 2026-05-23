import { useState, useCallback } from "react";
import {
  View,
  Text,
  TextInput,
  Pressable,
  StyleSheet,
  KeyboardAvoidingView,
  Platform,
  ScrollView,
  ActivityIndicator,
} from "react-native";
import { useRouter, Link } from "expo-router";
import { useTranslation } from "react-i18next";
import { Code, ConnectError } from "@connectrpc/connect";
import { useAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";

export default function EmailLoginScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { colors } = useTheme();
  const { loginWithEmail } = useAuth();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = useCallback(async () => {
    setError(null);
    const trimmed = email.trim();
    if (!trimmed || !password) {
      setError(t("auth.login.errors.missingFields"));
      return;
    }
    setSubmitting(true);
    try {
      await loginWithEmail(trimmed, password);
      router.replace("/(tabs)" as never);
    } catch (err) {
      // Map ConnectRPC error codes to friendly messages. Anything we don't
      // recognise falls back to the generic copy — we don't surface raw
      // server messages to the user.
      if (err instanceof ConnectError && err.code === Code.Unauthenticated) {
        setError(t("auth.login.errors.invalidCredentials"));
      } else if (
        err instanceof ConnectError &&
        err.code === Code.InvalidArgument
      ) {
        setError(t("auth.login.errors.invalidCredentials"));
      } else {
        setError(t("auth.login.errors.generic"));
      }
    } finally {
      setSubmitting(false);
    }
  }, [email, password, loginWithEmail, router, t]);

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surfaceSecondary },
    scroll: { flexGrow: 1, justifyContent: "center", padding: 24 },
    title: { fontSize: 28, fontWeight: "700", color: colors.textPrimary, marginBottom: 6 },
    subtitle: { fontSize: 15, color: colors.textSecondary, marginBottom: 24 },
    label: { fontSize: 14, fontWeight: "600", color: colors.textPrimary, marginBottom: 6 },
    input: {
      borderWidth: 1,
      borderColor: colors.inputBorder,
      backgroundColor: colors.inputBg,
      color: colors.textPrimary,
      borderRadius: 8,
      paddingHorizontal: 12,
      paddingVertical: 12,
      fontSize: 16,
      marginBottom: 16,
    },
    error: {
      color: colors.error,
      fontSize: 14,
      marginBottom: 12,
    },
    primaryButton: {
      backgroundColor: colors.accent,
      borderRadius: 8,
      paddingVertical: 14,
      paddingHorizontal: 24,
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "center",
      gap: 8,
    },
    disabledButton: { opacity: 0.5 },
    buttonText: { color: "#fff", fontSize: 16, fontWeight: "600" },
    switchRow: {
      flexDirection: "row",
      justifyContent: "center",
      alignItems: "center",
      marginTop: 20,
      gap: 6,
    },
    switchText: { color: colors.textSecondary, fontSize: 14 },
    switchLink: { color: colors.accent, fontSize: 14, fontWeight: "600" },
  });

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : undefined}
    >
      <ScrollView
        contentContainerStyle={styles.scroll}
        keyboardShouldPersistTaps="handled"
      >
        <Text style={styles.title} testID="email-login-title">
          {t("auth.login.title")}
        </Text>
        <Text style={styles.subtitle}>{t("auth.login.subtitle")}</Text>

        <Text style={styles.label}>{t("auth.login.emailLabel")}</Text>
        <TextInput
          testID="email-login-email"
          style={styles.input}
          value={email}
          onChangeText={setEmail}
          placeholder={t("auth.login.emailPlaceholder")}
          placeholderTextColor={colors.textTertiary}
          autoCapitalize="none"
          autoCorrect={false}
          autoComplete="email"
          keyboardType="email-address"
          textContentType="emailAddress"
          editable={!submitting}
        />

        <Text style={styles.label}>{t("auth.login.passwordLabel")}</Text>
        <TextInput
          testID="email-login-password"
          style={styles.input}
          value={password}
          onChangeText={setPassword}
          placeholder={t("auth.login.passwordPlaceholder")}
          placeholderTextColor={colors.textTertiary}
          secureTextEntry
          autoComplete="password"
          textContentType="password"
          editable={!submitting}
        />

        {error ? (
          <Text
            style={styles.error}
            testID="email-login-error"
            accessibilityLiveRegion="assertive"
          >
            {error}
          </Text>
        ) : null}

        <Pressable
          testID="email-login-submit"
          style={[styles.primaryButton, submitting && styles.disabledButton]}
          onPress={handleSubmit}
          disabled={submitting}
          accessibilityRole="button"
        >
          {submitting ? <ActivityIndicator color="#fff" size="small" /> : null}
          <Text style={styles.buttonText}>
            {submitting ? t("auth.login.submitting") : t("auth.login.submit")}
          </Text>
        </Pressable>

        <View style={styles.switchRow}>
          <Text style={styles.switchText}>{t("auth.login.noAccount")}</Text>
          <Link href="/auth/email-register" replace testID="email-login-register-link">
            <Text style={styles.switchLink}>{t("auth.login.registerLink")}</Text>
          </Link>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
