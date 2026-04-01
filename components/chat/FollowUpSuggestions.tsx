import { useEffect, useRef } from "react";
import { View, Text, Pressable, StyleSheet, Animated } from "react-native";
import { useTranslation } from "react-i18next";
import { useTheme } from "@/lib/theme";

export interface TripContext {
  destination?: string;
  hasItinerary?: boolean;
  hasBookings?: boolean;
}

export interface FollowUpSuggestionsProps {
  lastAssistantMessage: string;
  tripContext?: TripContext;
  onSelect: (suggestion: string) => void;
  mode?: "planning" | "companion";
  hasLocation?: boolean;
}

interface SuggestionRule {
  test: (message: string, context?: TripContext) => boolean;
  keys: string[];
}

const RULES: SuggestionRule[] = [
  {
    test: (msg) =>
      /\b(food|restaurants?|dining|cuisine|eat|meals?|dishes?|chef|bistro|cafe)\b/i.test(msg),
    keys: ["localDishes", "reservationTips"],
  },
  {
    test: (msg) =>
      /\b(attraction|museum|monument|landmark|temple|church|palace|castle|park|garden|gallery|ruins|site)\b/i.test(msg),
    keys: ["howLongToSpend", "whatsNearby"],
  },
  {
    test: (msg) =>
      /\b(itinerary|day\s*\d|schedule|planned|morning|afternoon|evening|agenda)\b/i.test(msg),
    keys: ["addMoreActivities", "adjustPace", "alternativeOptions"],
  },
  {
    test: (_msg, ctx) => ctx?.hasBookings === false,
    keys: ["findFlights", "recommendHotels"],
  },
  {
    test: (_msg, ctx) => ctx?.hasItinerary === false,
    keys: ["createDayByDay", "mustSees"],
  },
];

const FALLBACK_KEYS = ["whatElse", "localTips", "packingSuggestions"];

const COMPANION_LOCATION_KEYS = ["whatsNearby", "findCoffeeShop", "translateSomething"];
const COMPANION_DEFAULT_KEYS = ["findCoffeeShop", "navigateHotel", "translateSomething"];

/**
 * Returns companion-specific follow-up suggestion keys.
 */
export function generateCompanionFollowUps(hasLocation: boolean): string[] {
  return hasLocation ? COMPANION_LOCATION_KEYS : COMPANION_DEFAULT_KEYS;
}

/**
 * Pure function that generates follow-up suggestion i18n keys based on
 * the last assistant message content and optional trip context.
 * Returns 2-3 suggestion keys for use with the `chat.followUp.*` namespace.
 */
export function generateFollowUps(
  lastMessage: string,
  context?: TripContext,
): string[] {
  const matched: string[] = [];

  for (const rule of RULES) {
    if (rule.test(lastMessage, context)) {
      for (const key of rule.keys) {
        if (!matched.includes(key)) {
          matched.push(key);
        }
      }
    }
    if (matched.length >= 3) break;
  }

  if (matched.length === 0) {
    return FALLBACK_KEYS.slice(0, 3);
  }

  return matched.slice(0, 3);
}

export function FollowUpSuggestions({
  lastAssistantMessage,
  tripContext,
  onSelect,
  mode = "planning",
  hasLocation = false,
}: FollowUpSuggestionsProps) {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const fadeAnim = useRef(new Animated.Value(0)).current;

  const keys =
    mode === "companion"
      ? generateCompanionFollowUps(hasLocation)
      : generateFollowUps(lastAssistantMessage, tripContext);

  useEffect(() => {
    fadeAnim.setValue(0);
    Animated.timing(fadeAnim, {
      toValue: 1,
      duration: 300,
      useNativeDriver: true,
    }).start();
  }, [lastAssistantMessage, fadeAnim]);

  const styles = StyleSheet.create({
    container: {
      flexDirection: "row",
      flexWrap: "wrap",
      gap: 8,
      paddingVertical: 8,
      paddingHorizontal: 4,
    },
    chip: {
      backgroundColor: colors.surface,
      borderWidth: 1,
      borderColor: colors.accent,
      borderRadius: 20,
      paddingHorizontal: 14,
      paddingVertical: 8,
    },
    chipText: {
      fontSize: 13,
      color: colors.accent,
      fontWeight: "500",
    },
  });

  return (
    <Animated.View style={[styles.container, { opacity: fadeAnim }]}>
      {keys.map((key) => {
        const label = t(`chat.followUp.${key}`);
        return (
          <Pressable
            key={key}
            style={styles.chip}
            onPress={() => onSelect(label)}
            accessibilityRole="button"
            accessibilityLabel={label}
          >
            <Text style={styles.chipText}>{label}</Text>
          </Pressable>
        );
      })}
    </Animated.View>
  );
}
