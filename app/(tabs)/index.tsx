import { View, Text, StyleSheet, Pressable, FlatList, ActivityIndicator, ScrollView, Platform } from "react-native";
import { useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import { Plus, MapPin, ChevronRight, Crown, Plane, AlertCircle, RefreshCw } from "lucide-react-native";
import { useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth";
import { useGoogleAuth } from "@/lib/google-auth";
import { useTrips } from "@/lib/hooks/useTrips";
import { useTheme } from "@/lib/theme";
import { TripStatus } from "@gen/toqui/v1/trip_pb";
import type { Trip } from "@gen/toqui/v1/trip_pb";

const DESTINATIONS = [
  { key: "tokyo", flag: "\u{1F1EF}\u{1F1F5}", title: "Tokyo" },
  { key: "paris", flag: "\u{1F1EB}\u{1F1F7}", title: "Paris" },
  { key: "rome", flag: "\u{1F1EE}\u{1F1F9}", title: "Rome" },
  { key: "bali", flag: "\u{1F1EE}\u{1F1E9}", title: "Bali" },
  { key: "newYork", flag: "\u{1F1FA}\u{1F1F8}", title: "New York" },
] as const;

function TripCard({ trip, onPress }: { trip: Trip; onPress: () => void }) {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const statusConfig: Record<number, { labelKey: string; color: string }> = {
    [TripStatus.PLANNING]: { labelKey: "trips.status.planning", color: colors.info },
    [TripStatus.ACTIVE]: { labelKey: "trips.status.active", color: colors.success },
    [TripStatus.COMPLETED]: { labelKey: "trips.status.completed", color: colors.textTertiary },
  };
  const { labelKey, color: statusColor } = statusConfig[trip.status] ?? { labelKey: "trips.status.planning", color: colors.textTertiary };
  const statusLabel = t(labelKey);

  return (
    <Pressable
      style={{
        backgroundColor: colors.surface,
        borderRadius: 12,
        padding: 16,
        marginBottom: 12,
        flexDirection: "row",
        alignItems: "center",
        borderWidth: 1,
        borderColor: colors.border,
      }}
      onPress={onPress}
      accessibilityLabel={`Open trip: ${trip.title}`}
      accessibilityRole="button"
    >
      <View style={{ flex: 1 }}>
        <View style={{ flexDirection: "row", alignItems: "center", gap: 8, marginBottom: 4 }}>
          <Text style={{ fontSize: 16, fontWeight: "600", color: colors.textPrimary, flex: 1 }} numberOfLines={1}>{trip.title}</Text>
          <View style={{ paddingHorizontal: 8, paddingVertical: 2, borderRadius: 10, backgroundColor: statusColor }}>
            <Text style={{ fontSize: 11, fontWeight: "600", color: "#fff", textTransform: "capitalize" }}>{statusLabel}</Text>
          </View>
          {trip.isUnlocked && (
            <View style={{ flexDirection: "row", alignItems: "center", gap: 3, backgroundColor: colors.accent, paddingHorizontal: 7, paddingVertical: 2, borderRadius: 10 }}>
              <Crown color="#fff" size={10} />
              <Text style={{ fontSize: 11, fontWeight: "700", color: "#fff" }}>{t("trips.proBadge")}</Text>
            </View>
          )}
        </View>
        {trip.description ? (
          <Text style={{ fontSize: 14, color: colors.textSecondary, marginBottom: 8 }} numberOfLines={2}>{trip.description}</Text>
        ) : null}
        {trip.destinationCountry ? (
          <View style={{ flexDirection: "row", alignItems: "center", gap: 4 }}>
            <MapPin color={colors.textTertiary} size={14} />
            <Text style={{ fontSize: 12, color: colors.textTertiary }}>{trip.destinationCountry}</Text>
          </View>
        ) : null}
      </View>
      <ChevronRight color={colors.border} size={20} />
    </Pressable>
  );
}

export default function TripsScreen() {
  const { t } = useTranslation();
  const { accessToken, isLoading: authLoading } = useAuth();
  const router = useRouter();
  const { signIn, isReady: authReady } = useGoogleAuth();
  const queryClient = useQueryClient();
  const { trips, isLoading: tripsLoading, isError: tripsError } = useTrips();
  const { colors } = useTheme();

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surfaceSecondary },
    center: { flex: 1, justifyContent: "center", alignItems: "center", padding: 24 },
    signInContent: { flexGrow: 1, justifyContent: "center", padding: 24, alignItems: "center" },
    signInTitle: { fontSize: 36, fontWeight: "bold", color: colors.accent, marginBottom: 4 },
    signInTagline: { fontSize: 17, color: colors.textSecondary, textAlign: "center", marginBottom: 28 },
    valueProps: { marginBottom: 32, width: "100%" },
    valueProp: { fontSize: 14, color: colors.textSecondary, textAlign: "center", lineHeight: 24, marginBottom: 6 },
    signInNote: { fontSize: 12, color: colors.textTertiary, textAlign: "center", marginTop: 12 },
    welcomeContent: { padding: 24, alignItems: "center" },
    welcomeIcon: { marginTop: 32, marginBottom: 16 },
    welcomeTitle: { fontSize: 24, fontWeight: "bold", color: colors.textPrimary, marginBottom: 6, textAlign: "center" },
    welcomeSubtitle: { fontSize: 15, color: colors.textSecondary, textAlign: "center", marginBottom: 28 },
    destinationList: { width: "100%", marginBottom: 24 },
    destinationCard: {
      flexDirection: "row",
      alignItems: "center",
      backgroundColor: colors.surface,
      borderRadius: 12,
      padding: 14,
      marginBottom: 10,
      borderWidth: 1,
      borderColor: colors.border,
    },
    destinationFlag: { fontSize: 28, marginRight: 12 },
    destinationInfo: { flex: 1 },
    destinationName: { fontSize: 16, fontWeight: "600", color: colors.textPrimary },
    destinationHook: { fontSize: 13, color: colors.textTertiary, marginTop: 2 },
    primaryButton: {
      backgroundColor: colors.accent,
      borderRadius: 8,
      paddingVertical: 14,
      paddingHorizontal: 24,
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
    },
    disabledButton: { opacity: 0.5 },
    buttonText: { color: "#fff", fontSize: 16, fontWeight: "600" },
    listContent: { padding: 16 },
    newTripButton: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "center",
      gap: 8,
      padding: 14,
      backgroundColor: colors.surface,
      borderRadius: 12,
      borderWidth: 1,
      borderColor: colors.accent,
      borderStyle: "dashed",
      marginBottom: 12,
    },
    newTripText: { color: colors.accent, fontSize: 16, fontWeight: "600" },
    errorCard: {
      backgroundColor: colors.errorBg,
      borderRadius: 16,
      padding: 24,
      alignItems: "center",
      maxWidth: 320,
      width: "100%",
    },
    errorIcon: { marginBottom: 12 },
    errorTitle: { fontSize: 18, fontWeight: "600", color: colors.textPrimary, marginBottom: 6, textAlign: "center" },
    errorSubtitle: { fontSize: 14, color: colors.textSecondary, textAlign: "center", marginBottom: 20 },
    retryButton: {
      backgroundColor: colors.accent,
      borderRadius: 8,
      paddingVertical: 12,
      paddingHorizontal: 28,
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
    },
    retryButtonText: { color: "#fff", fontSize: 15, fontWeight: "600" },
  });

  if (authLoading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color={colors.accent} />
      </View>
    );
  }

  if (!accessToken) {
    return (
      <ScrollView style={styles.container} contentContainerStyle={styles.signInContent}>
        <Plane color={colors.accent} size={48} style={styles.welcomeIcon} />
        <Text style={styles.signInTitle}>{t("common.appName")}</Text>
        <Text style={styles.signInTagline}>{t("common.tagline")}</Text>

        <View style={styles.valueProps}>
          <Text style={styles.valueProp}>{t("home.valueProps.experts")}</Text>
          <Text style={styles.valueProp}>{t("home.valueProps.itineraries")}</Text>
          <Text style={styles.valueProp}>{t("home.valueProps.export")}</Text>
          <Text style={styles.valueProp}>{t("home.valueProps.free")}</Text>
        </View>

        <Pressable
          style={[styles.primaryButton, !authReady && styles.disabledButton]}
          onPress={signIn}
          disabled={!authReady}
        >
          <Text style={styles.buttonText}>{t("common.getStarted")}</Text>
        </Pressable>
        <Text style={styles.signInNote}>{t("home.signInNote")}</Text>
      </ScrollView>
    );
  }

  if (tripsLoading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color={colors.accent} />
      </View>
    );
  }

  if (tripsError) {
    return (
      <View style={styles.center}>
        <View style={styles.errorCard}>
          <AlertCircle color={colors.error} size={40} style={styles.errorIcon as object} />
          <Text style={styles.errorTitle} accessibilityLiveRegion="assertive">{t("trips.loadError")}</Text>
          <Text style={styles.errorSubtitle}>{t("trips.loadErrorSubtitle")}</Text>
          <Pressable
            style={styles.retryButton}
            onPress={() => void queryClient.invalidateQueries({ queryKey: ["trips"] })}
          >
            <RefreshCw color="#fff" size={16} />
            <Text style={styles.retryButtonText}>{t("common.retry")}</Text>
          </Pressable>
        </View>
      </View>
    );
  }

  if (!trips || trips.length === 0) {
    return (
      <ScrollView style={styles.container} contentContainerStyle={styles.welcomeContent}>
        <Plane color={colors.accent} size={40} style={styles.welcomeIcon} />
        <Text style={styles.welcomeTitle}>{t("home.welcomeTitle")}</Text>
        <Text style={styles.welcomeSubtitle}>{t("home.welcomeSubtitle")}</Text>

        <View style={styles.destinationList}>
          {DESTINATIONS.map((dest) => (
            <Pressable
              key={dest.key}
              style={styles.destinationCard}
              onPress={() =>
                router.push({
                  pathname: "/trips/new" as never,
                  params: { destination: dest.title },
                })
              }
              accessibilityLabel={`Start a trip to ${dest.title}`}
              accessibilityRole="button"
            >
              <Text style={styles.destinationFlag}>{dest.flag}</Text>
              <View style={styles.destinationInfo}>
                <Text style={styles.destinationName}>{dest.title}</Text>
                <Text style={styles.destinationHook}>
                  {t(`home.destinations.${dest.key}`)}
                </Text>
              </View>
              <ChevronRight color={colors.border} size={18} />
            </Pressable>
          ))}
        </View>

        <Pressable
          style={styles.primaryButton}
          onPress={() => router.push("/trips/new" as never)}
          accessibilityLabel="New Trip"
          accessibilityRole="button"
        >
          <Plus color="#fff" size={18} />
          <Text style={styles.buttonText}>{t("trips.newTrip")}</Text>
        </Pressable>
      </ScrollView>
    );
  }

  return (
    <View style={styles.container}>
      <FlatList
        data={trips}
        keyExtractor={(item) => item.id}
        renderItem={({ item }) => (
          <TripCard
            trip={item}
            onPress={() => router.push(`/trips/${item.id}` as never)}
          />
        )}
        contentContainerStyle={styles.listContent}
        ListHeaderComponent={
          <Pressable
            style={styles.newTripButton}
            onPress={() => router.push("/trips/new" as never)}
            accessibilityLabel="New Trip"
            accessibilityRole="button"
          >
            <Plus color={colors.accent} size={18} />
            <Text style={styles.newTripText}>{t("trips.newTrip")}</Text>
          </Pressable>
        }
      />
    </View>
  );
}
