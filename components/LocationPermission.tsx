import { View, Text, Pressable, StyleSheet } from "react-native";
import { useState, useEffect } from "react";
import { MapPin, X } from "lucide-react-native";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { useTheme } from "@/lib/theme";

const DISMISSED_KEY = "location_permission_dismissed_at";
const DISMISS_DURATION_MS = 24 * 60 * 60 * 1000; // 24 hours

interface LocationPermissionProps {
  /** Called when the user taps "Enable" */
  onEnable: () => void;
}

/**
 * A non-intrusive banner prompting the user to enable location access.
 * Dismisses for 24 hours when the user taps "Not now".
 */
export function LocationPermission({ onEnable }: LocationPermissionProps) {
  const { colors } = useTheme();
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    AsyncStorage.getItem(DISMISSED_KEY)
      .then((val) => {
        if (!val) {
          setVisible(true);
          return;
        }
        const dismissedAt = parseInt(val, 10);
        if (Date.now() - dismissedAt > DISMISS_DURATION_MS) {
          setVisible(true);
        }
      })
      .catch(() => {
        setVisible(true);
      });
  }, []);

  const handleDismiss = () => {
    setVisible(false);
    AsyncStorage.setItem(DISMISSED_KEY, String(Date.now())).catch(() => {});
  };

  if (!visible) return null;

  const styles = StyleSheet.create({
    container: {
      flexDirection: "row",
      alignItems: "center",
      backgroundColor: colors.infoBg,
      borderWidth: 1,
      borderColor: colors.infoBorder,
      borderRadius: 12,
      paddingVertical: 10,
      paddingHorizontal: 14,
      marginHorizontal: 16,
      marginTop: 8,
      gap: 10,
    },
    icon: {
      width: 28,
      height: 28,
      borderRadius: 14,
      backgroundColor: colors.accentSoft,
      alignItems: "center",
      justifyContent: "center",
    },
    textContainer: {
      flex: 1,
    },
    title: {
      fontSize: 13,
      fontWeight: "600",
      color: colors.textPrimary,
      marginBottom: 2,
    },
    description: {
      fontSize: 12,
      color: colors.textSecondary,
      lineHeight: 16,
    },
    enableButton: {
      backgroundColor: colors.accent,
      borderRadius: 8,
      paddingVertical: 6,
      paddingHorizontal: 12,
    },
    enableText: {
      fontSize: 12,
      fontWeight: "600",
      color: "#fff",
    },
    dismissButton: {
      padding: 4,
    },
  });

  return (
    <View style={styles.container} accessibilityRole="alert">
      <View style={styles.icon}>
        <MapPin color={colors.accent} size={16} />
      </View>
      <View style={styles.textContainer}>
        <Text style={styles.title}>Enable location</Text>
        <Text style={styles.description}>
          Share your location to get personalized nearby recommendations
        </Text>
      </View>
      <Pressable
        onPress={onEnable}
        style={styles.enableButton}
        accessibilityRole="button"
        accessibilityLabel="Enable location"
      >
        <Text style={styles.enableText}>Enable</Text>
      </Pressable>
      <Pressable
        onPress={handleDismiss}
        style={styles.dismissButton}
        accessibilityRole="button"
        accessibilityLabel="Not now"
      >
        <X color={colors.textTertiary} size={16} />
      </Pressable>
    </View>
  );
}
