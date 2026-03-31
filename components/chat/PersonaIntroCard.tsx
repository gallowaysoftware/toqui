import { View, Text, StyleSheet } from "react-native";
import type { PersonaIntroData } from "@/lib/hooks/useChat";
import { useTheme } from "@/lib/theme";

interface PersonaIntroCardProps {
  persona: PersonaIntroData;
}

export function PersonaIntroCard({ persona }: PersonaIntroCardProps) {
  const { colors } = useTheme();
  const accentColor = persona.accentColor || colors.accent;
  const initial = persona.name.charAt(0).toUpperCase();

  return (
    <View
      style={[
        styles.card,
        {
          backgroundColor: colors.surface,
          borderColor: accentColor,
          shadowColor: accentColor,
        },
      ]}
    >
      <View style={styles.header}>
        <View style={[styles.avatar, { backgroundColor: accentColor }]}>
          <Text style={styles.avatarText}>{initial}</Text>
        </View>
        <View style={styles.nameBlock}>
          <Text style={[styles.name, { color: colors.textPrimary }]}>
            {persona.name}
          </Text>
          {persona.specialties.length > 0 && (
            <Text style={[styles.specialties, { color: colors.textSecondary }]}>
              {persona.specialties.join(" \u00B7 ")}
            </Text>
          )}
        </View>
      </View>
      <Text style={[styles.handoff, { color: colors.textTertiary }]}>
        {persona.handoffMessage}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    alignSelf: "center",
    maxWidth: "90%",
    borderRadius: 16,
    borderLeftWidth: 3,
    padding: 16,
    marginBottom: 8,
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.08,
    shadowRadius: 8,
    elevation: 3,
  },
  header: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    marginBottom: 10,
  },
  avatar: {
    width: 40,
    height: 40,
    borderRadius: 20,
    justifyContent: "center",
    alignItems: "center",
  },
  avatarText: {
    color: "#ffffff",
    fontSize: 18,
    fontWeight: "700",
  },
  nameBlock: {
    flex: 1,
  },
  name: {
    fontSize: 16,
    fontWeight: "700",
  },
  specialties: {
    fontSize: 13,
    marginTop: 2,
  },
  handoff: {
    fontSize: 14,
    fontStyle: "italic",
    lineHeight: 20,
  },
});
