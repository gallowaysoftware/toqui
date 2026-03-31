import { View, Text, Pressable, StyleSheet, Platform } from "react-native";
import { useState } from "react";
import { useTheme } from "@/lib/theme";

interface DatePickerProps {
  value: string; // "YYYY-MM-DD" or ""
  onChange: (date: string) => void; // "YYYY-MM-DD" or ""
  placeholder?: string;
  label?: string;
}

function formatDateToString(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

function parseDateString(value: string): Date | undefined {
  if (!value) return undefined;
  const [y, m, d] = value.split("-").map(Number);
  if (!y || !m || !d) return undefined;
  return new Date(y, m - 1, d);
}

function WebDatePicker({ value, onChange, placeholder, label }: DatePickerProps) {
  const { colors } = useTheme();
  return (
    <View>
      {label ? <Text style={[styles.label, { color: colors.textPrimary }]}>{label}</Text> : null}
      <input
        type="date"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        style={{
          borderWidth: 1,
          borderColor: colors.inputBorder,
          borderStyle: "solid",
          borderRadius: 8,
          padding: 12,
          fontSize: 16,
          color: colors.textPrimary,
          backgroundColor: colors.inputBg,
          width: "100%",
          boxSizing: "border-box" as const,
        }}
      />
    </View>
  );
}

function NativeDatePicker({ value, onChange, placeholder, label }: DatePickerProps) {
  const { colors } = useTheme();
  const [showPicker, setShowPicker] = useState(false);

  const displayText = value || placeholder || "Select date";

  return (
    <View>
      {label ? <Text style={[styles.label, { color: colors.textPrimary }]}>{label}</Text> : null}
      <Pressable
        style={[styles.nativeInput, { backgroundColor: colors.inputBg, borderColor: colors.inputBorder }]}
        onPress={() => setShowPicker(true)}
      >
        <Text style={[styles.nativeInputText, { color: value ? colors.textPrimary : colors.textTertiary }]}>
          {displayText}
        </Text>
      </Pressable>
      {showPicker && (
        <NativeDateTimePickerModal
          value={value}
          onChange={(date) => {
            onChange(date);
            setShowPicker(false);
          }}
          onDismiss={() => setShowPicker(false)}
        />
      )}
    </View>
  );
}

function NativeDateTimePickerModal({
  value,
  onChange,
  onDismiss,
}: {
  value: string;
  onChange: (date: string) => void;
  onDismiss: () => void;
}) {
  // eslint-disable-next-line @typescript-eslint/no-var-requires
  const DateTimePicker = require("@react-native-community/datetimepicker").default;

  return (
    <DateTimePicker
      value={parseDateString(value) ?? new Date()}
      mode="date"
      display="default"
      onChange={(_event: unknown, selectedDate?: Date) => {
        if (selectedDate) {
          onChange(formatDateToString(selectedDate));
        } else {
          onDismiss();
        }
      }}
    />
  );
}

export function DatePicker(props: DatePickerProps) {
  if (Platform.OS === "web") {
    return <WebDatePicker {...props} />;
  }
  return <NativeDatePicker {...props} />;
}

const styles = StyleSheet.create({
  label: {
    fontSize: 14,
    fontWeight: "600",
    marginBottom: 6,
  },
  nativeInput: {
    borderWidth: 1,
    borderRadius: 8,
    padding: 12,
  },
  nativeInputText: {
    fontSize: 16,
  },
});
