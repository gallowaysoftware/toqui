import { View, Text, Pressable, StyleSheet, Linking } from "react-native";
import { ExternalLink, Plane, Hotel, Ticket, Car, Shield } from "lucide-react-native";
import type { Recommendation } from "@/lib/hooks/useChat";

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
  const config = partnerConfig[recommendation.partner.toLowerCase()];
  const Icon = config?.Icon ?? ExternalLink;
  const partnerLabel = config?.label ?? recommendation.partner;

  return (
    <Pressable
      style={styles.card}
      onPress={() => Linking.openURL(recommendation.url)}
    >
      <View style={styles.header}>
        <Icon color="#e8654a" size={20} />
        <Text style={styles.partner}>{partnerLabel}</Text>
      </View>
      <Text style={styles.title}>{recommendation.title}</Text>
      {recommendation.description ? (
        <Text style={styles.description} numberOfLines={2}>{recommendation.description}</Text>
      ) : null}
      {recommendation.price ? (
        <Text style={styles.price}>{recommendation.price}</Text>
      ) : null}
      <View style={styles.cta}>
        <Text style={styles.ctaText}>View on {partnerLabel}</Text>
        <ExternalLink color="#e8654a" size={14} />
      </View>
      {recommendation.disclosure ? (
        <Text style={styles.disclosure}>{recommendation.disclosure}</Text>
      ) : null}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: "#fff",
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "#e0e0e0",
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
  partner: { fontSize: 12, fontWeight: "600", color: "#666" },
  title: { fontSize: 15, fontWeight: "600", color: "#333", marginBottom: 4 },
  description: { fontSize: 13, color: "#666", marginBottom: 6, lineHeight: 18 },
  price: { fontSize: 16, fontWeight: "700", color: "#e8654a", marginBottom: 8 },
  cta: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
  },
  ctaText: { fontSize: 13, fontWeight: "600", color: "#e8654a" },
  disclosure: { fontSize: 10, color: "#999", marginTop: 8, lineHeight: 14 },
});
