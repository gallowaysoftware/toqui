import { View, Text, Pressable, ScrollView, StyleSheet, TextInput } from "react-native";
import { useState, useMemo, useCallback } from "react";
import { useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import {
  Heart,
  Landmark,
  TreePalm,
  Building2,
  Car,
  Sun,
  Backpack,
  Users,
  Sailboat,
  Compass,
  Search,
  Clock,
  MapPin,
} from "lucide-react-native";
import type { LucideIcon } from "lucide-react-native";
import { useTheme } from "@/lib/theme";
import {
  tripTemplates,
  TEMPLATE_CATEGORIES,
  searchTemplates,
  getTemplatesByCategory,
} from "@/lib/data/tripTemplates";
import type { TripTemplate, TemplateCategory } from "@/lib/data/tripTemplates";

const ICON_MAP: Record<string, LucideIcon> = {
  Heart,
  Landmark,
  TreePalm,
  Building2,
  Car,
  Sun,
  Backpack,
  Users,
  Sailboat,
  Compass,
};

function TemplateCard({ template, compact }: { template: TripTemplate; compact?: boolean }) {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const router = useRouter();
  const Icon = ICON_MAP[template.icon] ?? MapPin;

  const handlePress = useCallback(() => {
    router.push({
      pathname: "/trips/new" as never,
      params: { template: template.id },
    });
  }, [router, template.id]);

  if (compact) {
    return (
      <Pressable
        style={[
          styles.compactCard,
          {
            backgroundColor: colors.surface,
            borderColor: colors.border,
          },
        ]}
        onPress={handlePress}
        accessibilityLabel={t(template.titleKey)}
        accessibilityRole="button"
      >
        <View style={[styles.compactIconWrap, { backgroundColor: template.accentColor + "18" }]}>
          <Icon color={template.accentColor} size={18} />
        </View>
        <View style={styles.compactInfo}>
          <Text style={[styles.compactTitle, { color: colors.textPrimary }]} numberOfLines={1}>
            {t(template.titleKey)}
          </Text>
          <View style={styles.compactMeta}>
            <Clock color={colors.textTertiary} size={11} />
            <Text style={[styles.compactMetaText, { color: colors.textTertiary }]}>
              {t("templates.duration", { count: template.duration })}
            </Text>
          </View>
        </View>
      </Pressable>
    );
  }

  return (
    <Pressable
      style={[
        styles.card,
        {
          backgroundColor: colors.surface,
          borderColor: colors.border,
        },
      ]}
      onPress={handlePress}
      accessibilityLabel={t(template.titleKey)}
      accessibilityRole="button"
    >
      <View style={[styles.iconWrap, { backgroundColor: template.accentColor + "18" }]}>
        <Icon color={template.accentColor} size={24} />
      </View>
      <View style={styles.cardContent}>
        <Text style={[styles.cardTitle, { color: colors.textPrimary }]} numberOfLines={1}>
          {t(template.titleKey)}
        </Text>
        <Text style={[styles.cardDescription, { color: colors.textSecondary }]} numberOfLines={2}>
          {t(template.descriptionKey)}
        </Text>
        <View style={styles.cardMeta}>
          <View style={styles.metaBadge}>
            <MapPin color={colors.textTertiary} size={12} />
            <Text style={[styles.metaText, { color: colors.textTertiary }]}>
              {template.destination}
            </Text>
          </View>
          <View style={styles.metaBadge}>
            <Clock color={colors.textTertiary} size={12} />
            <Text style={[styles.metaText, { color: colors.textTertiary }]}>
              {t("templates.duration", { count: template.duration })}
            </Text>
          </View>
          <View
            style={[
              styles.categoryBadge,
              { backgroundColor: template.accentColor + "18" },
            ]}
          >
            <Text style={[styles.categoryText, { color: template.accentColor }]}>
              {t(`templates.categories.${template.category}`)}
            </Text>
          </View>
        </View>
      </View>
    </Pressable>
  );
}

interface TemplateBrowserProps {
  compact?: boolean;
}

export function TemplateBrowser({ compact }: TemplateBrowserProps) {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedCategory, setSelectedCategory] = useState<TemplateCategory | null>(null);

  const filteredTemplates = useMemo(() => {
    if (searchQuery.trim()) {
      return searchTemplates(searchQuery.trim());
    }
    if (selectedCategory) {
      return getTemplatesByCategory(selectedCategory);
    }
    return tripTemplates;
  }, [searchQuery, selectedCategory]);

  const handleCategoryPress = useCallback(
    (category: TemplateCategory) => {
      setSelectedCategory((prev) => (prev === category ? null : category));
      setSearchQuery("");
    },
    [],
  );

  return (
    <View style={styles.container}>
      {!compact && (
        <Text style={[styles.sectionTitle, { color: colors.textPrimary }]}>
          {t("templates.title")}
        </Text>
      )}

      {!compact && (
        <View
          style={[
            styles.searchContainer,
            { backgroundColor: colors.inputBg, borderColor: colors.inputBorder },
          ]}
        >
          <Search color={colors.textTertiary} size={16} />
          <TextInput
            style={[styles.searchInput, { color: colors.textPrimary }]}
            placeholder={t("templates.searchPlaceholder")}
            placeholderTextColor={colors.textTertiary}
            value={searchQuery}
            onChangeText={(text) => {
              setSearchQuery(text);
              setSelectedCategory(null);
            }}
            accessibilityLabel="Search templates"
          />
        </View>
      )}

      {!compact && (
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          style={styles.categoryScroll}
          contentContainerStyle={styles.categoryScrollContent}
        >
          {TEMPLATE_CATEGORIES.map((cat) => (
            <Pressable
              key={cat}
              style={[
                styles.categoryChip,
                {
                  backgroundColor:
                    selectedCategory === cat ? colors.accent : colors.surfaceTertiary,
                },
              ]}
              onPress={() => handleCategoryPress(cat)}
              accessibilityLabel={t(`templates.categories.${cat}`)}
              accessibilityRole="button"
            >
              <Text
                style={[
                  styles.categoryChipText,
                  { color: selectedCategory === cat ? "#fff" : colors.textSecondary },
                ]}
              >
                {t(`templates.categories.${cat}`)}
              </Text>
            </Pressable>
          ))}
        </ScrollView>
      )}

      {filteredTemplates.length === 0 ? (
        <Text style={[styles.emptyText, { color: colors.textTertiary }]}>
          {t("templates.noResults")}
        </Text>
      ) : (
        <View style={compact ? styles.compactGrid : styles.grid}>
          {filteredTemplates.map((template) => (
            <TemplateCard key={template.id} template={template} compact={compact} />
          ))}
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    width: "100%",
  },
  sectionTitle: {
    fontSize: 20,
    fontWeight: "700",
    marginBottom: 12,
  },
  searchContainer: {
    flexDirection: "row",
    alignItems: "center",
    borderWidth: 1,
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 8,
    marginBottom: 12,
    gap: 8,
  },
  searchInput: {
    flex: 1,
    fontSize: 15,
    padding: 0,
  },
  categoryScroll: {
    marginBottom: 16,
  },
  categoryScrollContent: {
    gap: 8,
  },
  categoryChip: {
    paddingHorizontal: 14,
    paddingVertical: 6,
    borderRadius: 16,
  },
  categoryChipText: {
    fontSize: 13,
    fontWeight: "600",
  },
  grid: {
    gap: 12,
  },
  compactGrid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 10,
  },
  card: {
    flexDirection: "row",
    borderRadius: 12,
    padding: 14,
    borderWidth: 1,
    gap: 12,
  },
  iconWrap: {
    width: 48,
    height: 48,
    borderRadius: 12,
    alignItems: "center",
    justifyContent: "center",
  },
  cardContent: {
    flex: 1,
  },
  cardTitle: {
    fontSize: 16,
    fontWeight: "600",
    marginBottom: 4,
  },
  cardDescription: {
    fontSize: 13,
    lineHeight: 18,
    marginBottom: 8,
  },
  cardMeta: {
    flexDirection: "row",
    alignItems: "center",
    gap: 10,
    flexWrap: "wrap",
  },
  metaBadge: {
    flexDirection: "row",
    alignItems: "center",
    gap: 4,
  },
  metaText: {
    fontSize: 12,
  },
  categoryBadge: {
    paddingHorizontal: 8,
    paddingVertical: 2,
    borderRadius: 10,
  },
  categoryText: {
    fontSize: 11,
    fontWeight: "600",
  },
  compactCard: {
    flexDirection: "row",
    alignItems: "center",
    borderRadius: 10,
    padding: 10,
    borderWidth: 1,
    gap: 10,
    width: "48%",
  },
  compactIconWrap: {
    width: 36,
    height: 36,
    borderRadius: 8,
    alignItems: "center",
    justifyContent: "center",
  },
  compactInfo: {
    flex: 1,
  },
  compactTitle: {
    fontSize: 13,
    fontWeight: "600",
    marginBottom: 2,
  },
  compactMeta: {
    flexDirection: "row",
    alignItems: "center",
    gap: 3,
  },
  compactMetaText: {
    fontSize: 11,
  },
  emptyText: {
    fontSize: 14,
    textAlign: "center",
    paddingVertical: 24,
  },
});
