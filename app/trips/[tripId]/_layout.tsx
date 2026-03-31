import { Stack } from "expo-router";
import { useTheme } from "@/lib/theme";

export default function TripDetailLayout() {
  const { colors } = useTheme();

  return (
    <Stack
      screenOptions={{
        headerStyle: { backgroundColor: colors.surface },
        headerTintColor: colors.textPrimary,
        headerTitleStyle: { fontWeight: "bold" },
        headerShadowVisible: false,
      }}
    >
      <Stack.Screen name="index" options={{ title: "Trip" }} />
      <Stack.Screen name="chat" options={{ title: "Chat" }} />
      <Stack.Screen name="bookings" options={{ title: "Bookings" }} />
      <Stack.Screen name="settings" options={{ title: "Trip Settings" }} />
    </Stack>
  );
}
