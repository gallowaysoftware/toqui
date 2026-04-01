/**
 * Shared Trip Page — the #1 organic growth surface for Toqui.
 *
 * When users share their trip with friends (via messaging apps), this is the
 * page they land on. It needs to look magazine-quality and drive sign-ups.
 *
 * Open Graph meta tags should be added server-side when SSR is enabled:
 *   <meta property="og:title" content="{trip.title} — Toqui" />
 *   <meta property="og:description" content="Check out this trip itinerary on Toqui" />
 *   <meta property="og:type" content="article" />
 *   <meta property="og:url" content="https://app.toqui.travel/shared/{token}" />
 */

import { useEffect, useMemo } from "react";
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  Pressable,
  Platform,
  useWindowDimensions,
} from "react-native";
import { useLocalSearchParams, useRouter } from "expo-router";
import Head from "expo-router/head";
import { useQuery } from "@tanstack/react-query";
import {
  MapPin,
  Calendar,
  Clock,
  Compass,
  Utensils,
  Camera,
  Hotel,
  Plane,
  Ticket,
  Sun,
  Sunset,
  Moon,
  ChevronRight,
} from "lucide-react-native";
import { useTheme } from "@/lib/theme";
import { useAuth } from "@/lib/auth";
import type { ThemeColors } from "@/lib/theme";
import { getConfig } from "@/lib/config";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface SharedTripInfo {
  title: string;
  description?: string;
  destination_country?: string;
  status: string;
  start_date?: string;
  end_date?: string;
}

interface SharedItineraryItem {
  title: string;
  type?: string;
  description?: string;
}

interface SharedItineraryDay {
  day_number: number;
  items: SharedItineraryItem[];
}

interface SharedTripResponse {
  trip: SharedTripInfo;
  itinerary: SharedItineraryDay[];
}

// ---------------------------------------------------------------------------
// Gradient presets — destination-aware hero backgrounds
// ---------------------------------------------------------------------------

const destinationGradients: Record<string, [string, string]> = {
  japan: ["#e8415c", "#cc2b5e"],
  italy: ["#2d6a4f", "#52b788"],
  france: ["#2c3e7a", "#6c5ce7"],
  mexico: ["#e17055", "#d63031"],
  thailand: ["#f7b731", "#fa8231"],
  brazil: ["#20bf6b", "#26de81"],
  greece: ["#0984e3", "#74b9ff"],
  spain: ["#e17055", "#fdcb6e"],
  default: ["#BF4028", "#8B2F1E"],
};

function getHeroColors(destination?: string): [string, string] {
  if (!destination) return destinationGradients.default!;
  const lower = destination.toLowerCase();
  for (const [key, colors] of Object.entries(destinationGradients)) {
    if (lower.includes(key)) return colors;
  }
  return destinationGradients.default!;
}

// ---------------------------------------------------------------------------
// Activity type icons (shared logic with ItineraryTimeline)
// ---------------------------------------------------------------------------

const typeIcons: Record<string, typeof MapPin> = {
  restaurant: Utensils,
  food: Utensils,
  dining: Utensils,
  activity: Ticket,
  attraction: Camera,
  sightseeing: Camera,
  hotel: Hotel,
  accommodation: Hotel,
  flight: Plane,
  transport: Plane,
};

function getActivityIcon(type?: string) {
  if (!type) return MapPin;
  const lower = type.toLowerCase();
  for (const [key, Icon] of Object.entries(typeIcons)) {
    if (lower.includes(key)) return Icon;
  }
  return MapPin;
}

// ---------------------------------------------------------------------------
// Time-of-day grouping
// ---------------------------------------------------------------------------

type TimeOfDay = "morning" | "afternoon" | "evening";

function classifyItem(item: SharedItineraryItem, index: number, total: number): TimeOfDay {
  // Rough heuristic: divide items evenly into 3 groups
  const third = total / 3;
  if (index < third) return "morning";
  if (index < third * 2) return "afternoon";
  return "evening";
}

const timeOfDayMeta: Record<TimeOfDay, { label: string; Icon: typeof Sun }> = {
  morning: { label: "Morning", Icon: Sun },
  afternoon: { label: "Afternoon", Icon: Sunset },
  evening: { label: "Evening", Icon: Moon },
};

// ---------------------------------------------------------------------------
// Date formatting helpers
// ---------------------------------------------------------------------------

function formatDateRange(start?: string, end?: string): string {
  if (!start) return "";
  const fmt = (d: string) => {
    const date = new Date(d);
    return date.toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
  };
  if (!end || start === end) return fmt(start);
  return `${fmt(start)} - ${fmt(end)}`;
}

function computeTripDays(start?: string, end?: string): number {
  if (!start || !end) return 0;
  const ms = new Date(end).getTime() - new Date(start).getTime();
  return Math.max(1, Math.round(ms / (1000 * 60 * 60 * 24)) + 1);
}

// ---------------------------------------------------------------------------
// Referral check
// ---------------------------------------------------------------------------

function getPendingRef(): string | null {
  if (Platform.OS === "web" && typeof window !== "undefined") {
    return sessionStorage.getItem("toqui_pending_ref");
  }
  return null;
}

// ---------------------------------------------------------------------------
// Skeleton loader
// ---------------------------------------------------------------------------

function SkeletonBlock({
  width,
  height,
  radius,
  colors,
  style,
}: {
  width: number | string;
  height: number;
  radius?: number;
  colors: ThemeColors;
  style?: object;
}) {
  return (
    <View
      style={[
        {
          width: width as number,
          height,
          borderRadius: radius ?? 8,
          backgroundColor: colors.surfaceTertiary,
          opacity: 0.6,
        },
        style,
      ]}
    />
  );
}

function LoadingSkeleton({ colors }: { colors: ThemeColors }) {
  const styles = makeStyles(colors);
  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.scrollContent}>
      {/* Hero skeleton */}
      <View style={[styles.heroSection, { backgroundColor: colors.surfaceTertiary }]}>
        <SkeletonBlock width="70%" height={28} colors={colors} style={{ marginBottom: 12 }} />
        <SkeletonBlock width="50%" height={16} colors={colors} style={{ marginBottom: 8 }} />
        <SkeletonBlock width="40%" height={16} colors={colors} />
      </View>

      {/* Stats skeleton */}
      <View style={styles.statsRow}>
        {[1, 2, 3].map((i) => (
          <View key={i} style={[styles.statCard, { backgroundColor: colors.surface }]}>
            <SkeletonBlock width={32} height={32} radius={16} colors={colors} style={{ marginBottom: 8 }} />
            <SkeletonBlock width={40} height={20} colors={colors} style={{ marginBottom: 4 }} />
            <SkeletonBlock width={60} height={12} colors={colors} />
          </View>
        ))}
      </View>

      {/* Day cards skeleton */}
      {[1, 2].map((d) => (
        <View key={d} style={[styles.dayCard, { backgroundColor: colors.surface }]}>
          <SkeletonBlock width={80} height={24} radius={12} colors={colors} style={{ marginBottom: 16 }} />
          {[1, 2, 3].map((i) => (
            <View key={i} style={{ marginBottom: 12 }}>
              <SkeletonBlock width="80%" height={16} colors={colors} style={{ marginBottom: 6 }} />
              <SkeletonBlock width="60%" height={12} colors={colors} />
            </View>
          ))}
        </View>
      ))}
    </ScrollView>
  );
}

// ---------------------------------------------------------------------------
// Error states
// ---------------------------------------------------------------------------

function ErrorState({
  error,
  colors,
}: {
  error: Error | null;
  colors: ThemeColors;
}) {
  const styles = makeStyles(colors);
  const is404 = error?.message?.includes("404");

  return (
    <View style={[styles.centerFill, { backgroundColor: colors.surfaceSecondary }]}>
      <View style={[styles.errorCard, { backgroundColor: colors.surface, borderColor: colors.border }]}>
        <Text style={[styles.errorEmoji]}>{is404 ? "🗺" : "⚡"}</Text>
        <Text style={[styles.errorTitle, { color: colors.textPrimary }]}>
          {is404 ? "Trip not found" : "Something went wrong"}
        </Text>
        <Text style={[styles.errorBody, { color: colors.textSecondary }]}>
          {is404
            ? "This shared trip link may have expired or been removed by the owner."
            : "We couldn't load this trip right now. Please try again in a moment."}
        </Text>
      </View>
    </View>
  );
}

// ---------------------------------------------------------------------------
// Hero Section
// ---------------------------------------------------------------------------

function HeroSection({
  trip,
  colors,
  refCode,
}: {
  trip: SharedTripInfo;
  colors: ThemeColors;
  refCode: string | null;
}) {
  const styles = makeStyles(colors);
  const [gradientStart, gradientEnd] = getHeroColors(trip.destination_country);
  const dateStr = formatDateRange(trip.start_date, trip.end_date);

  return (
    <View style={styles.heroWrapper}>
      {/* Gradient background */}
      <View
        style={[
          styles.heroSection,
          {
            // LinearGradient isn't available in RN without expo-linear-gradient.
            // Use the primary gradient color as a solid background — still gorgeous.
            backgroundColor: gradientStart,
          },
        ]}
      >
        {/* Subtle pattern overlay */}
        <View style={styles.heroOverlay} />

        {/* Referral banner, placed inside hero for visual continuity */}
        {refCode && (
          <View style={styles.refBanner}>
            <Text style={styles.refBannerText}>
              Your friend shared this trip with you. Sign up and you both get a bonus.
            </Text>
          </View>
        )}

        {/* Destination icon area */}
        <View style={styles.heroIconContainer}>
          <Compass color="rgba(255,255,255,0.3)" size={64} />
        </View>

        {/* Title & meta */}
        <Text style={styles.heroTitle}>{trip.title}</Text>

        {trip.description ? (
          <Text style={styles.heroDescription} numberOfLines={3}>
            {trip.description}
          </Text>
        ) : null}

        <View style={styles.heroMeta}>
          {trip.destination_country ? (
            <View style={styles.heroChip}>
              <MapPin color="rgba(255,255,255,0.85)" size={14} />
              <Text style={styles.heroChipText}>{trip.destination_country}</Text>
            </View>
          ) : null}
          {dateStr ? (
            <View style={styles.heroChip}>
              <Calendar color="rgba(255,255,255,0.85)" size={14} />
              <Text style={styles.heroChipText}>{dateStr}</Text>
            </View>
          ) : null}
        </View>
      </View>

      {/* Curved bottom edge */}
      <View style={[styles.heroCurve, { backgroundColor: colors.surfaceSecondary }]} />
    </View>
  );
}

// ---------------------------------------------------------------------------
// Quick Stats
// ---------------------------------------------------------------------------

function QuickStats({
  trip,
  itinerary,
  colors,
}: {
  trip: SharedTripInfo;
  itinerary: SharedItineraryDay[];
  colors: ThemeColors;
}) {
  const styles = makeStyles(colors);
  const days = computeTripDays(trip.start_date, trip.end_date) || itinerary.length;
  const activities = itinerary.reduce((sum, d) => sum + d.items.length, 0);
  const locations = new Set(
    itinerary.flatMap((d) =>
      d.items.filter((i) => i.type).map((i) => i.type!.toLowerCase()),
    ),
  ).size;

  const stats = [
    { value: days || itinerary.length, label: days === 1 ? "Day" : "Days", Icon: Calendar },
    { value: activities, label: activities === 1 ? "Activity" : "Activities", Icon: Compass },
    { value: locations || itinerary.length, label: locations === 1 ? "Category" : "Categories", Icon: MapPin },
  ];

  return (
    <View style={styles.statsRow}>
      {stats.map((s) => (
        <View
          key={s.label}
          style={[styles.statCard, { backgroundColor: colors.surface, borderColor: colors.border }]}
        >
          <View style={[styles.statIconCircle, { backgroundColor: colors.accentSoft }]}>
            <s.Icon color={colors.accent} size={18} />
          </View>
          <Text style={[styles.statValue, { color: colors.textPrimary }]}>{s.value}</Text>
          <Text style={[styles.statLabel, { color: colors.textTertiary }]}>{s.label}</Text>
        </View>
      ))}
    </View>
  );
}

// ---------------------------------------------------------------------------
// Day Card with time-of-day grouping
// ---------------------------------------------------------------------------

const dayAccentColors = [
  "#BF4028", "#3b82f6", "#22c55e", "#f59e0b", "#8b5cf6",
  "#ec4899", "#06b6d4", "#ef4444", "#10b981", "#6366f1",
];

function DayCard({
  day,
  dayIndex,
  colors,
}: {
  day: SharedItineraryDay;
  dayIndex: number;
  colors: ThemeColors;
}) {
  const styles = makeStyles(colors);
  const accent = dayAccentColors[dayIndex % dayAccentColors.length]!;

  // Group items by time of day
  const grouped = useMemo(() => {
    const groups: Record<TimeOfDay, SharedItineraryItem[]> = {
      morning: [],
      afternoon: [],
      evening: [],
    };
    day.items.forEach((item, i) => {
      const tod = classifyItem(item, i, day.items.length);
      groups[tod].push(item);
    });
    return groups;
  }, [day.items]);

  return (
    <View style={[styles.dayCard, { backgroundColor: colors.surface, borderColor: colors.border }]}>
      {/* Day header */}
      <View style={styles.dayCardHeader}>
        <View style={[styles.dayBadge, { backgroundColor: accent }]}>
          <Text style={styles.dayBadgeText}>Day {day.day_number}</Text>
        </View>
        <Text style={[styles.dayItemCount, { color: colors.textTertiary }]}>
          {day.items.length} {day.items.length === 1 ? "activity" : "activities"}
        </Text>
      </View>

      {/* Time-of-day sections */}
      {(["morning", "afternoon", "evening"] as TimeOfDay[]).map((tod) => {
        const items = grouped[tod];
        if (items.length === 0) return null;
        const meta = timeOfDayMeta[tod];

        return (
          <View key={tod} style={styles.todSection}>
            <View style={styles.todHeader}>
              <meta.Icon color={colors.textTertiary} size={14} />
              <Text style={[styles.todLabel, { color: colors.textTertiary }]}>{meta.label}</Text>
            </View>
            {items.map((item, j) => {
              const Icon = getActivityIcon(item.type);
              return (
                <View key={j} style={styles.activityRow}>
                  <View style={[styles.activityDot, { backgroundColor: accent + "20" }]}>
                    <Icon color={accent} size={14} />
                  </View>
                  <View style={styles.activityContent}>
                    <Text style={[styles.activityTitle, { color: colors.textPrimary }]}>
                      {item.title}
                    </Text>
                    {item.description ? (
                      <Text
                        style={[styles.activityDesc, { color: colors.textSecondary }]}
                        numberOfLines={2}
                      >
                        {item.description}
                      </Text>
                    ) : null}
                    {item.type ? (
                      <View style={[styles.activityTypeBadge, { backgroundColor: accent + "15" }]}>
                        <Text style={[styles.activityTypeText, { color: accent }]}>{item.type}</Text>
                      </View>
                    ) : null}
                  </View>
                </View>
              );
            })}
          </View>
        );
      })}
    </View>
  );
}

// ---------------------------------------------------------------------------
// CTA Section
// ---------------------------------------------------------------------------

function CtaSection({
  colors,
  isLoggedIn,
  onPress,
}: {
  colors: ThemeColors;
  isLoggedIn: boolean;
  onPress: () => void;
}) {
  const styles = makeStyles(colors);

  return (
    <View style={[styles.ctaSection, { backgroundColor: colors.surface, borderColor: colors.border }]}>
      <Text style={[styles.ctaHeading, { color: colors.textPrimary }]}>
        {isLoggedIn ? "Like this itinerary?" : "Plan your own dream trip"}
      </Text>
      <Text style={[styles.ctaBody, { color: colors.textSecondary }]}>
        {isLoggedIn
          ? "Start a similar trip in your account and customize it with AI."
          : "Toqui's AI builds personalized itineraries in minutes. Free to try."}
      </Text>
      <Pressable
        style={({ pressed }) => [
          styles.ctaButton,
          { backgroundColor: colors.accent, opacity: pressed ? 0.9 : 1 },
        ]}
        onPress={onPress}
        accessibilityRole="button"
        accessibilityLabel={isLoggedIn ? "Plan a Similar Trip" : "Start Planning for Free"}
      >
        <Text style={styles.ctaButtonText}>
          {isLoggedIn ? "Plan a Similar Trip" : "Start Planning for Free"}
        </Text>
        <ChevronRight color="#fff" size={18} />
      </Pressable>
      {!isLoggedIn && (
        <Text style={[styles.ctaSocialProof, { color: colors.textTertiary }]}>
          Join 10,000+ travelers already using Toqui
        </Text>
      )}
    </View>
  );
}

// ---------------------------------------------------------------------------
// Footer
// ---------------------------------------------------------------------------

function Footer({ colors }: { colors: ThemeColors }) {
  const styles = makeStyles(colors);
  return (
    <View style={styles.footer}>
      <Text style={[styles.footerText, { color: colors.textTertiary }]}>
        Powered by{" "}
      </Text>
      <Pressable
        onPress={() => {
          if (Platform.OS === "web" && typeof window !== "undefined") {
            window.open("https://toqui.travel", "_blank");
          }
        }}
        accessibilityRole="link"
        accessibilityLabel="Visit toqui.travel"
      >
        <Text style={[styles.footerLink, { color: colors.accent }]}>Toqui</Text>
      </Pressable>
    </View>
  );
}

// ---------------------------------------------------------------------------
// Sticky CTA (mobile)
// ---------------------------------------------------------------------------

function StickyCta({
  colors,
  onPress,
  isLoggedIn,
}: {
  colors: ThemeColors;
  onPress: () => void;
  isLoggedIn: boolean;
}) {
  const styles = makeStyles(colors);
  return (
    <View style={[styles.stickyCta, { backgroundColor: colors.surface, borderTopColor: colors.border }]}>
      <Pressable
        style={({ pressed }) => [
          styles.stickyCtaButton,
          { backgroundColor: colors.accent, opacity: pressed ? 0.9 : 1 },
        ]}
        onPress={onPress}
        accessibilityRole="button"
        accessibilityLabel={isLoggedIn ? "Plan a Similar Trip" : "Start Planning for Free"}
      >
        <Text style={styles.stickyCtaText}>
          {isLoggedIn ? "Plan a Similar Trip" : "Start Planning for Free"}
        </Text>
      </Pressable>
    </View>
  );
}

// ---------------------------------------------------------------------------
// Main Screen
// ---------------------------------------------------------------------------

export default function SharedTripScreen() {
  const { token } = useLocalSearchParams<{ token: string }>();
  const { colors } = useTheme();
  const { accessToken } = useAuth();
  const router = useRouter();
  const refCode = getPendingRef();
  const { width } = useWindowDimensions();
  const isWide = width >= 768;

  const { data, isLoading, error } = useQuery<SharedTripResponse>({
    queryKey: ["shared-trip", token],
    queryFn: async () => {
      const res = await fetch(
        `${getConfig().apiUrl}/shared/${encodeURIComponent(token!)}`,
      );
      if (!res.ok) throw new Error(`Failed to load shared trip (${res.status})`);
      return res.json();
    },
    enabled: !!token,
  });

  const tripTitle = data?.trip?.title;

  useEffect(() => {
    if (tripTitle && typeof document !== "undefined") {
      document.title = `${tripTitle} — Toqui`;
    }
  }, [tripTitle]);

  const styles = makeStyles(colors);

  const handleCta = () => {
    if (accessToken) {
      router.push("/trips/new" as never);
    } else {
      router.push("/" as never);
    }
  };

  if (isLoading) {
    return <LoadingSkeleton colors={colors} />;
  }

  if (error || !data) {
    return <ErrorState error={error as Error | null} colors={colors} />;
  }

  const { trip, itinerary } = data;
  const sortedDays = itinerary
    .slice()
    .sort((a, b) => a.day_number - b.day_number);

  const ogTitle = trip.title
    ? `${trip.title} — Toqui`
    : "Check out this trip on Toqui";
  const ogDescription = trip.destination_country
    ? `AI-powered travel itinerary for ${trip.destination_country} with expert recommendations`
    : "AI-powered travel itinerary with expert recommendations";
  const ogUrl = `https://app.toqui.travel/shared/${token}`;
  const ogImage = "https://toqui.travel/og-share-image.png";

  return (
    <View style={styles.root}>
      <Head>
        <meta property="og:title" content={ogTitle} />
        <meta property="og:description" content={ogDescription} />
        <meta property="og:image" content={ogImage} />
        <meta property="og:url" content={ogUrl} />
        <meta property="og:type" content="website" />
        <meta property="og:site_name" content="Toqui" />
        <meta name="twitter:card" content="summary_large_image" />
        <meta name="twitter:title" content={ogTitle} />
        <meta name="twitter:description" content={ogDescription} />
        <meta name="twitter:image" content={ogImage} />
      </Head>
      <ScrollView
        style={styles.container}
        contentContainerStyle={[
          styles.scrollContent,
          // On wider screens, constrain content width for readability
          isWide && styles.wideContent,
        ]}
      >
        {/* 1. Hero */}
        <HeroSection trip={trip} colors={colors} refCode={refCode} />

        {/* 2. Quick Stats */}
        <QuickStats trip={trip} itinerary={sortedDays} colors={colors} />

        {/* 3. Section heading */}
        {sortedDays.length > 0 && (
          <View style={styles.sectionHeader}>
            <Text style={[styles.sectionTitle, { color: colors.textPrimary }]}>
              Itinerary
            </Text>
            <Text style={[styles.sectionSubtitle, { color: colors.textTertiary }]}>
              Day-by-day breakdown of this trip
            </Text>
          </View>
        )}

        {/* 4. Day-by-day itinerary cards */}
        {sortedDays.length > 0 ? (
          sortedDays.map((day, i) => (
            <DayCard key={day.day_number} day={day} dayIndex={i} colors={colors} />
          ))
        ) : (
          <View style={[styles.emptyItinerary, { backgroundColor: colors.surface, borderColor: colors.border }]}>
            <Compass color={colors.textTertiary} size={32} />
            <Text style={[styles.emptyTitle, { color: colors.textPrimary }]}>
              Itinerary in progress
            </Text>
            <Text style={[styles.emptyBody, { color: colors.textSecondary }]}>
              The trip owner is still planning this itinerary with AI. Check back soon!
            </Text>
          </View>
        )}

        {/* 5. CTA Section */}
        <CtaSection colors={colors} isLoggedIn={!!accessToken} onPress={handleCta} />

        {/* 6. Footer */}
        <Footer colors={colors} />

        {/* Bottom padding for sticky CTA */}
        <View style={{ height: 80 }} />
      </ScrollView>

      {/* Sticky CTA for mobile */}
      {!isWide && <StickyCta colors={colors} onPress={handleCta} isLoggedIn={!!accessToken} />}
    </View>
  );
}

// ---------------------------------------------------------------------------
// Styles — created inside a function so we can reference theme colors
// for border/shadow values that StyleSheet.create needs statically.
// We memoize via the colors object identity (stable from ThemeProvider).
// ---------------------------------------------------------------------------

const styleCache = new WeakMap<ThemeColors, ReturnType<typeof createStyles>>();

function makeStyles(colors: ThemeColors) {
  const cached = styleCache.get(colors);
  if (cached) return cached;
  const s = createStyles(colors);
  styleCache.set(colors, s);
  return s;
}

function createStyles(colors: ThemeColors) {
  return StyleSheet.create({
    root: {
      flex: 1,
      backgroundColor: colors.surfaceSecondary,
    },
    container: {
      flex: 1,
    },
    scrollContent: {
      paddingBottom: 0,
    },
    wideContent: {
      maxWidth: 680,
      alignSelf: "center",
      width: "100%",
    },
    centerFill: {
      flex: 1,
      justifyContent: "center",
      alignItems: "center",
      padding: 24,
    },

    // ----- Hero -----
    heroWrapper: {
      position: "relative",
      marginBottom: -16,
    },
    heroSection: {
      paddingTop: 48,
      paddingBottom: 40,
      paddingHorizontal: 24,
    },
    heroOverlay: {
      position: "absolute",
      top: 0,
      left: 0,
      right: 0,
      bottom: 0,
      backgroundColor: "rgba(0,0,0,0.15)",
    },
    heroCurve: {
      height: 24,
      borderTopLeftRadius: 24,
      borderTopRightRadius: 24,
      marginTop: -24,
      position: "relative",
      zIndex: 1,
    },
    heroIconContainer: {
      marginBottom: 20,
      opacity: 0.8,
    },
    heroTitle: {
      fontSize: 30,
      fontWeight: "800",
      color: "#fff",
      marginBottom: 10,
      lineHeight: 36,
    },
    heroDescription: {
      fontSize: 16,
      color: "rgba(255,255,255,0.85)",
      lineHeight: 24,
      marginBottom: 16,
    },
    heroMeta: {
      flexDirection: "row",
      flexWrap: "wrap",
      gap: 12,
    },
    heroChip: {
      flexDirection: "row",
      alignItems: "center",
      gap: 6,
      backgroundColor: "rgba(255,255,255,0.18)",
      paddingHorizontal: 12,
      paddingVertical: 6,
      borderRadius: 20,
    },
    heroChipText: {
      color: "rgba(255,255,255,0.95)",
      fontSize: 13,
      fontWeight: "500",
    },

    // ----- Referral banner -----
    refBanner: {
      backgroundColor: "rgba(255,255,255,0.18)",
      borderRadius: 10,
      padding: 12,
      marginBottom: 20,
    },
    refBannerText: {
      fontSize: 14,
      lineHeight: 20,
      color: "rgba(255,255,255,0.95)",
    },

    // ----- Stats -----
    statsRow: {
      flexDirection: "row",
      gap: 12,
      paddingHorizontal: 16,
      marginTop: -8,
      marginBottom: 24,
    },
    statCard: {
      flex: 1,
      alignItems: "center",
      paddingVertical: 16,
      paddingHorizontal: 8,
      borderRadius: 14,
      borderWidth: 1,
    },
    statIconCircle: {
      width: 36,
      height: 36,
      borderRadius: 18,
      justifyContent: "center",
      alignItems: "center",
      marginBottom: 8,
    },
    statValue: {
      fontSize: 22,
      fontWeight: "700",
    },
    statLabel: {
      fontSize: 12,
      fontWeight: "500",
      marginTop: 2,
    },

    // ----- Section header -----
    sectionHeader: {
      paddingHorizontal: 16,
      marginBottom: 16,
    },
    sectionTitle: {
      fontSize: 22,
      fontWeight: "700",
      marginBottom: 4,
    },
    sectionSubtitle: {
      fontSize: 14,
    },

    // ----- Day card -----
    dayCard: {
      marginHorizontal: 16,
      marginBottom: 16,
      borderRadius: 16,
      padding: 20,
      borderWidth: 1,
    },
    dayCardHeader: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
      marginBottom: 16,
    },
    dayBadge: {
      paddingHorizontal: 14,
      paddingVertical: 6,
      borderRadius: 20,
    },
    dayBadgeText: {
      color: "#fff",
      fontSize: 13,
      fontWeight: "700",
    },
    dayItemCount: {
      fontSize: 13,
    },

    // ----- Time of day -----
    todSection: {
      marginBottom: 16,
    },
    todHeader: {
      flexDirection: "row",
      alignItems: "center",
      gap: 6,
      marginBottom: 10,
      paddingBottom: 6,
      borderBottomWidth: StyleSheet.hairlineWidth,
      borderBottomColor: colors.border,
    },
    todLabel: {
      fontSize: 12,
      fontWeight: "600",
      textTransform: "uppercase",
      letterSpacing: 0.5,
    },

    // ----- Activity row -----
    activityRow: {
      flexDirection: "row",
      marginBottom: 12,
      gap: 12,
    },
    activityDot: {
      width: 32,
      height: 32,
      borderRadius: 16,
      justifyContent: "center",
      alignItems: "center",
      marginTop: 2,
    },
    activityContent: {
      flex: 1,
    },
    activityTitle: {
      fontSize: 15,
      fontWeight: "600",
      lineHeight: 20,
    },
    activityDesc: {
      fontSize: 13,
      lineHeight: 19,
      marginTop: 3,
    },
    activityTypeBadge: {
      alignSelf: "flex-start",
      paddingHorizontal: 8,
      paddingVertical: 3,
      borderRadius: 6,
      marginTop: 6,
    },
    activityTypeText: {
      fontSize: 11,
      fontWeight: "600",
      textTransform: "capitalize",
    },

    // ----- Empty itinerary -----
    emptyItinerary: {
      marginHorizontal: 16,
      marginBottom: 16,
      borderRadius: 16,
      padding: 32,
      alignItems: "center",
      borderWidth: 1,
    },
    emptyTitle: {
      fontSize: 17,
      fontWeight: "600",
      marginTop: 12,
      marginBottom: 6,
    },
    emptyBody: {
      fontSize: 14,
      lineHeight: 20,
      textAlign: "center",
    },

    // ----- CTA -----
    ctaSection: {
      marginHorizontal: 16,
      marginTop: 8,
      marginBottom: 16,
      borderRadius: 16,
      padding: 24,
      alignItems: "center",
      borderWidth: 1,
    },
    ctaHeading: {
      fontSize: 22,
      fontWeight: "700",
      marginBottom: 8,
      textAlign: "center",
    },
    ctaBody: {
      fontSize: 15,
      lineHeight: 22,
      textAlign: "center",
      marginBottom: 20,
    },
    ctaButton: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "center",
      gap: 6,
      borderRadius: 12,
      paddingVertical: 14,
      paddingHorizontal: 28,
      width: "100%",
      marginBottom: 12,
    },
    ctaButtonText: {
      color: "#fff",
      fontSize: 16,
      fontWeight: "600",
    },
    ctaSocialProof: {
      fontSize: 13,
      textAlign: "center",
    },

    // ----- Footer -----
    footer: {
      flexDirection: "row",
      justifyContent: "center",
      alignItems: "center",
      paddingVertical: 24,
    },
    footerText: {
      fontSize: 13,
    },
    footerLink: {
      fontSize: 13,
      fontWeight: "600",
    },

    // ----- Sticky CTA -----
    stickyCta: {
      position: "absolute",
      bottom: 0,
      left: 0,
      right: 0,
      paddingHorizontal: 16,
      paddingVertical: 12,
      borderTopWidth: StyleSheet.hairlineWidth,
    },
    stickyCtaButton: {
      borderRadius: 12,
      paddingVertical: 14,
      alignItems: "center",
      justifyContent: "center",
    },
    stickyCtaText: {
      color: "#fff",
      fontSize: 16,
      fontWeight: "600",
    },

    // ----- Error -----
    errorCard: {
      borderRadius: 16,
      padding: 32,
      alignItems: "center",
      borderWidth: 1,
      maxWidth: 400,
      width: "100%",
    },
    errorEmoji: {
      fontSize: 48,
      marginBottom: 16,
    },
    errorTitle: {
      fontSize: 20,
      fontWeight: "700",
      marginBottom: 8,
      textAlign: "center",
    },
    errorBody: {
      fontSize: 15,
      lineHeight: 22,
      textAlign: "center",
    },
  });
}
