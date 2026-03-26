import { View, Text, StyleSheet, Pressable, ScrollView, TextInput, Alert, ActivityIndicator } from "react-native";
import { useMemo, useState } from "react";
import { useRouter } from "expo-router";
import { useTranslation } from "react-i18next";
import { useMutation } from "@tanstack/react-query";
import { createClient } from "@connectrpc/connect";
import { LogOut, Download, Trash2, User, FileText, Shield } from "lucide-react-native";
import { useAuth } from "@/lib/auth";
import { useTransport } from "@/lib/transport";
import { AuthService } from "@gen/toqui/v1/auth_pb";

export default function SettingsScreen() {
  const { t } = useTranslation();
  const { user, logout, accessToken } = useAuth();
  const transport = useTransport();
  const router = useRouter();
  const client = useMemo(() => createClient(AuthService, transport), [transport]);
  const [deleteConfirm, setDeleteConfirm] = useState("");

  const exportData = useMutation({
    mutationFn: async () => {
      await client.exportData({});
    },
  });

  const deleteAccount = useMutation({
    mutationFn: async () => {
      await client.deleteAccount({});
      await logout();
    },
  });

  const handleDelete = () => {
    if (deleteConfirm !== "DELETE") return;
    Alert.alert(
      t("settings.deleteAccount"),
      t("settings.deleteWarning"),
      [
        { text: t("common.cancel"), style: "cancel" },
        {
          text: t("settings.deleteConfirm"),
          style: "destructive",
          onPress: () => deleteAccount.mutate(),
        },
      ],
    );
  };

  if (!accessToken) {
    return (
      <View style={styles.center}>
        <Text style={styles.emptyText}>Sign in to view settings</Text>
      </View>
    );
  }

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {/* Account Info */}
      <View style={styles.section}>
        <View style={styles.sectionHeader}>
          <User color="#333" size={20} />
          <Text style={styles.sectionTitle}>{t("settings.account")}</Text>
        </View>
        {user && (
          <View style={styles.accountInfo}>
            <Text style={styles.userName}>{user.name}</Text>
            <Text style={styles.userEmail}>{user.email}</Text>
          </View>
        )}
        <Pressable style={styles.actionRow} onPress={logout}>
          <LogOut color="#e8654a" size={18} />
          <Text style={styles.actionText}>{t("common.signOut")}</Text>
        </Pressable>
      </View>

      {/* Data */}
      <View style={styles.section}>
        <View style={styles.sectionHeader}>
          <Download color="#333" size={20} />
          <Text style={styles.sectionTitle}>{t("settings.exportData")}</Text>
        </View>
        <Pressable
          style={[styles.outlineButton, exportData.isPending && styles.disabledButton]}
          onPress={() => exportData.mutate()}
          disabled={exportData.isPending}
        >
          {exportData.isPending ? (
            <ActivityIndicator color="#e8654a" size="small" />
          ) : (
            <Text style={styles.outlineButtonText}>
              {exportData.isSuccess ? t("settings.exported") : t("settings.exportData")}
            </Text>
          )}
        </Pressable>
      </View>

      {/* Legal */}
      <View style={styles.section}>
        <View style={styles.sectionHeader}>
          <FileText color="#333" size={20} />
          <Text style={styles.sectionTitle}>Legal</Text>
        </View>
        <Pressable style={styles.actionRow} onPress={() => router.push("/privacy" as never)}>
          <Shield color="#666" size={16} />
          <Text style={styles.linkText}>Privacy Policy</Text>
        </Pressable>
        <Pressable style={styles.actionRow} onPress={() => router.push("/terms" as never)}>
          <FileText color="#666" size={16} />
          <Text style={styles.linkText}>Terms of Service</Text>
        </Pressable>
      </View>

      {/* Danger Zone */}
      <View style={[styles.section, styles.dangerSection]}>
        <View style={styles.sectionHeader}>
          <Trash2 color="#ef4444" size={20} />
          <Text style={[styles.sectionTitle, { color: "#ef4444" }]}>{t("settings.deleteAccount")}</Text>
        </View>
        <Text style={styles.dangerWarning}>{t("settings.deleteWarning")}</Text>
        <Text style={styles.dangerLabel}>{t("settings.typeDelete")}</Text>
        <TextInput
          style={styles.dangerInput}
          value={deleteConfirm}
          onChangeText={setDeleteConfirm}
          placeholder="DELETE"
          placeholderTextColor="#ccc"
          autoCapitalize="characters"
        />
        <Pressable
          style={[styles.deleteButton, (deleteConfirm !== "DELETE" || deleteAccount.isPending) && styles.disabledButton]}
          onPress={handleDelete}
          disabled={deleteConfirm !== "DELETE" || deleteAccount.isPending}
        >
          <Text style={styles.deleteText}>
            {deleteAccount.isPending ? t("settings.deleting") : t("settings.deleteConfirm")}
          </Text>
        </Pressable>
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#f5f5f5" },
  content: { padding: 16, paddingBottom: 40 },
  center: { flex: 1, justifyContent: "center", alignItems: "center" },
  emptyText: { fontSize: 16, color: "#666" },
  section: {
    backgroundColor: "#fff",
    borderRadius: 12,
    padding: 16,
    marginBottom: 16,
    borderWidth: 1,
    borderColor: "#e0e0e0",
  },
  dangerSection: { borderColor: "#fca5a5" },
  sectionHeader: { flexDirection: "row", alignItems: "center", gap: 8, marginBottom: 12 },
  sectionTitle: { fontSize: 16, fontWeight: "600", color: "#333" },
  accountInfo: { marginBottom: 12 },
  userName: { fontSize: 16, fontWeight: "600", color: "#333" },
  userEmail: { fontSize: 14, color: "#666", marginTop: 2 },
  actionRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 10,
    paddingVertical: 10,
    borderTopWidth: 1,
    borderTopColor: "#f0f0f0",
  },
  actionText: { fontSize: 15, color: "#e8654a", fontWeight: "500" },
  linkText: { fontSize: 15, color: "#666" },
  outlineButton: {
    borderWidth: 1,
    borderColor: "#e8654a",
    borderRadius: 8,
    padding: 12,
    alignItems: "center",
  },
  disabledButton: { opacity: 0.5 },
  outlineButtonText: { color: "#e8654a", fontWeight: "600" },
  dangerWarning: { fontSize: 14, color: "#666", marginBottom: 12, lineHeight: 20 },
  dangerLabel: { fontSize: 13, color: "#999", marginBottom: 6 },
  dangerInput: {
    borderWidth: 1,
    borderColor: "#fca5a5",
    borderRadius: 8,
    padding: 10,
    fontSize: 14,
    marginBottom: 12,
    color: "#333",
  },
  deleteButton: {
    borderWidth: 1,
    borderColor: "#ef4444",
    borderRadius: 8,
    padding: 12,
    alignItems: "center",
  },
  deleteText: { color: "#ef4444", fontWeight: "600" },
});
