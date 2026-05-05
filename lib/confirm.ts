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

/**
 * Options for `alertNotice` — a single-OK-button informational dialog,
 * the cross-platform replacement for `Alert.alert(title)` and
 * `Alert.alert(title, message)`.
 */
export interface NoticeOptions {
  /** Dialog title — short, e.g. "Error". */
  title: string;
  /**
   * Optional body text. Many error-toast call sites pass title only
   * (e.g. `Alert.alert(t("common.error"))`); the helper degrades to
   * a title-only dialog when this is omitted.
   */
  message?: string;
  /** Label on the dismiss button. Defaults to "OK". Native-only — `window.alert` has no customizable button. */
  okLabel?: string;
}

/**
 * Cross-platform informational alert.
 *
 * Sibling of `confirmDestructive` for the no-confirmation, single-OK
 * use case (error toasts, "could not share this trip", etc). On web
 * `Alert.alert` is a silent no-op — same root cause as the X-button
 * bug — so this helper routes through `window.alert` on web and
 * `Alert.alert` (single OK button) on native.
 *
 * Returns `void` to match both backends: `window.alert` is
 * synchronous-blocking and returns `undefined`, `Alert.alert` is
 * fire-and-forget. There's nothing meaningful to await — call sites
 * that previously did `Alert.alert(t("common.error"))` should swap
 * one for one without an `await`.
 *
 * Usage:
 *
 *   alertNotice({ title: t("common.error") });
 *   alertNotice({ title: "Error", message: "Could not share this trip." });
 */
export function alertNotice(opts: NoticeOptions): void {
  const okLabel = opts.okLabel ?? "OK";

  if (Platform.OS === "web") {
    // Same SSR / jsdom guard as confirmDestructive — silently degrade
    // to a no-op rather than throw if window.alert isn't there. The
    // call site already treats this as best-effort UX (it's an error
    // toast, not load-bearing logic), so swallowing in non-browser
    // web envs is the right posture.
    if (typeof window === "undefined" || typeof window.alert !== "function") {
      return;
    }
    // window.alert has no separate title slot; concatenate when a
    // message is provided, otherwise show the title alone.
    const text = opts.message ? `${opts.title}\n\n${opts.message}` : opts.title;
    window.alert(text);
    return;
  }

  // Native: Alert.alert with a single OK button. Passing `undefined`
  // for message matches the title-only call sites (Alert.alert(title)
  // with no second arg).
  Alert.alert(opts.title, opts.message, [{ text: okLabel }]);
}
