import { useEffect, useRef } from "react";
import { Animated, StyleSheet, Text, View, Pressable } from "react-native";
import { WifiOff, X } from "lucide-react-native";
import { useTheme } from "@/lib/theme";
import { useNetworkStatus } from "@/lib/hooks/useNetworkStatus";

const BANNER_HEIGHT = 44;

export function OfflineBanner() {
  const { isConnected } = useNetworkStatus();
  const { isDark } = useTheme();
  const translateY = useRef(new Animated.Value(-BANNER_HEIGHT)).current;
  const visible = useRef(false);

  useEffect(() => {
    const shouldShow = !isConnected;

    if (shouldShow && !visible.current) {
      visible.current = true;
      Animated.timing(translateY, {
        toValue: 0,
        duration: 300,
        useNativeDriver: true,
      }).start();
    } else if (!shouldShow && visible.current) {
      visible.current = false;
      Animated.timing(translateY, {
        toValue: -BANNER_HEIGHT,
        duration: 300,
        useNativeDriver: true,
      }).start();
    }
  }, [isConnected, translateY]);

  const handleDismiss = () => {
    visible.current = false;
    Animated.timing(translateY, {
      toValue: -BANNER_HEIGHT,
      duration: 200,
      useNativeDriver: true,
    }).start();
  };

  const bgColor = isDark ? "#78350f" : "#fbbf24";
  const textColor = isDark ? "#fef3c7" : "#78350f";
  const iconColor = textColor;

  return (
    <Animated.View
      style={[
        styles.container,
        { backgroundColor: bgColor, transform: [{ translateY }] },
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
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    height: BANNER_HEIGHT,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingHorizontal: 16,
    zIndex: 1000,
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
