import { useState, useEffect, useCallback } from "react";
import {
  View,
  Text,
  TextInput,
  Pressable,
  Modal,
  StyleSheet,
  ActivityIndicator,
  ScrollView,
  KeyboardAvoidingView,
  Platform,
} from "react-native";
import { usePathname } from "expo-router";
import { useLocalSearchParams } from "expo-router";
import { useTranslation } from "react-i18next";
import { X, Check } from "lucide-react-native";
import { useTheme } from "@/lib/theme";
import { useFeedback, type FeedbackType } from "@/lib/hooks/useFeedback";

interface FeedbackModalProps {
  visible: boolean;
  onClose: () => void;
}

const FEEDBACK_TYPES: { key: FeedbackType; i18nKey: string }[] = [
  { key: "bug", i18nKey: "feedback.typeBug" },
  { key: "feature", i18nKey: "feedback.typeFeature" },
  { key: "general", i18nKey: "feedback.typeGeneral" },
  { key: "chat_quality", i18nKey: "feedback.typeChatQuality" },
];

const MIN_MESSAGE_LENGTH = 10;

export default function FeedbackModal({ visible, onClose }: FeedbackModalProps) {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const pathname = usePathname();
  const params = useLocalSearchParams<{ tripId?: string }>();
  const { submit, isSubmitting, isSuccess, error, reset } = useFeedback();

  const [type, setType] = useState<FeedbackType>("general");
  const [message, setMessage] = useState("");

  const canSubmit = message.trim().length >= MIN_MESSAGE_LENGTH && !isSubmitting;

  const handleClose = useCallback(() => {
    setType("general");
    setMessage("");
    reset();
    onClose();
  }, [onClose, reset]);

  useEffect(() => {
    if (isSuccess) {
      const timer = setTimeout(handleClose, 2000);
      return () => clearTimeout(timer);
    }
  }, [isSuccess, handleClose]);

  function handleSubmit() {
    if (!canSubmit) return;
    submit(type, message.trim(), pathname, params.tripId);
  }

  return (
    <Modal
      visible={visible}
      transparent
      animationType="slide"
      onRequestClose={handleClose}
    >
      <KeyboardAvoidingView
        style={styles.overlay}
        behavior={Platform.OS === "ios" ? "padding" : undefined}
      >
        <Pressable style={styles.backdrop} onPress={handleClose} />
        <View
          style={[
            styles.sheet,
            { backgroundColor: colors.surface, borderColor: colors.border },
          ]}
        >
          {isSuccess ? (
            <View style={styles.successContainer}>
              <View
                style={[styles.successIcon, { backgroundColor: colors.successBg }]}
              >
                <Check color={colors.success} size={28} />
              </View>
              <Text style={[styles.successTitle, { color: colors.textPrimary }]}>
                {t("feedback.success")}
              </Text>
              <Text
                style={[styles.successDetail, { color: colors.textSecondary }]}
              >
                {t("feedback.successDetail")}
              </Text>
            </View>
          ) : (
            <ScrollView keyboardShouldPersistTaps="handled">
              {/* Header */}
              <View style={styles.header}>
                <Text style={[styles.title, { color: colors.textPrimary }]}>
                  {t("feedback.title")}
                </Text>
                <Pressable onPress={handleClose} hitSlop={8}>
                  <X color={colors.textSecondary} size={22} />
                </Pressable>
              </View>

              {/* Type Picker */}
              <View style={styles.typePicker}>
                {FEEDBACK_TYPES.map(({ key, i18nKey }) => {
                  const selected = type === key;
                  return (
                    <Pressable
                      key={key}
                      style={[
                        styles.typeChip,
                        {
                          borderColor: selected
                            ? colors.accent
                            : colors.border,
                          backgroundColor: selected
                            ? colors.accentSoft
                            : colors.surfaceTertiary,
                        },
                      ]}
                      onPress={() => setType(key)}
                    >
                      <Text
                        style={[
                          styles.typeChipText,
                          {
                            color: selected
                              ? colors.accent
                              : colors.textSecondary,
                          },
                        ]}
                      >
                        {t(i18nKey)}
                      </Text>
                    </Pressable>
                  );
                })}
              </View>

              {/* Message */}
              <TextInput
                style={[
                  styles.textArea,
                  {
                    backgroundColor: colors.inputBg,
                    borderColor: colors.inputBorder,
                    color: colors.textPrimary,
                  },
                ]}
                placeholder={t("feedback.messagePlaceholder")}
                placeholderTextColor={colors.textTertiary}
                multiline
                numberOfLines={5}
                textAlignVertical="top"
                value={message}
                onChangeText={setMessage}
                maxLength={2000}
              />

              {error && (
                <Text style={[styles.errorText, { color: colors.error }]}>
                  {t("feedback.error")}
                </Text>
              )}

              {/* Submit */}
              <Pressable
                style={[
                  styles.submitButton,
                  { backgroundColor: colors.accent },
                  !canSubmit && styles.disabledButton,
                ]}
                onPress={handleSubmit}
                disabled={!canSubmit}
              >
                {isSubmitting ? (
                  <ActivityIndicator color="#fff" size="small" />
                ) : (
                  <Text style={styles.submitText}>{t("feedback.submit")}</Text>
                )}
              </Pressable>
            </ScrollView>
          )}
        </View>
      </KeyboardAvoidingView>
    </Modal>
  );
}

const styles = StyleSheet.create({
  overlay: {
    flex: 1,
    justifyContent: "flex-end",
  },
  backdrop: {
    ...StyleSheet.absoluteFillObject,
    backgroundColor: "rgba(0,0,0,0.4)",
  },
  sheet: {
    borderTopLeftRadius: 20,
    borderTopRightRadius: 20,
    borderWidth: 1,
    borderBottomWidth: 0,
    padding: 20,
    paddingBottom: 36,
    maxHeight: "80%",
  },
  header: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    marginBottom: 16,
  },
  title: {
    fontSize: 18,
    fontWeight: "700",
  },
  typePicker: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 8,
    marginBottom: 16,
  },
  typeChip: {
    paddingHorizontal: 14,
    paddingVertical: 8,
    borderRadius: 20,
    borderWidth: 1,
  },
  typeChipText: {
    fontSize: 13,
    fontWeight: "500",
  },
  textArea: {
    borderWidth: 1,
    borderRadius: 10,
    padding: 12,
    fontSize: 15,
    minHeight: 120,
    marginBottom: 12,
  },
  errorText: {
    fontSize: 13,
    marginBottom: 12,
  },
  submitButton: {
    borderRadius: 10,
    padding: 14,
    alignItems: "center",
  },
  disabledButton: {
    opacity: 0.5,
  },
  submitText: {
    color: "#fff",
    fontSize: 16,
    fontWeight: "600",
  },
  successContainer: {
    alignItems: "center",
    paddingVertical: 32,
  },
  successIcon: {
    width: 56,
    height: 56,
    borderRadius: 28,
    alignItems: "center",
    justifyContent: "center",
    marginBottom: 16,
  },
  successTitle: {
    fontSize: 18,
    fontWeight: "700",
    marginBottom: 8,
  },
  successDetail: {
    fontSize: 14,
    textAlign: "center",
    lineHeight: 20,
  },
});
