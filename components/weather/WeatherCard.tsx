import { View, Text, StyleSheet, ScrollView, Pressable } from "react-native";
import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { ChevronDown, ChevronUp, Info } from "lucide-react-native";
import { useTheme } from "@/lib/theme";
import { type WeatherDay, getWeatherEmoji, celsiusToFahrenheit } from "@/lib/hooks/useWeather";

const TEMP_UNIT_KEY = "toqui_temp_unit";

interface WeatherCardProps {
  weather: WeatherDay[];
  isClimate: boolean;
}

function formatDayLabel(dateStr: string): string {
  const date = new Date(`${dateStr}T00:00:00Z`);
  return new Intl.DateTimeFormat("en-US", { weekday: "short", timeZone: "UTC" }).format(date);
}

function formatDateLabel(dateStr: string): string {
  const date = new Date(`${dateStr}T00:00:00Z`);
  return new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric", timeZone: "UTC" }).format(date);
}

export function WeatherCard({ weather, isClimate }: WeatherCardProps) {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const [expanded, setExpanded] = useState(true);
  const [useFahrenheit, setUseFahrenheit] = useState(false);
  const [showTooltip, setShowTooltip] = useState(false);

  useEffect(() => {
    AsyncStorage.getItem(TEMP_UNIT_KEY).then((val) => {
      if (val === "F") setUseFahrenheit(true);
    });
  }, []);

  const toggleUnit = () => {
    const next = !useFahrenheit;
    setUseFahrenheit(next);
    void AsyncStorage.setItem(TEMP_UNIT_KEY, next ? "F" : "C");
  };

  const formatTemp = (c: number) => {
    const val = useFahrenheit ? celsiusToFahrenheit(c) : c;
    return `${val}°`;
  };

  const styles = StyleSheet.create({
    container: {
      backgroundColor: colors.surface,
      borderRadius: 12,
      borderWidth: 1,
      borderColor: colors.border,
      marginBottom: 16,
      overflow: "hidden",
    },
    header: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
      padding: 12,
      paddingBottom: expanded ? 4 : 12,
    },
    headerLeft: {
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
    },
    headerTitle: {
      fontSize: 15,
      fontWeight: "600",
      color: colors.textPrimary,
    },
    unitToggle: {
      paddingHorizontal: 8,
      paddingVertical: 4,
      borderRadius: 6,
      backgroundColor: colors.surfaceTertiary,
    },
    unitToggleText: {
      fontSize: 12,
      fontWeight: "600",
      color: colors.textSecondary,
    },
    headerRight: {
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
    },
    scrollContent: {
      paddingHorizontal: 12,
      paddingBottom: 12,
      gap: 2,
    },
    dayColumn: {
      alignItems: "center",
      paddingHorizontal: 10,
      paddingVertical: 8,
      minWidth: 60,
    },
    dayLabel: {
      fontSize: 12,
      fontWeight: "600",
      color: colors.textSecondary,
      marginBottom: 2,
    },
    dateLabel: {
      fontSize: 11,
      color: colors.textTertiary,
      marginBottom: 6,
    },
    emoji: {
      fontSize: 22,
      marginBottom: 6,
    },
    tempHigh: {
      fontSize: 14,
      fontWeight: "600",
      color: colors.textPrimary,
    },
    tempLow: {
      fontSize: 12,
      color: colors.textTertiary,
      marginTop: 2,
    },
    tooltip: {
      backgroundColor: colors.surfaceTertiary,
      borderRadius: 8,
      padding: 8,
      marginHorizontal: 12,
      marginBottom: 8,
    },
    tooltipText: {
      fontSize: 12,
      color: colors.textSecondary,
      lineHeight: 16,
    },
  });

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <View style={styles.headerLeft}>
          <Text style={styles.headerTitle}>{t("weather.title")}</Text>
          {isClimate && (
            <Pressable
              onPress={() => setShowTooltip((v) => !v)}
              accessibilityRole="button"
              accessibilityLabel={t("weather.climateInfo")}
            >
              <Info color={colors.textTertiary} size={14} />
            </Pressable>
          )}
        </View>
        <View style={styles.headerRight}>
          <Pressable
            style={styles.unitToggle}
            onPress={toggleUnit}
            accessibilityRole="button"
            accessibilityLabel={useFahrenheit ? t("weather.switchToCelsius") : t("weather.switchToFahrenheit")}
          >
            <Text style={styles.unitToggleText}>
              {useFahrenheit ? "\u00B0F" : "\u00B0C"}
            </Text>
          </Pressable>
          <Pressable
            onPress={() => setExpanded((v) => !v)}
            accessibilityRole="button"
            accessibilityLabel={expanded ? t("weather.collapse") : t("weather.expand")}
          >
            {expanded ? (
              <ChevronUp color={colors.textTertiary} size={18} />
            ) : (
              <ChevronDown color={colors.textTertiary} size={18} />
            )}
          </Pressable>
        </View>
      </View>

      {showTooltip && isClimate && (
        <View style={styles.tooltip}>
          <Text style={styles.tooltipText}>{t("weather.climateNote")}</Text>
        </View>
      )}

      {expanded && (
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          contentContainerStyle={styles.scrollContent}
        >
          {weather.map((day) => (
            <View key={day.date} style={styles.dayColumn}>
              <Text style={styles.dayLabel}>{formatDayLabel(day.date)}</Text>
              <Text style={styles.dateLabel}>{formatDateLabel(day.date)}</Text>
              <Text style={styles.emoji}>{getWeatherEmoji(day.weatherCode)}</Text>
              <Text style={styles.tempHigh}>{formatTemp(day.tempHigh)}</Text>
              <Text style={styles.tempLow}>{formatTemp(day.tempLow)}</Text>
            </View>
          ))}
        </ScrollView>
      )}
    </View>
  );
}
