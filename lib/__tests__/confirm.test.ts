/**
 * Cross-platform confirm helper tests.
 *
 * Behaviour pinned:
 *   - On web (Platform.OS = "web"), uses window.confirm and resolves
 *     to its boolean return value.
 *   - On web with no window.confirm available (SSR / jsdom edge),
 *     resolves to false rather than throwing.
 *   - On native (Platform.OS = "ios"), uses Alert.alert with a
 *     Cancel/Confirm button pair and resolves true/false based on
 *     which button was pressed.
 *
 * The native path is the harder case — Alert.alert's API is
 * fire-and-callback. The test invokes the registered Cancel or
 * Confirm onPress directly to drive the resolve.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";

// ---------------------------------------------------------------------------
// Web path
// ---------------------------------------------------------------------------

describe("confirmDestructive on web", () => {
  beforeEach(() => {
    vi.resetModules();
    // The default vitest jsdom environment treats Platform.OS as the
    // value baked into our test mock, so we mock react-native fresh
    // in this describe block to ensure web behaviour.
    vi.doMock("react-native", () => ({
      Platform: { OS: "web" },
      Alert: { alert: vi.fn() }, // Not used on web; included so the import resolves.
    }));
  });

  it("resolves true when window.confirm returns true", async () => {
    const original = window.confirm;
    window.confirm = vi.fn(() => true);

    const { confirmDestructive } = await import("../confirm");
    const result = await confirmDestructive({
      title: "Remove member?",
      message: "Are you sure?",
    });

    expect(result).toBe(true);
    expect(window.confirm).toHaveBeenCalledTimes(1);
    // Concatenated title + message — the only way window.confirm
    // shows both since it has no separate title slot.
    expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining("Remove member?"));
    expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining("Are you sure?"));

    window.confirm = original;
  });

  it("resolves false when window.confirm returns false", async () => {
    const original = window.confirm;
    window.confirm = vi.fn(() => false);

    const { confirmDestructive } = await import("../confirm");
    const result = await confirmDestructive({
      title: "Remove?",
      message: "X",
    });

    expect(result).toBe(false);

    window.confirm = original;
  });

  it("resolves false when window.confirm is unavailable (SSR / weird env)", async () => {
    // Some web envs (SSR, jsdom without confirm stub) won't have
    // window.confirm. Helper must not throw — the contract is that
    // a missing dialog is treated as "user declined".
    const original = window.confirm;
    // @ts-expect-error — deliberate runtime delete to simulate the SSR shape
    delete window.confirm;

    const { confirmDestructive } = await import("../confirm");
    const result = await confirmDestructive({ title: "X", message: "Y" });

    expect(result).toBe(false);

    window.confirm = original;
  });
});

// ---------------------------------------------------------------------------
// Native path
// ---------------------------------------------------------------------------

describe("confirmDestructive on native", () => {
  beforeEach(() => {
    vi.resetModules();
  });

  it("resolves true when the destructive Alert button is pressed", async () => {
    type AlertButton = { text: string; style?: string; onPress?: () => void };
    let capturedButtons: AlertButton[] = [];
    const alertMock = vi.fn((_t: string, _m: string, buttons: AlertButton[]) => {
      capturedButtons = buttons;
    });

    vi.doMock("react-native", () => ({
      Platform: { OS: "ios" },
      Alert: { alert: alertMock },
    }));

    const { confirmDestructive } = await import("../confirm");
    const promise = confirmDestructive({
      title: "Remove?",
      message: "Are you sure?",
      confirmLabel: "Remove",
    });

    expect(alertMock).toHaveBeenCalledTimes(1);
    expect(capturedButtons).toHaveLength(2);

    const confirmButton = capturedButtons.find((b) => b.text === "Remove");
    expect(confirmButton).toBeDefined();
    expect(confirmButton?.style).toBe("destructive");

    // Drive the resolve by invoking the destructive onPress.
    confirmButton?.onPress?.();
    await expect(promise).resolves.toBe(true);
  });

  it("resolves false when the cancel Alert button is pressed", async () => {
    type AlertButton = { text: string; style?: string; onPress?: () => void };
    let capturedButtons: AlertButton[] = [];
    vi.doMock("react-native", () => ({
      Platform: { OS: "android" },
      Alert: {
        alert: (_t: string, _m: string, buttons: AlertButton[]) => {
          capturedButtons = buttons;
        },
      },
    }));

    const { confirmDestructive } = await import("../confirm");
    const promise = confirmDestructive({ title: "X", message: "Y" });

    const cancelButton = capturedButtons.find((b) => b.style === "cancel");
    expect(cancelButton).toBeDefined();
    cancelButton?.onPress?.();
    await expect(promise).resolves.toBe(false);
  });

  it("uses default labels when none provided", async () => {
    type AlertButton = { text: string; style?: string; onPress?: () => void };
    let capturedButtons: AlertButton[] = [];
    vi.doMock("react-native", () => ({
      Platform: { OS: "ios" },
      Alert: {
        alert: (_t: string, _m: string, buttons: AlertButton[]) => {
          capturedButtons = buttons;
        },
      },
    }));

    const { confirmDestructive } = await import("../confirm");
    void confirmDestructive({ title: "X", message: "Y" });

    const labels = capturedButtons.map((b) => b.text);
    expect(labels).toContain("Cancel");
    expect(labels).toContain("Confirm");
  });
});
