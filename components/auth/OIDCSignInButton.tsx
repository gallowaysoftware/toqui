import { Pressable, StyleSheet, Text, View } from "react-native";
import { useTranslation } from "react-i18next";
import { useTheme } from "@/lib/theme";
import { useOIDCAuth } from "@/lib/oidc-auth";

/**
 * "Sign in with <provider>" button for the operator's configured OIDC/SSO
 * provider (Authelia, Authentik, Keycloak, ...). Self-contained — it owns the
 * useOIDCAuth hook — so the parent can mount it conditionally on
 * `authProviders?.oidc?.enabled` and the discovery request never fires when
 * SSO is disabled.
 */
/**
 * @param showSeparator render the "or" divider above the button. Pass false
 * when another alternative (e.g. the Google button) already shows one, to
 * avoid a redundant second divider.
 */
export function OIDCSignInButton({ showSeparator = true }: { showSeparator?: boolean }) {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const { signIn, isReady, name } = useOIDCAuth();
  const styles = createStyles(colors);

  return (
    <>
      {showSeparator ? (
        <View style={styles.separator}>
          <View style={styles.separatorLine} />
          <Text style={styles.separatorText}>{t("common.or")}</Text>
          <View style={styles.separatorLine} />
        </View>
      ) : null}
      <Pressable
        style={[styles.button, !isReady && styles.disabled]}
        onPress={signIn}
        disabled={!isReady}
        accessibilityRole="button"
        testID="signin-oidc"
      >
        <Text style={styles.buttonText}>{t("common.signInWithSSO", { provider: name })}</Text>
      </Pressable>
    </>
  );
}

function createStyles(colors: ReturnType<typeof useTheme>["colors"]) {
  return StyleSheet.create({
    separator: {
      flexDirection: "row",
      alignItems: "center",
      width: "100%",
      maxWidth: 320,
      marginVertical: 14,
      gap: 8,
    },
    separatorLine: { flex: 1, height: 1, backgroundColor: colors.border },
    separatorText: { color: colors.textTertiary, fontSize: 12, textTransform: "uppercase" },
    button: {
      backgroundColor: colors.surface,
      borderRadius: 8,
      paddingVertical: 14,
      paddingHorizontal: 24,
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "center",
      gap: 8,
      borderWidth: 1,
      borderColor: colors.borderStrong,
      width: "100%",
      maxWidth: 320,
    },
    buttonText: { color: colors.textPrimary, fontSize: 16, fontWeight: "600" },
    disabled: { opacity: 0.5 },
  });
}
