import { View, Text, Image, StyleSheet } from "react-native";
import type { PersonaIntroData } from "@/lib/hooks/useChat";
import { useTheme } from "@/lib/theme";

interface PersonaIntroCardProps {
  persona: PersonaIntroData;
}

export function PersonaIntroCard({ persona }: PersonaIntroCardProps) {
  const { colors } = useTheme();
  const accentColor = persona.accentColor || colors.accent;

  return (
    <View style={styles.wrapper}>
      <View style={[styles.rule, { backgroundColor: accentColor }]} />
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
          <View style={[styles.avatarContainer, { borderColor: accentColor }]}>
            {persona.avatarUrl ? (
              <Image source={{ uri: persona.avatarUrl }} style={styles.avatarImage} />
            ) : (
              <View style={[styles.avatarFallback, { backgroundColor: accentColor }]}>
                <Text style={styles.avatarText}>{persona.name.charAt(0).toUpperCase()}</Text>
              </View>
            )}
          </View>
          <View style={styles.nameBlock}>
            <Text style={[styles.meetLabel, { color: colors.textSecondary }]}>
              Meet your expert
            </Text>
            <Text style={[styles.name, { color: colors.textPrimary }]}>
              {persona.name}
            </Text>
            {persona.specialties.length > 0 && (
              <Text style={[styles.specialties, { color: accentColor }]}>
                {persona.specialties.join(" \u00B7 ")}
              </Text>
            )}
          </View>
        </View>
        <Text style={[styles.handoff, { color: colors.textSecondary }]}>
          {persona.handoffMessage}
        </Text>
      </View>
      <View style={[styles.rule, { backgroundColor: accentColor }]} />
    </View>
  );
}

const styles = StyleSheet.create({
  wrapper: {
    marginVertical: 12,
  },
  rule: {
    height: 1,
    opacity: 0.25,
    marginHorizontal: 16,
  },
  card: {
    marginHorizontal: 8,
    marginVertical: 10,
    borderRadius: 16,
    borderLeftWidth: 4,
    padding: 16,
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 10,
    elevation: 4,
  },
  header: {
    flexDirection: "row",
    alignItems: "center",
    gap: 14,
    marginBottom: 12,
  },
  avatarContainer: {
    width: 56,
    height: 56,
    borderRadius: 28,
    borderWidth: 2,
    overflow: "hidden",
  },
  avatarImage: {
    width: "100%",
    height: "100%",
  },
  avatarFallback: {
    width: "100%",
    height: "100%",
    justifyContent: "center",
    alignItems: "center",
  },
  avatarText: {
    color: "#ffffff",
    fontSize: 22,
    fontWeight: "700",
  },
  nameBlock: {
    flex: 1,
  },
  meetLabel: {
    fontSize: 11,
    fontWeight: "500",
    textTransform: "uppercase",
    letterSpacing: 0.8,
    marginBottom: 2,
  },
  name: {
    fontSize: 17,
    fontWeight: "700",
    marginBottom: 2,
  },
  specialties: {
    fontSize: 13,
    fontWeight: "500",
  },
  handoff: {
    fontSize: 14,
    fontStyle: "italic",
    lineHeight: 21,
  },
});
