import { View, Text, TextInput, StyleSheet, Pressable, Platform, ScrollView, Alert, ActivityIndicator } from "react-native";
import { useState, useEffect } from "react";
import * as Clipboard from "expo-clipboard";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { AlertCircle, RefreshCw, CheckCircle, Clock, Copy, X, Users, Send } from "lucide-react-native";
import { useTrip, useUpdateTrip, useDeleteTrip } from "@/lib/hooks/useTrips";
import { useCollaborators, useInviteCollaborator, useRemoveCollaborator } from "@/lib/hooks/useCollaborators";
import { DatePicker } from "@/components/DatePicker";
import { useTheme } from "@/lib/theme";
import { useAuth } from "@/lib/auth";

export default function TripSettingsScreen() {
  const { tripId } = useLocalSearchParams<{ tripId: string }>();
  const { t } = useTranslation();
  const router = useRouter();
  const { colors } = useTheme();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const { trip, isLoading, error: tripError } = useTrip(tripId!);
  const updateTrip = useUpdateTrip();
  const deleteTrip = useDeleteTrip();

  const { collaborators, isLoading: collabLoading } = useCollaborators(tripId!);
  const inviteCollaborator = useInviteCollaborator();
  const removeCollaborator = useRemoveCollaborator();

  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState<"editor" | "viewer">("editor");
  const [inviteError, setInviteError] = useState<string | null>(null);
  const [inviteSuccess, setInviteSuccess] = useState(false);
  const [emailFallbackUrl, setEmailFallbackUrl] = useState<string | null>(null);
  const [linkCopied, setLinkCopied] = useState(false);

  const handleCopyFallbackLink = async () => {
    if (!emailFallbackUrl) return;
    try {
      if (Platform.OS === "web") {
        await navigator.clipboard.writeText(emailFallbackUrl);
      } else {
        await Clipboard.setStringAsync(emailFallbackUrl);
      }
      setLinkCopied(true);
      setTimeout(() => setLinkCopied(false), 2000);
    } catch {
      // best-effort; the URL stays selectable in the UI as a fallback
    }
  };

  const MAX_COLLABORATORS = 10;
  const isOwner = user != null && trip?.userId === user.id;
  const canInvite = collaborators.length < MAX_COLLABORATORS;

  useEffect(() => {
    if (trip) {
      setTitle(trip.title);
      setDescription(trip.description);
      setStartDate(trip.startDate);
      setEndDate(trip.endDate);
    }
  }, [trip]);

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surfaceSecondary },
    content: { padding: 16 },
    center: { flex: 1, justifyContent: "center", alignItems: "center" },
    label: { fontSize: 14, fontWeight: "600", color: colors.textPrimary, marginBottom: 6, marginTop: 16 },
    input: {
      backgroundColor: colors.inputBg,
      borderWidth: 1,
      borderColor: colors.inputBorder,
      borderRadius: 8,
      padding: 12,
      fontSize: 15,
      color: colors.textPrimary,
    },
    textArea: { minHeight: 80, textAlignVertical: "top" },
    dateRow: { flexDirection: "row", gap: 12 },
    dateField: { flex: 1 },
    saveButton: {
      backgroundColor: colors.accent,
      borderRadius: 8,
      padding: 14,
      alignItems: "center",
      marginTop: 24,
    },
    disabledButton: { opacity: 0.5 },
    saveText: { color: "#fff", fontSize: 16, fontWeight: "600" },
    dangerZone: {
      marginTop: 40,
      padding: 16,
      backgroundColor: colors.surface,
      borderRadius: 12,
      borderWidth: 1,
      borderColor: colors.error,
    },
    dangerTitle: { fontSize: 16, fontWeight: "600", color: colors.error, marginBottom: 8 },
    dangerWarning: { fontSize: 14, color: colors.textSecondary, marginBottom: 16, lineHeight: 20 },
    deleteButton: {
      borderWidth: 1,
      borderColor: colors.error,
      borderRadius: 8,
      padding: 12,
      alignItems: "center",
    },
    deleteText: { color: colors.error, fontWeight: "600" },
    feedbackBanner: {
      flexDirection: "row",
      alignItems: "center",
      gap: 8,
      borderRadius: 10,
      padding: 12,
      marginTop: 12,
    },
    successBanner: { backgroundColor: colors.successBg },
    errorBanner: { backgroundColor: colors.errorBg },
    feedbackText: { fontSize: 14, fontWeight: "500", flex: 1 },
    successText: { color: colors.success },
    errorText: { color: colors.error },
    fallback: {
      marginTop: 12,
      padding: 12,
      borderRadius: 8,
      backgroundColor: colors.warningBg,
      borderWidth: 1,
      borderColor: colors.warningBorder,
      gap: 8,
    },
    fallbackHeader: { flexDirection: "row", alignItems: "center", gap: 8 },
    fallbackText: { fontSize: 13, color: colors.textPrimary, flex: 1 },
    fallbackUrl: {
      fontSize: 12,
      color: colors.textSecondary,
      fontFamily: Platform.select({ ios: "Menlo", android: "monospace", default: "monospace" }),
      backgroundColor: colors.surface,
      padding: 8,
      borderRadius: 6,
    },
    copyButton: {
      flexDirection: "row",
      alignItems: "center",
      gap: 6,
      alignSelf: "flex-start",
      paddingVertical: 6,
      paddingHorizontal: 10,
      borderRadius: 6,
      backgroundColor: colors.accentSoft,
    },
    copyButtonText: { fontSize: 13, color: colors.accent, fontWeight: "600" },
    errorContainer: {
      flex: 1,
      justifyContent: "center",
      alignItems: "center",
      padding: 24,
      backgroundColor: colors.surfaceSecondary,
    },
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
    teamSection: {
      marginTop: 32,
      padding: 16,
      backgroundColor: colors.surface,
      borderRadius: 12,
      borderWidth: 1,
      borderColor: colors.border,
    },
    teamHeader: { flexDirection: "row", alignItems: "center", gap: 8, marginBottom: 16 },
    teamTitle: { fontSize: 16, fontWeight: "600", color: colors.textPrimary },
    teamCount: { fontSize: 13, color: colors.textSecondary },
    collaboratorRow: {
      flexDirection: "row",
      alignItems: "center",
      paddingVertical: 10,
      borderBottomWidth: 1,
      borderBottomColor: colors.border,
    },
    collaboratorInfo: { flex: 1 },
    collaboratorEmail: { fontSize: 14, color: colors.textPrimary },
    collaboratorMeta: { flexDirection: "row", alignItems: "center", gap: 6, marginTop: 2 },
    roleBadge: {
      paddingHorizontal: 6,
      paddingVertical: 1,
      borderRadius: 6,
      backgroundColor: colors.surfaceTertiary,
    },
    roleBadgeOwner: { backgroundColor: colors.accentSoft },
    roleBadgeText: { fontSize: 11, fontWeight: "600", color: colors.textSecondary, textTransform: "capitalize" },
    roleBadgeOwnerText: { color: colors.accent },
    pendingBadge: { flexDirection: "row", alignItems: "center", gap: 3 },
    pendingText: { fontSize: 11, color: colors.textTertiary },
    removeButton: { padding: 6 },
    inviteForm: { marginTop: 16, gap: 8 },
    inviteRow: { flexDirection: "row", gap: 8, alignItems: "flex-end" },
    inviteEmailInput: {
      flex: 1,
      backgroundColor: colors.inputBg,
      borderWidth: 1,
      borderColor: colors.inputBorder,
      borderRadius: 8,
      padding: 10,
      fontSize: 14,
      color: colors.textPrimary,
    },
    roleSelector: { flexDirection: "row", gap: 4, marginBottom: 8 },
    roleOption: {
      paddingHorizontal: 12,
      paddingVertical: 6,
      borderRadius: 6,
      borderWidth: 1,
      borderColor: colors.border,
    },
    roleOptionActive: { backgroundColor: colors.accent, borderColor: colors.accent },
    roleOptionText: { fontSize: 13, color: colors.textSecondary },
    roleOptionActiveText: { color: "#fff", fontWeight: "600" },
    inviteSendButton: {
      backgroundColor: colors.accent,
      borderRadius: 8,
      padding: 10,
      alignItems: "center",
      justifyContent: "center",
    },
    inviteDisabled: { opacity: 0.5 },
    maxCollabNote: { fontSize: 12, color: colors.textTertiary, marginTop: 4 },
  });

  if (isLoading) {
    return <View style={styles.center}><ActivityIndicator size="large" color={colors.accent} /></View>;
  }

  if (tripError || !trip) {
    return (
      <View style={styles.errorContainer}>
        <View style={styles.errorCard}>
          <AlertCircle color={colors.error} size={40} style={styles.errorIcon as object} />
          <Text style={styles.errorTitle}>{t("tripSettings.loadError")}</Text>
          <Text style={styles.errorSubtitle}>{t("tripSettings.loadErrorSubtitle")}</Text>
          <Pressable
            style={styles.retryButton}
            onPress={() => void queryClient.invalidateQueries({ queryKey: ["trip", tripId] })}
          >
            <RefreshCw color="#fff" size={16} />
            <Text style={styles.retryButtonText}>{t("common.retry")}</Text>
          </Pressable>
        </View>
      </View>
    );
  }

  const handleSave = async () => {
    setSaveSuccess(false);
    setSaveError(null);
    try {
      await updateTrip.mutateAsync({
        id: tripId!,
        title: title.trim(),
        description: description.trim(),
        startDate,
        endDate,
      });
      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 3000);
    } catch {
      setSaveError(t("tripSettings.saveError"));
    }
  };

  const handleDelete = () => {
    Alert.alert(
      t("tripSettings.deleteTrip"),
      t("tripSettings.deleteWarning"),
      [
        { text: t("common.cancel"), style: "cancel" },
        {
          text: t("tripSettings.deleteConfirm"),
          style: "destructive",
          onPress: async () => {
            try {
              await deleteTrip.mutateAsync(tripId!);
              router.replace("/(tabs)" as never);
            } catch {
              Alert.alert(t("common.error"), t("tripSettings.deleteError"));
            }
          },
        },
      ],
    );
  };

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <Text style={styles.label}>{t("tripSettings.editTitle")}</Text>
      <TextInput style={styles.input} value={title} onChangeText={setTitle} accessibilityLabel="Trip title" />

      <Text style={styles.label}>{t("tripSettings.editDescription")}</Text>
      <TextInput style={[styles.input, styles.textArea]} value={description} onChangeText={setDescription} multiline accessibilityLabel="Trip description" />

      <View style={styles.dateRow}>
        <View style={styles.dateField}>
          <DatePicker
            label={t("tripSettings.editStartDate")}
            value={startDate}
            onChange={setStartDate}
            placeholder="YYYY-MM-DD"
          />
        </View>
        <View style={styles.dateField}>
          <DatePicker
            label={t("tripSettings.editEndDate")}
            value={endDate}
            onChange={setEndDate}
            placeholder="YYYY-MM-DD"
          />
        </View>
      </View>

      <Pressable
        style={[styles.saveButton, updateTrip.isPending && styles.disabledButton]}
        onPress={handleSave}
        disabled={updateTrip.isPending}
      >
        <Text style={styles.saveText}>
          {updateTrip.isPending ? t("tripSettings.saving") : t("tripSettings.save")}
        </Text>
      </Pressable>

      {saveSuccess && (
        <View style={[styles.feedbackBanner, styles.successBanner]}>
          <CheckCircle color={colors.success} size={16} />
          <Text style={[styles.feedbackText, styles.successText]}>{t("tripSettings.saved")}</Text>
        </View>
      )}

      {saveError && (
        <View style={[styles.feedbackBanner, styles.errorBanner]}>
          <AlertCircle color={colors.error} size={16} />
          <Text style={[styles.feedbackText, styles.errorText]}>{saveError}</Text>
        </View>
      )}

      {/* Team / Collaborators Section */}
      <View style={styles.teamSection}>
        <View style={styles.teamHeader}>
          <Users color={colors.textPrimary} size={18} />
          <Text style={styles.teamTitle}>{t("collaborators.title")}</Text>
          <Text style={styles.teamCount}>({collaborators.length}/{MAX_COLLABORATORS})</Text>
        </View>

        {collabLoading ? (
          <ActivityIndicator size="small" color={colors.accent} />
        ) : (
          <>
            {collaborators.map((collab) => (
              <View key={collab.id} style={styles.collaboratorRow}>
                <View style={styles.collaboratorInfo}>
                  <Text style={styles.collaboratorEmail}>{collab.email}</Text>
                  <View style={styles.collaboratorMeta}>
                    <View style={[styles.roleBadge, collab.role === "owner" && styles.roleBadgeOwner]}>
                      <Text style={[styles.roleBadgeText, collab.role === "owner" && styles.roleBadgeOwnerText]}>
                        {t(`collaborators.role.${collab.role}`)}
                      </Text>
                    </View>
                    {!collab.acceptedAt && (
                      <View style={styles.pendingBadge}>
                        <Clock color={colors.textTertiary} size={10} />
                        <Text style={styles.pendingText}>{t("collaborators.pending")}</Text>
                      </View>
                    )}
                  </View>
                </View>
                {isOwner && collab.role !== "owner" && (
                  <Pressable
                    style={styles.removeButton}
                    onPress={() => {
                      Alert.alert(
                        t("collaborators.removeTitle"),
                        t("collaborators.removeConfirm", { email: collab.email }),
                        [
                          { text: t("common.cancel"), style: "cancel" },
                          {
                            text: t("common.delete"),
                            style: "destructive",
                            onPress: () => {
                              void removeCollaborator.mutateAsync({ tripId: tripId!, email: collab.email });
                            },
                          },
                        ],
                      );
                    }}
                    accessibilityLabel={`Remove ${collab.email}`}
                    accessibilityRole="button"
                  >
                    <X color={colors.textTertiary} size={16} />
                  </Pressable>
                )}
              </View>
            ))}

            {isOwner && canInvite && (
              <View style={styles.inviteForm}>
                <View style={styles.roleSelector}>
                  {(["editor", "viewer"] as const).map((role) => (
                    <Pressable
                      key={role}
                      style={[styles.roleOption, inviteRole === role && styles.roleOptionActive]}
                      onPress={() => setInviteRole(role)}
                      accessibilityLabel={`Select role ${role}`}
                      accessibilityRole="button"
                    >
                      <Text style={[styles.roleOptionText, inviteRole === role && styles.roleOptionActiveText]}>
                        {t(`collaborators.role.${role}`)}
                      </Text>
                    </Pressable>
                  ))}
                </View>
                <View style={styles.inviteRow}>
                  <TextInput
                    style={styles.inviteEmailInput}
                    value={inviteEmail}
                    onChangeText={setInviteEmail}
                    placeholder={t("collaborators.emailPlaceholder")}
                    placeholderTextColor={colors.textTertiary}
                    keyboardType="email-address"
                    autoCapitalize="none"
                    autoCorrect={false}
                    accessibilityLabel="Collaborator email"
                  />
                  <Pressable
                    style={[styles.inviteSendButton, (inviteCollaborator.isPending || !inviteEmail.trim()) && styles.inviteDisabled]}
                    onPress={async () => {
                      setInviteError(null);
                      setInviteSuccess(false);
                      setEmailFallbackUrl(null);
                      setLinkCopied(false);
                      try {
                        const result = await inviteCollaborator.mutateAsync({
                          tripId: tripId!,
                          email: inviteEmail.trim(),
                          role: inviteRole,
                        });
                        setInviteEmail("");
                        if (result.emailSent) {
                          setInviteSuccess(true);
                          setTimeout(() => setInviteSuccess(false), 3000);
                        } else if (result.acceptUrl) {
                          setEmailFallbackUrl(result.acceptUrl);
                        } else {
                          setInviteSuccess(true);
                          setTimeout(() => setInviteSuccess(false), 3000);
                        }
                      } catch (err) {
                        setInviteError(err instanceof Error ? err.message : t("collaborators.inviteError"));
                      }
                    }}
                    disabled={inviteCollaborator.isPending || !inviteEmail.trim()}
                    accessibilityLabel={t("collaborators.invite")}
                    accessibilityRole="button"
                  >
                    <Send color="#fff" size={16} />
                  </Pressable>
                </View>
                {inviteSuccess && (
                  <View style={[styles.feedbackBanner, styles.successBanner]}>
                    <CheckCircle color={colors.success} size={14} />
                    <Text style={[styles.feedbackText, styles.successText]}>{t("collaborators.inviteSent")}</Text>
                  </View>
                )}
                {inviteError && (
                  <View style={[styles.feedbackBanner, styles.errorBanner]}>
                    <AlertCircle color={colors.error} size={14} />
                    <Text style={[styles.feedbackText, styles.errorText]}>{inviteError}</Text>
                  </View>
                )}
                {emailFallbackUrl && (
                  <View style={styles.fallback}>
                    <View style={styles.fallbackHeader}>
                      <AlertCircle size={16} color={colors.warning} />
                      <Text style={styles.fallbackText}>
                        {t("collaborators.inviteEmailFallback")}
                      </Text>
                    </View>
                    <Text style={styles.fallbackUrl} selectable numberOfLines={2}>
                      {emailFallbackUrl}
                    </Text>
                    <Pressable
                      onPress={() => void handleCopyFallbackLink()}
                      style={styles.copyButton}
                      accessibilityRole="button"
                      accessibilityLabel={t("collaborators.copyLink")}
                    >
                      <Copy size={14} color={colors.accent} />
                      <Text style={styles.copyButtonText}>
                        {linkCopied ? t("collaborators.linkCopied") : t("collaborators.copyLink")}
                      </Text>
                    </Pressable>
                  </View>
                )}
              </View>
            )}

            {isOwner && !canInvite && (
              <Text style={styles.maxCollabNote}>{t("collaborators.maxReached")}</Text>
            )}
          </>
        )}
      </View>

      <View style={styles.dangerZone}>
        <Text style={styles.dangerTitle}>{t("tripSettings.deleteTrip")}</Text>
        <Text style={styles.dangerWarning}>{t("tripSettings.deleteWarning")}</Text>
        <Pressable
          style={[styles.deleteButton, deleteTrip.isPending && styles.disabledButton]}
          onPress={handleDelete}
          disabled={deleteTrip.isPending}
        >
          <Text style={styles.deleteText}>
            {deleteTrip.isPending ? t("tripSettings.deleting") : t("tripSettings.deleteConfirm")}
          </Text>
        </Pressable>
      </View>
    </ScrollView>
  );
}
