import { View, Text, StyleSheet, ActivityIndicator } from "react-native";
import { useEffect } from "react";
import * as WebBrowser from "expo-web-browser";

// This page handles the OAuth redirect. When Google redirects back here
// with ?code=..., maybeCompleteAuthSession() detects the params and
// resolves the promptAsync() promise in the parent window (popup flow)
// or the same window (redirect flow).
WebBrowser.maybeCompleteAuthSession();

export default function AuthCallbackScreen() {
  useEffect(() => {
    // If we're here in a popup, maybeCompleteAuthSession above will close it.
    // If we're here via full redirect (e.g. mobile browser), we need to
    // wait for the auth to complete, then the app will navigate away.
  }, []);

  return (
    <View style={styles.container}>
      <ActivityIndicator size="large" color="#BF4028" />
      <Text style={styles.text}>Signing you in...</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: "center", alignItems: "center", gap: 16 },
  text: { fontSize: 16, color: "#666" },
});
