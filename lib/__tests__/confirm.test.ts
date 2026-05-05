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

// ---------------------------------------------------------------------------
// alertNotice — single-OK informational alert (sibling of confirmDestructive)
// ---------------------------------------------------------------------------

describe("alertNotice on web", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.doMock("react-native", () => ({
      Platform: { OS: "web" },
      Alert: { alert: vi.fn() }, // Not used on web; included so the import resolves.
    }));
  });

  it("calls window.alert with title + message concatenated", () => {
    const original = window.alert;
    window.alert = vi.fn();

    return import("../confirm").then(({ alertNotice }) => {
      alertNotice({ title: "Error", message: "Could not share this trip." });

      expect(window.alert).toHaveBeenCalledTimes(1);
      // Single string argument with both title and message — same
      // shape as confirmDestructive's window.confirm path, since
      // window.alert has no separate title slot.
      expect(window.alert).toHaveBeenCalledWith(expect.stringContaining("Error"));
      expect(window.alert).toHaveBeenCalledWith(expect.stringContaining("Could not share this trip."));

      window.alert = original;
    });
  });

  it("calls window.alert with title only when message is omitted", () => {
    // Most error-toast call sites in the sweep pass title only —
    // e.g. `Alert.alert(t("common.error"))`. The helper must
    // degrade to a title-only dialog without crashing on the
    // missing message.
    const original = window.alert;
    window.alert = vi.fn();

    return import("../confirm").then(({ alertNotice }) => {
      alertNotice({ title: "Error" });

      expect(window.alert).toHaveBeenCalledTimes(1);
      expect(window.alert).toHaveBeenCalledWith("Error");

      window.alert = original;
    });
  });

  it("does not throw when window.alert is unavailable (SSR / weird env)", async () => {
    // Mirrors the confirmDestructive SSR guard. Error toasts are
    // best-effort UX — swallowing is the right posture in non-browser
    // web envs rather than crashing the app.
    const original = window.alert;
    // @ts-expect-error — deliberate runtime delete to simulate the SSR shape
    delete window.alert;

    const { alertNotice } = await import("../confirm");
    expect(() => alertNotice({ title: "X", message: "Y" })).not.toThrow();

    window.alert = original;
  });
});

describe("alertNotice on native", () => {
  beforeEach(() => {
    vi.resetModules();
  });

  it("calls Alert.alert with a single OK button", async () => {
    type AlertButton = { text: string; style?: string; onPress?: () => void };
    let capturedTitle = "";
    let capturedMessage: string | undefined;
    let capturedButtons: AlertButton[] = [];
    vi.doMock("react-native", () => ({
      Platform: { OS: "ios" },
      Alert: {
        alert: (title: string, message: string | undefined, buttons: AlertButton[]) => {
          capturedTitle = title;
          capturedMessage = message;
          capturedButtons = buttons;
        },
      },
    }));

    const { alertNotice } = await import("../confirm");
    alertNotice({ title: "Error", message: "Something failed." });

    expect(capturedTitle).toBe("Error");
    expect(capturedMessage).toBe("Something failed.");
    // Single button — no Cancel/Confirm pair, this is informational.
    expect(capturedButtons).toHaveLength(1);
    expect(capturedButtons[0].text).toBe("OK");
  });

  it("forwards undefined message to Alert.alert when omitted", async () => {
    // Title-only call sites (Alert.alert(t("common.error"))) used
    // to pass undefined as the second arg to RN's Alert.alert. The
    // helper must preserve that — RN renders title-only correctly
    // when message is undefined.
    let capturedMessage: string | undefined = "sentinel";
    vi.doMock("react-native", () => ({
      Platform: { OS: "android" },
      Alert: {
        alert: (_t: string, message: string | undefined) => {
          capturedMessage = message;
        },
      },
    }));

    const { alertNotice } = await import("../confirm");
    alertNotice({ title: "Error" });

    expect(capturedMessage).toBeUndefined();
  });

  it("respects a custom okLabel on native", async () => {
    type AlertButton = { text: string };
    let capturedButtons: AlertButton[] = [];
    vi.doMock("react-native", () => ({
      Platform: { OS: "ios" },
      Alert: {
        alert: (_t: string, _m: string | undefined, buttons: AlertButton[]) => {
          capturedButtons = buttons;
        },
      },
    }));

    const { alertNotice } = await import("../confirm");
    alertNotice({ title: "X", okLabel: "Got it" });

    expect(capturedButtons[0].text).toBe("Got it");
  });
});
