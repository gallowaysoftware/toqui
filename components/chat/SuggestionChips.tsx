import { Text, Pressable, StyleSheet, ScrollView } from "react-native";
import type { LucideIcon } from "lucide-react-native";
import { useTheme } from "@/lib/theme";

interface Suggestion {
  key: string;
  icon: LucideIcon;
  label: string;
}

interface SuggestionChipsProps {
  suggestions: Suggestion[];
  onSelect: (label: string) => void;
}

export function SuggestionChips({ suggestions, onSelect }: SuggestionChipsProps) {
  const { colors } = useTheme();

  const styles = StyleSheet.create({
    row: { flexDirection: "row", gap: 8, paddingHorizontal: 16 },
    chip: {
      flexDirection: "row",
      alignItems: "center",
      gap: 6,
      backgroundColor: colors.surface,
      borderWidth: 1,
      borderColor: colors.accent,
      borderRadius: 20,
      paddingHorizontal: 14,
      paddingVertical: 8,
    },
    text: { fontSize: 13, color: colors.accent, fontWeight: "500" },
    chipFocused: {
      outlineWidth: 2,
      outlineColor: colors.accent,
      outlineStyle: "solid",
    },
  });

  return (
    <ScrollView
      horizontal
      showsHorizontalScrollIndicator={false}
      contentContainerStyle={styles.row}
    >
      {suggestions.map(({ key, icon: Icon, label }) => (
        <Pressable
          key={key}
          style={styles.chip}
          onPress={() => onSelect(label)}
        >
          <Icon color={colors.accent} size={14} />
          <Text style={styles.text}>{label}</Text>
        </Pressable>
      ))}
    </ScrollView>
  );
}
