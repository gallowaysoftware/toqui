import { View, Text, TextInput, Pressable, StyleSheet, Platform } from "react-native";
import { useState, useEffect, useCallback } from "react";
import { useTheme } from "@/lib/theme";

const STORAGE_KEY = "toqui_age_verified";

async function isVerified(): Promise<boolean> {
  if (Platform.OS === "web") {
    return localStorage.getItem(STORAGE_KEY) === "true";
  }
  const { getItemAsync } = await import("expo-secure-store");
  return (await getItemAsync(STORAGE_KEY)) === "true";
}

async function setVerified(): Promise<void> {
  if (Platform.OS === "web") {
    localStorage.setItem(STORAGE_KEY, "true");
    return;
  }
  const { setItemAsync } = await import("expo-secure-store");
  await setItemAsync(STORAGE_KEY, "true");
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
  const { colors } = useTheme();
  const [verified, setVerifiedState] = useState<boolean | null>(null);
  const [denied, setDenied] = useState(false);
  const [year, setYear] = useState("");
  const [month, setMonth] = useState("");
  const [day, setDay] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    isVerified().then(setVerifiedState);
  }, []);

  const handleVerify = useCallback(() => {
    setError("");
    const dob = parseDate(year, month, day);
    if (!dob) {
      setError("Please enter a valid date of birth.");
      return;
    }
    const age = calculateAge(dob);
    if (age < 18) {
      setDenied(true);
      return;
    }
    void setVerified();
    setVerifiedState(true);
  }, [year, month, day]);

  if (verified === null) return null; // loading
  if (verified) return <>{children}</>;

  if (denied) {
    return (
      <View style={[styles.container, { backgroundColor: colors.surface }]}>
        <Text style={[styles.title, { color: colors.error }]}>Access Denied</Text>
        <Text style={[styles.subtitle, { color: colors.textSecondary }]}>
          You must be at least 18 years old to use Toqui. Please come back when you're old enough.
        </Text>
      </View>
    );
  }

  return (
    <View style={[styles.container, { backgroundColor: colors.surface }]}>
      <Text style={[styles.title, { color: colors.accent }]}>Age Verification</Text>
      <Text style={[styles.subtitle, { color: colors.textSecondary }]}>
        You must be at least 18 years old to use Toqui. Please enter your date of birth.
      </Text>

      <View style={styles.dateRow}>
        <View style={styles.dateField}>
          <Text style={[styles.label, { color: colors.textSecondary }]}>Month</Text>
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
          <Text style={[styles.label, { color: colors.textSecondary }]}>Day</Text>
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
          <Text style={[styles.label, { color: colors.textSecondary }]}>Year</Text>
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
        <Text style={styles.buttonText}>Verify Age</Text>
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
  buttonText: { color: "#fff", fontSize: 16, fontWeight: "600" },
});
