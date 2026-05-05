/**
 * Cross-platform confirm dialog.
 *
 * `Alert.alert` from React Native is native-only. On web — and Toqui
 * runs on web at app.toqui.travel — it's a silent no-op: the press
 * registers, no dialog appears, and the destructive callback never
 * fires. This silently broke the "remove pending invite" X button on
 * the Trip Members screen (Kyle, 2026-05-05) and is a latent hazard
 * on every other `Alert.alert` site in the codebase.
 *
 * `confirmDestructive` returns a Promise<boolean> that resolves to
 * true if the user confirmed, false otherwise:
 *
 *   - On web: uses the native `window.confirm` dialog. Ugly but
 *     functional. Does not require importing React Native APIs that
 *     don't exist there.
 *   - On native (iOS/Android): uses `Alert.alert` with a Cancel /
 *     Confirm button pair, mapping the destructive button to the
 *     iOS-standard `style: "destructive"` for the red affordance.
 *
 * Usage:
 *
 *   if (await confirmDestructive({
 *     title: "Remove member?",
 *     message: `Remove ${email} from this trip?`,
 *     confirmLabel: "Remove",
 *   })) {
 *     await removeMember.mutateAsync(...);
 *   }
 *
 * Why a Promise instead of an onConfirm callback: callsites can use
 * a single `await` and keep their existing async error-handling
 * try/catch shape, instead of restructuring around the callback.
 * The previous `Alert.alert(..., [{ onPress: async () => { ... } }])`
 * pattern made it easy to forget error handling because the
 * destructive logic lived in a nested closure.
 */
import { Alert, Platform } from "react-native";

export interface ConfirmOptions {
  /** Dialog title — short, e.g. "Remove member?" */
  title: string;
  /** Body text — describes the consequence. */
  message: string;
  /** Label on the destructive button. Defaults to "Confirm". */
  confirmLabel?: string;
  /** Label on the cancel button. Defaults to "Cancel". */
  cancelLabel?: string;
}

export function confirmDestructive(opts: ConfirmOptions): Promise<boolean> {
  const confirmLabel = opts.confirmLabel ?? "Confirm";
  const cancelLabel = opts.cancelLabel ?? "Cancel";

  if (Platform.OS === "web") {
    // window.confirm doesn't render the title separately from the
    // message, so concatenate. Most browsers cap the prompt at a few
    // hundred chars; well within our needs.
    const text = `${opts.title}\n\n${opts.message}`;
    // typeof guard: in non-browser web environments (SSR, vitest
    // jsdom without window.confirm stub) we treat the absence of
    // confirm as "user declined" rather than throwing — same posture
    // as Alert.alert no-op on native when called from a non-UI
    // context. Tests that need to drive this path should stub
    // window.confirm explicitly.
    if (typeof window === "undefined" || typeof window.confirm !== "function") {
      return Promise.resolve(false);
    }
    return Promise.resolve(window.confirm(text));
  }

  return new Promise<boolean>((resolve) => {
    Alert.alert(opts.title, opts.message, [
      { text: cancelLabel, style: "cancel", onPress: () => resolve(false) },
      { text: confirmLabel, style: "destructive", onPress: () => resolve(true) },
    ]);
  });
}
