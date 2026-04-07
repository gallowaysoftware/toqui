import { View, Text, Pressable, StyleSheet, Linking } from "react-native";
import { ExternalLink, Plane, Hotel, Ticket, Car, Shield } from "lucide-react-native";
import type { Recommendation } from "@/lib/hooks/useChat";
import { useTheme } from "@/lib/theme";
import { useAnalytics } from "@/lib/analytics";

const partnerConfig: Record<string, { label: string; Icon: typeof Plane }> = {
  skyscanner: { label: "Skyscanner", Icon: Plane },
  "booking.com": { label: "Booking.com", Icon: Hotel },
  bookingcom: { label: "Booking.com", Icon: Hotel },
  booking_com: { label: "Booking.com", Icon: Hotel },
  getyourguide: { label: "GetYourGuide", Icon: Ticket },
  viator: { label: "Viator", Icon: Ticket },
  discovercars: { label: "DiscoverCars", Icon: Car },
  safetywing: { label: "SafetyWing", Icon: Shield },
};

interface RecommendationCardProps {
  recommendation: Recommendation;
}

export function RecommendationCard({ recommendation }: RecommendationCardProps) {
  const { colors } = useTheme();
  const { track } = useAnalytics();
  const config = partnerConfig[recommendation.partner.toLowerCase()];
  const Icon = config?.Icon ?? ExternalLink;
  const partnerLabel = config?.label ?? recommendation.partner;

  const styles = StyleSheet.create({
    card: {
      backgroundColor: colors.surface,
      borderRadius: 12,
      borderWidth: 1,
      borderColor: colors.border,
      padding: 14,
      marginBottom: 8,
      maxWidth: "85%",
      alignSelf: "flex-start",
    },
    header: {
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
      marginBottom: 8,
    },
    partner: { fontSize: 12, fontWeight: "600", color: colors.textSecondary },
    title: { fontSize: 15, fontWeight: "600", color: colors.textPrimary, marginBottom: 4 },
    description: { fontSize: 13, color: colors.textSecondary, marginBottom: 6, lineHeight: 18 },
    price: { fontSize: 16, fontWeight: "700", color: colors.accent, marginBottom: 8 },
    cta: {
      flexDirection: "row",
      alignItems: "center",
      gap: 6,
    },
    ctaText: { fontSize: 13, fontWeight: "600", color: colors.accent },
    disclosureBadge: {
      flexDirection: "row",
      alignItems: "center",
      marginBottom: 8,
    },
    disclosureLabel: {
      fontSize: 11,
      color: colors.textTertiary,
      fontWeight: "500",
      lineHeight: 14,
    },
  });

  return (
    <Pressable
      style={styles.card}
      onPress={() => {
        const url = recommendation.url;
        if (url.startsWith("https://")) {
          track("recommendation_clicked", {
            partner: recommendation.partner,
            category: recommendation.category,
          });
          Linking.openURL(url);
        }
      }}
    >
      <View style={styles.header}>
        <Icon color={colors.accent} size={20} />
        <Text style={styles.partner}>{partnerLabel}</Text>
      </View>
      <Text style={styles.title}>{recommendation.title}</Text>
      {recommendation.description ? (
        <Text style={styles.description} numberOfLines={2}>{recommendation.description}</Text>
      ) : null}
      {recommendation.price ? (
        <Text style={styles.price}>{recommendation.price}</Text>
      ) : null}
      <View style={styles.disclosureBadge}>
        <Text style={styles.disclosureLabel}>
          {recommendation.disclosure ?? "Partner link — booking supports Toqui at no cost to you."}
        </Text>
      </View>
      <View style={styles.cta}>
        <Text style={styles.ctaText}>View on {partnerLabel}</Text>
        <ExternalLink color={colors.accent} size={14} />
      </View>
    </Pressable>
  );
}
