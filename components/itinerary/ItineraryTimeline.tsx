import { View, Text, StyleSheet } from "react-native";
import { MapPin, Clock, Utensils, Ticket, Hotel, Plane, Camera } from "lucide-react-native";
import { useTheme } from "@/lib/theme";
import type { Itinerary, ItineraryDay, ItineraryItem } from "@gen/toqui/v1/trip_pb";

// Day colors cycle — same palette as the original map markers
const dayColors = [
  "#e8654a", "#3b82f6", "#22c55e", "#f59e0b", "#8b5cf6",
  "#ec4899", "#06b6d4", "#ef4444", "#10b981", "#6366f1",
];

function getDayColor(index: number): string {
  return dayColors[index % dayColors.length]!;
}

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

function getIcon(type: string) {
  const lower = type.toLowerCase();
  for (const [key, Icon] of Object.entries(typeIcons)) {
    if (lower.includes(key)) return Icon;
  }
  return MapPin;
}

function TimelineItem({ item, color }: { item: ItineraryItem; color: string }) {
  const { colors } = useTheme();
  const Icon = getIcon(item.type);
  const time = item.startTime
    ? new Date(Number(item.startTime.seconds) * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
    : null;

  return (
    <View style={styles.itemRow}>
      <View style={styles.itemTimeline}>
        <View style={[styles.itemDot, { backgroundColor: color }]}>
          <Icon color="#fff" size={10} />
        </View>
        <View style={[styles.itemLine, { backgroundColor: colors.border }]} />
      </View>
      <View style={[styles.itemContent, { backgroundColor: colors.surface, borderColor: colors.border }]}>
        <View style={styles.itemHeader}>
          <Text style={[styles.itemTitle, { color: colors.textPrimary }]} numberOfLines={1}>
            {item.title}
          </Text>
          {time && (
            <View style={styles.timeChip}>
              <Clock color={colors.textTertiary} size={11} />
              <Text style={[styles.timeText, { color: colors.textTertiary }]}>{time}</Text>
            </View>
          )}
        </View>
        {item.description ? (
          <Text style={[styles.itemDescription, { color: colors.textSecondary }]} numberOfLines={2}>
            {item.description}
          </Text>
        ) : null}
        {item.type ? (
          <Text style={[styles.itemType, { color }]}>{item.type}</Text>
        ) : null}
      </View>
    </View>
  );
}

function DaySection({ day, index }: { day: ItineraryDay; index: number }) {
  const { colors } = useTheme();
  const color = getDayColor(index);

  return (
    <View style={styles.daySection}>
      <View style={[styles.dayHeader, { borderColor: color }]}>
        <View style={[styles.dayBadge, { backgroundColor: color }]}>
          <Text style={styles.dayBadgeText}>Day {day.dayNumber}</Text>
        </View>
        {day.date ? (
          <Text style={[styles.dayDate, { color: colors.textSecondary }]}>{day.date}</Text>
        ) : null}
        {day.title ? (
          <Text style={[styles.dayTitle, { color: colors.textPrimary }]}>{day.title}</Text>
        ) : null}
      </View>
      {day.items.length > 0 ? (
        day.items
          .slice()
          .sort((a, b) => a.orderInDay - b.orderInDay)
          .map((item) => <TimelineItem key={item.id} item={item} color={color} />)
      ) : (
        <Text style={[styles.emptyDay, { color: colors.textTertiary }]}>No items yet</Text>
      )}
    </View>
  );
}

interface ItineraryTimelineProps {
  itinerary: Itinerary;
}

export function ItineraryTimeline({ itinerary }: ItineraryTimelineProps) {
  const { colors } = useTheme();

  if (!itinerary.days.length) {
    return (
      <View style={styles.emptyContainer}>
        <Text style={[styles.emptyText, { color: colors.textSecondary }]}>
          No itinerary yet. Chat with the AI to start building one.
        </Text>
      </View>
    );
  }

  return (
    <View>
      {itinerary.days
        .slice()
        .sort((a, b) => a.dayNumber - b.dayNumber)
        .map((day, i) => (
          <DaySection key={day.id} day={day} index={i} />
        ))}
    </View>
  );
}

const styles = StyleSheet.create({
  daySection: { marginBottom: 20 },
  dayHeader: { flexDirection: "row", alignItems: "center", gap: 10, marginBottom: 10, borderLeftWidth: 3, paddingLeft: 10 },
  dayBadge: { paddingHorizontal: 10, paddingVertical: 4, borderRadius: 12 },
  dayBadgeText: { color: "#fff", fontSize: 12, fontWeight: "700" },
  dayDate: { fontSize: 13 },
  dayTitle: { fontSize: 14, fontWeight: "500", flex: 1 },
  emptyDay: { fontSize: 13, fontStyle: "italic", paddingLeft: 34, marginBottom: 8 },
  itemRow: { flexDirection: "row", marginBottom: 2 },
  itemTimeline: { width: 30, alignItems: "center" },
  itemDot: { width: 22, height: 22, borderRadius: 11, justifyContent: "center", alignItems: "center", zIndex: 1 },
  itemLine: { width: 2, flex: 1, marginTop: -2 },
  itemContent: {
    flex: 1,
    borderRadius: 10,
    padding: 10,
    marginLeft: 6,
    marginBottom: 6,
    borderWidth: 1,
  },
  itemHeader: { flexDirection: "row", alignItems: "center", justifyContent: "space-between" },
  itemTitle: { fontSize: 14, fontWeight: "600", flex: 1 },
  timeChip: { flexDirection: "row", alignItems: "center", gap: 3, marginLeft: 8 },
  timeText: { fontSize: 11 },
  itemDescription: { fontSize: 13, marginTop: 4, lineHeight: 18 },
  itemType: { fontSize: 11, fontWeight: "500", marginTop: 4, textTransform: "capitalize" },
  emptyContainer: { padding: 20, alignItems: "center" },
  emptyText: { fontSize: 14, textAlign: "center" },
});
