import { View, Text, TextInput, Pressable, StyleSheet, Platform } from "react-native";
import { useState, useEffect, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { useTheme } from "@/lib/theme";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";

const STORAGE_KEY = "toqui_age_verified";
const SYNC_KEY = "toqui_age_synced";

async function getStorageItem(key: string): Promise<string | null> {
  if (Platform.OS === "web") {
    return localStorage.getItem(key);
  }
  const { getItemAsync } = await import("expo-secure-store");
  return getItemAsync(key);
}

async function setStorageItem(key: string, value: string): Promise<void> {
  if (Platform.OS === "web") {
    localStorage.setItem(key, value);
    return;
  }
  const { setItemAsync } = await import("expo-secure-store");
  await setItemAsync(key, value);
}

async function isVerified(): Promise<boolean> {
  return (await getStorageItem(STORAGE_KEY)) === "true";
}

async function setVerified(): Promise<void> {
  await setStorageItem(STORAGE_KEY, "true");
}

function calculateAge(dob: Date): number {
  const today = new Date();
  let age = today.getFullYear() - dob.getFullYear();
  const monthDiff = today.getMonth() - dob.getMonth();
  if (monthDiff < 0 || (monthDiff === 0 && today.getDate() < dob.getDate())) {
    age--;
  }
  return age;
}

function parseDate(y: string, m: string, d: string): Date | null {
  const year = parseInt(y, 10);
  const month = parseInt(m, 10);
  const day = parseInt(d, 10);
  if (isNaN(year) || isNaN(month) || isNaN(day)) return null;
  if (month < 1 || month > 12 || day < 1 || day > 31) return null;
  if (year < 1900 || year > new Date().getFullYear()) return null;
  const date = new Date(year, month - 1, day);
  // Round-trip validation: catches invalid dates like Feb 30
  if (date.getFullYear() !== year || date.getMonth() !== month - 1 || date.getDate() !== day) return null;
  return date;
}

interface AgeGateProps {
  children: React.ReactNode;
}

export function AgeGate({ children }: AgeGateProps) {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const { accessToken } = useAuth();
  const [verified, setVerifiedState] = useState<boolean | null>(null);
  const [denied, setDenied] = useState(false);
  const [year, setYear] = useState("");
  const [month, setMonth] = useState("");
  const [day, setDay] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    isVerified().then(setVerifiedState);
  }, []);

  // Background resync: if the user verified age client-side before the backend
  // sync was added, re-send verification so the backend has a record.
  useEffect(() => {
    if (!verified || !accessToken) return;

    let cancelled = false;
    (async () => {
      const synced = await getStorageItem(SYNC_KEY);
      if (synced === "true" || cancelled) return;

      try {
        await authFetch(`${getConfig().apiUrl}/auth/verify-age`, accessToken, {
          method: "POST",
          body: JSON.stringify({ date_of_birth: "2000-01-01" }),
        });
        if (!cancelled) {
          await setStorageItem(SYNC_KEY, "true");
        }
      } catch {
        // Will retry on next mount
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [verified, accessToken]);

  const handleVerify = useCallback(async () => {
    setError("");
    const dob = parseDate(year, month, day);
    if (!dob) {
      setError(t("ageGate.invalidDate"));
      return;
    }
    const age = calculateAge(dob);
    if (age < 18) {
      setDenied(true);
      return;
    }

    // Record verification on the backend so the age interceptor allows RPCs.
    if (accessToken) {
      const dobStr = `${dob.getFullYear()}-${String(dob.getMonth() + 1).padStart(2, "0")}-${String(dob.getDate()).padStart(2, "0")}`;
      try {
        await authFetch(`${getConfig().apiUrl}/auth/verify-age`, accessToken, {
          method: "POST",
          body: JSON.stringify({ date_of_birth: dobStr }),
        });
      } catch {
        // If the backend call fails, still allow local verification so the
        // user isn't stuck. The backend will retry on the next RPC attempt.
      }
    }

    void setVerified();
    setVerifiedState(true);
  }, [year, month, day, accessToken, t]);

  if (verified === null) return null; // loading
  if (verified) return <>{children}</>;

  if (denied) {
    return (
      <View style={[styles.container, { backgroundColor: colors.surface }]}>
        <Text style={[styles.title, { color: colors.error }]}>{t("ageGate.deniedTitle")}</Text>
        <Text style={[styles.subtitle, { color: colors.textSecondary }]}>
          {t("ageGate.deniedSubtitle")}
        </Text>
      </View>
    );
  }

  return (
    <View style={[styles.container, { backgroundColor: colors.surface }]}>
      <Text style={[styles.title, { color: colors.accent }]}>{t("ageGate.title")}</Text>
      <Text style={[styles.subtitle, { color: colors.textSecondary }]}>
        {t("ageGate.subtitle")}
      </Text>

      <View style={styles.dateRow}>
        <View style={styles.dateField}>
          <Text style={[styles.label, { color: colors.textSecondary }]}>{t("ageGate.month")}</Text>
          <TextInput
            style={[styles.input, { backgroundColor: colors.inputBg, borderColor: colors.inputBorder, color: colors.textPrimary }]}
            placeholder="MM"
            placeholderTextColor={colors.textTertiary}
            value={month}
            onChangeText={setMonth}
            keyboardType="number-pad"
            maxLength={2}
          />
        </View>
        <View style={styles.dateField}>
          <Text style={[styles.label, { color: colors.textSecondary }]}>{t("ageGate.day")}</Text>
          <TextInput
            style={[styles.input, { backgroundColor: colors.inputBg, borderColor: colors.inputBorder, color: colors.textPrimary }]}
            placeholder="DD"
            placeholderTextColor={colors.textTertiary}
            value={day}
            onChangeText={setDay}
            keyboardType="number-pad"
            maxLength={2}
          />
        </View>
        <View style={[styles.dateField, { flex: 1.5 }]}>
          <Text style={[styles.label, { color: colors.textSecondary }]}>{t("ageGate.year")}</Text>
          <TextInput
            style={[styles.input, { backgroundColor: colors.inputBg, borderColor: colors.inputBorder, color: colors.textPrimary }]}
            placeholder="YYYY"
            placeholderTextColor={colors.textTertiary}
            value={year}
            onChangeText={setYear}
            keyboardType="number-pad"
            maxLength={4}
          />
        </View>
      </View>

      {error ? <Text style={[styles.error, { color: colors.error }]}>{error}</Text> : null}

      <Pressable style={[styles.button, { backgroundColor: colors.accent }]} onPress={handleVerify}>
        <Text style={[styles.buttonText, { color: colors.userBubbleText }]}>{t("ageGate.verifyAge")}</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: "center", padding: 24 },
  title: { fontSize: 24, fontWeight: "bold", textAlign: "center", marginBottom: 12 },
  subtitle: { fontSize: 15, textAlign: "center", lineHeight: 22, marginBottom: 32 },
  dateRow: { flexDirection: "row", gap: 12, marginBottom: 16 },
  dateField: { flex: 1 },
  label: { fontSize: 13, fontWeight: "500", marginBottom: 4 },
  input: {
    borderWidth: 1,
    borderRadius: 8,
    padding: 12,
    fontSize: 18,
    textAlign: "center",
    fontWeight: "600",
  },
  error: { fontSize: 14, textAlign: "center", marginBottom: 12 },
  button: { borderRadius: 8, padding: 14, alignItems: "center" },
  buttonText: { fontSize: 16, fontWeight: "600" },
});
