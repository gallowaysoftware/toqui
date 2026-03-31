import { Text, Pressable, StyleSheet, ScrollView } from "react-native";
import type { LucideIcon } from "lucide-react-native";

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
          <Icon color="#BF4028" size={14} />
          <Text style={styles.text}>{label}</Text>
        </Pressable>
      ))}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", gap: 8, paddingHorizontal: 16 },
  chip: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    backgroundColor: "#fff",
    borderWidth: 1,
    borderColor: "#BF4028",
    borderRadius: 20,
    paddingHorizontal: 14,
    paddingVertical: 8,
  },
  text: { fontSize: 13, color: "#BF4028", fontWeight: "500" },
});
