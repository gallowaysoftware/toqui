import { useState } from "react";
import {
  ActivityIndicator,
  Alert,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import { useLocalSearchParams } from "expo-router";
import { useTranslation } from "react-i18next";
import { AlertCircle, CheckCircle, Clock, Crown, Send, Users, X } from "lucide-react-native";
import { useTrip } from "@/lib/hooks/useTrips";
import {
  Collaborator,
  useCollaborators,
  useInviteCollaborator,
  useRemoveCollaborator,
} from "@/lib/hooks/useCollaborators";
import { useAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import { MemberAvatar } from "@/components/collaborators/MemberAvatar";

const MAX_COLLABORATORS = 10;

function isValidEmail(email: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim());
}

export default function TripMembersScreen() {
  const { t } = useTranslation();
  const { tripId } = useLocalSearchParams<{ tripId: string }>();
  const { colors } = useTheme();
  const { user } = useAuth();
  const { trip, isLoading: tripLoading } = useTrip(tripId!);
  const { collaborators, isLoading: collabLoading, refetch } = useCollaborators(tripId!);
  const inviteCollaborator = useInviteCollaborator();
  const removeCollaborator = useRemoveCollaborator();

  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState<"editor" | "viewer">("editor");
  const [inviteError, setInviteError] = useState<string | null>(null);
  const [inviteSuccess, setInviteSuccess] = useState(false);

  const isOwner = user != null && trip?.userId === user.id;
  const canInvite = isOwner && collaborators.length < MAX_COLLABORATORS;
  const isUnlocked = trip?.isUnlocked ?? false;

  // Owner is implicit on the trip object — surface it as the first row so the
  // members screen always shows everyone, even before any invites go out.
  const allMembers: Collaborator[] = (() => {
    const hasOwnerRow = collaborators.some((c) => c.role === "owner");
    if (hasOwnerRow || !trip) return collaborators;
    return [
      {
        id: `owner-${trip.userId}`,
        email: isOwner && user ? user.email : t("collaborators.role.owner"),
        role: "owner",
        invitedAt: trip.createdAt?.toString() ?? "",
        acceptedAt: trip.createdAt?.toString() ?? "",
        userId: trip.userId,
      },
      ...collaborators,
    ];
  })();

  const handleInvite = async () => {
    setInviteError(null);
    setInviteSuccess(false);
    if (!inviteEmail || !isValidEmail(inviteEmail)) {
      setInviteError(t("collaborators.inviteError"));
      return;
    }
    try {
      await inviteCollaborator.mutateAsync({
        tripId: tripId!,
        email: inviteEmail.trim(),
        role: inviteRole,
      });
      setInviteEmail("");
      setInviteSuccess(true);
      void refetch();
      setTimeout(() => setInviteSuccess(false), 3000);
    } catch (err) {
      setInviteError(
        err instanceof Error ? err.message : t("collaborators.inviteError"),
      );
    }
  };

  const handleRemove = (collab: Collaborator) => {
    Alert.alert(
      t("collaborators.removeTitle"),
      t("collaborators.removeConfirm", { email: collab.email }),
      [
        { text: t("common.cancel"), style: "cancel" },
        {
          text: t("common.remove"),
          style: "destructive",
          onPress: async () => {
            try {
              await removeCollaborator.mutateAsync({ tripId: tripId!, email: collab.email });
              void refetch();
            } catch {
              // Silently ignore — UI will refetch on next mount
            }
          },
        },
      ],
    );
  };

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surfaceSecondary },
    content: { padding: 16, paddingBottom: 48 },
    center: { flex: 1, justifyContent: "center", alignItems: "center", padding: 32 },
    header: { flexDirection: "row", alignItems: "center", marginBottom: 8 },
    headerTitle: { fontSize: 22, fontWeight: "700", color: colors.textPrimary, marginLeft: 10 },
    subtitle: { fontSize: 14, color: colors.textSecondary, marginBottom: 20 },
    proBadge: {
      flexDirection: "row",
      alignItems: "center",
      backgroundColor: colors.accentSoft,
      borderRadius: 10,
      paddingVertical: 10,
      paddingHorizontal: 12,
      marginBottom: 20,
      gap: 10,
    },
    proBadgeText: {
      flex: 1,
      color: colors.textPrimary,
      fontSize: 13,
      lineHeight: 18,
    },
    avatarStrip: {
      flexDirection: "row",
      flexWrap: "wrap",
      gap: 8,
      marginBottom: 24,
    },
    section: {
      backgroundColor: colors.surface,
      borderRadius: 12,
      borderWidth: 1,
      borderColor: colors.border,
      marginBottom: 20,
      overflow: "hidden",
    },
    sectionHeader: {
      paddingHorizontal: 16,
      paddingVertical: 12,
      borderBottomWidth: 1,
      borderBottomColor: colors.border,
    },
    sectionTitle: { fontSize: 14, fontWeight: "600", color: colors.textPrimary },
    memberRow: {
      flexDirection: "row",
      alignItems: "center",
      paddingHorizontal: 16,
      paddingVertical: 14,
      borderBottomWidth: 1,
      borderBottomColor: colors.border,
      gap: 12,
    },
    memberRowLast: { borderBottomWidth: 0 },
    memberInfo: { flex: 1, minWidth: 0 },
    memberEmail: {
      fontSize: 15,
      color: colors.textPrimary,
      fontWeight: "600",
    },
    memberMeta: {
      fontSize: 12,
      color: colors.textSecondary,
      marginTop: 2,
    },
    badgeRow: { flexDirection: "row", alignItems: "center", gap: 6 },
    roleBadge: {
      paddingHorizontal: 8,
      paddingVertical: 3,
      borderRadius: 999,
      backgroundColor: colors.surfaceTertiary,
    },
    roleBadgeOwner: { backgroundColor: colors.accentSoft },
    roleBadgeText: {
      fontSize: 11,
      fontWeight: "700",
      color: colors.textSecondary,
      textTransform: "uppercase",
      letterSpacing: 0.5,
    },
    roleBadgeTextOwner: { color: colors.accent },
    pendingBadge: {
      flexDirection: "row",
      alignItems: "center",
      gap: 4,
      paddingHorizontal: 8,
      paddingVertical: 3,
      borderRadius: 999,
      backgroundColor: colors.warningBg,
    },
    pendingText: { fontSize: 11, fontWeight: "600", color: colors.warning },
    removeButton: { padding: 6, marginLeft: 4 },
    inviteSection: {
      backgroundColor: colors.surface,
      borderRadius: 12,
      borderWidth: 1,
      borderColor: colors.border,
      padding: 16,
      marginBottom: 20,
    },
    inviteTitle: { fontSize: 14, fontWeight: "600", color: colors.textPrimary, marginBottom: 12 },
    inviteRow: { flexDirection: "row", gap: 8, alignItems: "center" },
    input: {
      flex: 1,
      backgroundColor: colors.inputBg,
      borderWidth: 1,
      borderColor: colors.inputBorder,
      borderRadius: 8,
      paddingHorizontal: 12,
      paddingVertical: 10,
      fontSize: 14,
      color: colors.textPrimary,
    },
    roleToggleRow: { flexDirection: "row", gap: 8, marginTop: 10 },
    roleToggle: {
      flex: 1,
      paddingVertical: 10,
      borderRadius: 8,
      borderWidth: 1,
      borderColor: colors.border,
      alignItems: "center",
      backgroundColor: colors.surface,
    },
    roleToggleActive: {
      borderColor: colors.accent,
      backgroundColor: colors.accentSoft,
    },
    roleToggleText: { fontSize: 13, color: colors.textSecondary, fontWeight: "600" },
    roleToggleTextActive: { color: colors.accent },
    sendButton: {
      backgroundColor: colors.accent,
      borderRadius: 8,
      width: 44,
      height: 44,
      alignItems: "center",
      justifyContent: "center",
    },
    sendButtonDisabled: { opacity: 0.5 },
    feedback: {
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
      marginTop: 12,
      padding: 10,
      borderRadius: 8,
    },
    feedbackError: { backgroundColor: colors.errorBg },
    feedbackSuccess: { backgroundColor: colors.successBg },
    feedbackText: { fontSize: 13, flex: 1 },
    feedbackTextError: { color: colors.error },
    feedbackTextSuccess: { color: colors.success },
    maxNotice: { fontSize: 12, color: colors.textTertiary, marginTop: 10, textAlign: "center" },
    emptyText: {
      fontSize: 14,
      color: colors.textSecondary,
      textAlign: "center",
      paddingHorizontal: 16,
      paddingVertical: 24,
    },
  });

  if (tripLoading || collabLoading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator color={colors.accent} />
      </View>
    );
  }

  if (!trip) {
    return (
      <View style={styles.center}>
        <Text style={styles.emptyText}>{t("collaborators.title")}</Text>
      </View>
    );
  }

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <View style={styles.header}>
        <Users size={22} color={colors.textPrimary} />
        <Text style={styles.headerTitle}>{t("collaborators.title")}</Text>
      </View>
      <Text style={styles.subtitle}>
        {trip.title}
      </Text>

      {isUnlocked && allMembers.length > 1 && (
        <View style={styles.proBadge}>
          <Crown size={18} color={colors.accent} />
          <Text style={styles.proBadgeText}>{t("collaborators.proAppliesAll")}</Text>
        </View>
      )}

      <View style={styles.avatarStrip}>
        {allMembers.map((m) => (
          <MemberAvatar key={m.id} identity={m.email} size={42} />
        ))}
      </View>

      <View style={styles.section}>
        <View style={styles.sectionHeader}>
          <Text style={styles.sectionTitle}>
            {t("collaborators.title")} ({allMembers.length})
          </Text>
        </View>
        {allMembers.length === 0 ? (
          <Text style={styles.emptyText}>{t("collaborators.emptyState")}</Text>
        ) : (
          allMembers.map((m, idx) => {
            const last = idx === allMembers.length - 1;
            const isPending = m.role !== "owner" && !m.acceptedAt;
            const isOwnerRow = m.role === "owner";
            const canRemove = isOwner && !isOwnerRow;
            return (
              <View key={m.id} style={[styles.memberRow, last && styles.memberRowLast]}>
                <MemberAvatar identity={m.email} size={40} />
                <View style={styles.memberInfo}>
                  <Text style={styles.memberEmail} numberOfLines={1}>
                    {m.email}
                  </Text>
                  <View style={[styles.badgeRow, { marginTop: 4 }]}>
                    <View style={[styles.roleBadge, isOwnerRow && styles.roleBadgeOwner]}>
                      <Text
                        style={[
                          styles.roleBadgeText,
                          isOwnerRow && styles.roleBadgeTextOwner,
                        ]}
                      >
                        {t(`collaborators.role.${m.role}`)}
                      </Text>
                    </View>
                    {isPending && (
                      <View style={styles.pendingBadge}>
                        <Clock size={11} color={colors.warning} />
                        <Text style={styles.pendingText}>{t("collaborators.pending")}</Text>
                      </View>
                    )}
                  </View>
                </View>
                {canRemove && (
                  <Pressable
                    onPress={() => handleRemove(m)}
                    style={styles.removeButton}
                    accessibilityLabel={t("collaborators.removeTitle")}
                  >
                    <X size={18} color={colors.textTertiary} />
                  </Pressable>
                )}
              </View>
            );
          })
        )}
      </View>

      {isOwner && (
        <View style={styles.inviteSection}>
          <Text style={styles.inviteTitle}>{t("collaborators.invite")}</Text>
          <View style={styles.inviteRow}>
            <TextInput
              value={inviteEmail}
              onChangeText={setInviteEmail}
              placeholder={t("collaborators.emailPlaceholder")}
              placeholderTextColor={colors.textTertiary}
              autoCapitalize="none"
              autoCorrect={false}
              keyboardType="email-address"
              editable={canInvite && !inviteCollaborator.isPending}
              style={styles.input}
            />
            <Pressable
              onPress={handleInvite}
              disabled={!canInvite || inviteCollaborator.isPending}
              style={[
                styles.sendButton,
                (!canInvite || inviteCollaborator.isPending) && styles.sendButtonDisabled,
              ]}
              accessibilityLabel={t("collaborators.invite")}
            >
              {inviteCollaborator.isPending ? (
                <ActivityIndicator color="#fff" size="small" />
              ) : (
                <Send size={18} color="#fff" />
              )}
            </Pressable>
          </View>
          <View style={styles.roleToggleRow}>
            <Pressable
              onPress={() => setInviteRole("editor")}
              style={[styles.roleToggle, inviteRole === "editor" && styles.roleToggleActive]}
            >
              <Text
                style={[
                  styles.roleToggleText,
                  inviteRole === "editor" && styles.roleToggleTextActive,
                ]}
              >
                {t("collaborators.role.editor")}
              </Text>
            </Pressable>
            <Pressable
              onPress={() => setInviteRole("viewer")}
              style={[styles.roleToggle, inviteRole === "viewer" && styles.roleToggleActive]}
            >
              <Text
                style={[
                  styles.roleToggleText,
                  inviteRole === "viewer" && styles.roleToggleTextActive,
                ]}
              >
                {t("collaborators.role.viewer")}
              </Text>
            </Pressable>
          </View>
          {inviteError && (
            <View style={[styles.feedback, styles.feedbackError]}>
              <AlertCircle size={16} color={colors.error} />
              <Text style={[styles.feedbackText, styles.feedbackTextError]}>{inviteError}</Text>
            </View>
          )}
          {inviteSuccess && (
            <View style={[styles.feedback, styles.feedbackSuccess]}>
              <CheckCircle size={16} color={colors.success} />
              <Text style={[styles.feedbackText, styles.feedbackTextSuccess]}>
                {t("collaborators.inviteSent")}
              </Text>
            </View>
          )}
          {!canInvite && (
            <Text style={styles.maxNotice}>{t("collaborators.maxReached")}</Text>
          )}
        </View>
      )}
    </ScrollView>
  );
}
