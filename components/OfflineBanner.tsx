import { useEffect, useRef, useState } from "react";
import { Animated, StyleSheet, Text, View, Pressable } from "react-native";
import { WifiOff, X } from "lucide-react-native";
import { useTheme } from "@/lib/theme";
import { useNetworkStatus } from "@/lib/hooks/useNetworkStatus";

const BANNER_HEIGHT = 44;

export function OfflineBanner() {
  const { isConnected } = useNetworkStatus();
  const { isDark } = useTheme();
  const heightAnim = useRef(new Animated.Value(0)).current;
  const [showBanner, setShowBanner] = useState(false);
  const dismissed = useRef(false);

  useEffect(() => {
    const shouldShow = !isConnected && !dismissed.current;

    if (shouldShow && !showBanner) {
      setShowBanner(true);
      Animated.timing(heightAnim, {
        toValue: BANNER_HEIGHT,
        duration: 300,
        useNativeDriver: false,
      }).start();
    } else if (!shouldShow && showBanner) {
      Animated.timing(heightAnim, {
        toValue: 0,
        duration: 300,
        useNativeDriver: false,
      }).start(() => setShowBanner(false));
    }
  }, [isConnected, showBanner, heightAnim]);

  // Reset dismissed state when connectivity is restored
  useEffect(() => {
    if (isConnected) {
      dismissed.current = false;
    }
  }, [isConnected]);

  const handleDismiss = () => {
    dismissed.current = true;
    Animated.timing(heightAnim, {
      toValue: 0,
      duration: 200,
      useNativeDriver: false,
    }).start(() => setShowBanner(false));
  };

  if (!showBanner) return null;

  const bgColor = isDark ? "#78350f" : "#fbbf24";
  const textColor = isDark ? "#fef3c7" : "#78350f";
  const iconColor = textColor;

  return (
    <Animated.View
      style={[
        styles.container,
        { backgroundColor: bgColor, height: heightAnim },
      ]}
      accessibilityRole="alert"
      accessibilityLabel="You are offline. Some features may not work."
      testID="offline-banner"
    >
      <View style={styles.content}>
        <WifiOff size={16} color={iconColor} />
        <Text style={[styles.text, { color: textColor }]}>
          You're offline — some features may not work
        </Text>
      </View>
      <Pressable
        onPress={handleDismiss}
        hitSlop={8}
        accessibilityLabel="Dismiss offline banner"
        testID="offline-banner-dismiss"
      >
        <X size={16} color={iconColor} />
      </Pressable>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  container: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingHorizontal: 16,
    overflow: "hidden",
  },
  content: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    flex: 1,
  },
  text: {
    fontSize: 14,
    fontWeight: "600",
  },
});
