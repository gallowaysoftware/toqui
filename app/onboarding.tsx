import { useRef, useState, useCallback } from "react";
import {
  View,
  Text,
  ScrollView,
  Pressable,
  StyleSheet,
  Dimensions,
  type NativeSyntheticEvent,
  type NativeScrollEvent,
} from "react-native";
import { useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import { Compass, Map, Briefcase } from "lucide-react-native";
import { useTheme } from "@/lib/theme";
import { useOnboarding } from "@/lib/hooks/useOnboarding";

const SCREEN_COUNT = 3;

interface SlideData {
  headlineKey: string;
  subtextKey: string;
  Icon: typeof Compass;
}

const SLIDES: SlideData[] = [
  {
    headlineKey: "onboarding.planSmarter.headline",
    subtextKey: "onboarding.planSmarter.subtext",
    Icon: Compass,
  },
  {
    headlineKey: "onboarding.travelConfidently.headline",
    subtextKey: "onboarding.travelConfidently.subtext",
    Icon: Map,
  },
  {
    headlineKey: "onboarding.getStarted.headline",
    subtextKey: "onboarding.getStarted.subtext",
    Icon: Briefcase,
  },
];

export default function OnboardingScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { colors } = useTheme();
  const { completeOnboarding } = useOnboarding();
  const scrollRef = useRef<ScrollView>(null);
  const [activeIndex, setActiveIndex] = useState(0);
  const screenWidth = Dimensions.get("window").width;

  const handleScroll = useCallback(
    (event: NativeSyntheticEvent<NativeScrollEvent>) => {
      const offsetX = event.nativeEvent.contentOffset.x;
      const index = Math.round(offsetX / screenWidth);
      setActiveIndex(index);
    },
    [screenWidth],
  );

  const handleStartPlanning = useCallback(async () => {
    await completeOnboarding();
    router.replace("/trips/new" as never);
  }, [completeOnboarding, router]);

  const handleExploreFirst = useCallback(async () => {
    await completeOnboarding();
    router.replace("/(tabs)" as never);
  }, [completeOnboarding, router]);

  const styles = StyleSheet.create({
    container: {
      flex: 1,
      backgroundColor: colors.surfaceSecondary,
    },
    scrollView: {
      flex: 1,
    },
    slide: {
      width: screenWidth,
      flex: 1,
      justifyContent: "center",
      alignItems: "center",
      paddingHorizontal: 40,
    },
    iconContainer: {
      width: 120,
      height: 120,
      borderRadius: 60,
      backgroundColor: colors.accentSoft,
      justifyContent: "center",
      alignItems: "center",
      marginBottom: 32,
    },
    headline: {
      fontSize: 28,
      fontWeight: "bold",
      color: colors.textPrimary,
      textAlign: "center",
      marginBottom: 12,
    },
    subtext: {
      fontSize: 16,
      color: colors.textSecondary,
      textAlign: "center",
      lineHeight: 24,
      maxWidth: 320,
    },
    footer: {
      paddingHorizontal: 24,
      paddingBottom: 48,
      alignItems: "center",
    },
    dotsContainer: {
      flexDirection: "row",
      justifyContent: "center",
      gap: 8,
      marginBottom: 24,
    },
    dot: {
      width: 8,
      height: 8,
      borderRadius: 4,
    },
    dotActive: {
      backgroundColor: colors.accent,
    },
    dotInactive: {
      backgroundColor: colors.border,
    },
    primaryButton: {
      backgroundColor: colors.accent,
      borderRadius: 12,
      paddingVertical: 16,
      paddingHorizontal: 32,
      width: "100%",
      maxWidth: 320,
      alignItems: "center",
      marginBottom: 12,
    },
    primaryButtonText: {
      color: "#fff",
      fontSize: 17,
      fontWeight: "600",
    },
    secondaryButton: {
      paddingVertical: 12,
      paddingHorizontal: 32,
      alignItems: "center",
    },
    secondaryButtonText: {
      color: colors.textSecondary,
      fontSize: 15,
      fontWeight: "500",
    },
  });

  return (
    <View style={styles.container}>
      <ScrollView
        ref={scrollRef}
        horizontal
        pagingEnabled
        showsHorizontalScrollIndicator={false}
        onScroll={handleScroll}
        scrollEventThrottle={16}
        style={styles.scrollView}
        testID="onboarding-scroll"
      >
        {SLIDES.map((slide, index) => (
          <View key={index} style={styles.slide} testID={`onboarding-slide-${index}`}>
            <View style={styles.iconContainer}>
              <slide.Icon color={colors.accent} size={56} />
            </View>
            <Text style={styles.headline}>{t(slide.headlineKey)}</Text>
            <Text style={styles.subtext}>{t(slide.subtextKey)}</Text>
          </View>
        ))}
      </ScrollView>

      <View style={styles.footer}>
        <View style={styles.dotsContainer} accessibilityLabel={`Page ${activeIndex + 1} of ${SCREEN_COUNT}`}>
          {Array.from({ length: SCREEN_COUNT }).map((_, i) => (
            <View
              key={i}
              style={[styles.dot, i === activeIndex ? styles.dotActive : styles.dotInactive]}
            />
          ))}
        </View>

        {activeIndex === SCREEN_COUNT - 1 ? (
          <>
            <Pressable
              style={styles.primaryButton}
              onPress={handleStartPlanning}
              accessibilityRole="button"
              accessibilityLabel={t("onboarding.getStarted.startPlanning")}
              testID="onboarding-start-planning"
            >
              <Text style={styles.primaryButtonText}>
                {t("onboarding.getStarted.startPlanning")}
              </Text>
            </Pressable>
            <Pressable
              style={styles.secondaryButton}
              onPress={handleExploreFirst}
              accessibilityRole="button"
              accessibilityLabel={t("onboarding.getStarted.exploreFirst")}
              testID="onboarding-explore-first"
            >
              <Text style={styles.secondaryButtonText}>
                {t("onboarding.getStarted.exploreFirst")}
              </Text>
            </Pressable>
          </>
        ) : (
          <View style={{ height: 80 }} />
        )}
      </View>
    </View>
  );
}
