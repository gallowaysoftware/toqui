import React from "react";
import { StyleSheet, Text, View, ViewStyle } from "react-native";
import { useTheme } from "@/lib/theme";

interface MemberAvatarProps {
  /** Email or display name used for initials + deterministic color */
  identity: string;
  /** Optional explicit display name; falls back to identity */
  name?: string | null;
  /** Avatar diameter in pixels. Default 36. */
  size?: number;
  /** When true, draws a 1px ring in the surface color so stacked avatars overlap cleanly */
  withRing?: boolean;
  style?: ViewStyle;
}

// 8 well-spaced hues so neighboring members are visually distinct.
const PALETTE = [
  "#E8654A", // toqui orange
  "#3B82F6", // blue
  "#10B981", // emerald
  "#8B5CF6", // violet
  "#F59E0B", // amber
  "#EC4899", // pink
  "#14B8A6", // teal
  "#6366F1", // indigo
];

function hashIdentity(s: string): number {
  // FNV-1a 32-bit, deterministic across platforms.
  let h = 0x811c9dc5;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = (h + ((h << 1) + (h << 4) + (h << 7) + (h << 8) + (h << 24))) >>> 0;
  }
  return h >>> 0;
}

function initialsFor(name: string | null | undefined, identity: string): string {
  const source = (name && name.trim()) || identity;
  if (!source) return "?";
  // If it's an email, take the local part.
  const localPart = source.includes("@") ? source.split("@")[0] : source;
  const cleaned = localPart.replace(/[^a-zA-Z0-9 .-]/g, " ").trim();
  if (!cleaned) return source[0]?.toUpperCase() || "?";
  const parts = cleaned.split(/[\s.\-_]+/).filter(Boolean);
  if (parts.length === 0) return cleaned[0].toUpperCase();
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

/**
 * MemberAvatar renders a circle with the member's initials, colored
 * deterministically from the identity hash. No avatar URLs are fetched —
 * we never collect or share third-party user images for privacy reasons.
 */
export function MemberAvatar({
  identity,
  name,
  size = 36,
  withRing = false,
  style,
}: MemberAvatarProps) {
  const { colors } = useTheme();
  const bg = PALETTE[hashIdentity(identity) % PALETTE.length];
  const initials = initialsFor(name, identity);
  return (
    <View
      style={[
        styles.base,
        {
          width: size,
          height: size,
          borderRadius: size / 2,
          backgroundColor: bg,
          borderColor: colors.surface,
          borderWidth: withRing ? 2 : 0,
        },
        style,
      ]}
      accessibilityLabel={name || identity}
    >
      <Text
        style={{
          color: "#ffffff",
          fontSize: Math.max(11, Math.round(size * 0.4)),
          fontWeight: "700",
          letterSpacing: 0.3,
        }}
      >
        {initials}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  base: {
    alignItems: "center",
    justifyContent: "center",
  },
});
