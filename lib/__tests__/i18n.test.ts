import { describe, it, expect, beforeAll } from "vitest";
import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import en from "@/messages/en.json";

/**
 * Initialize i18next in the same way the app does (lib/i18n.tsx).
 * Because init() is synchronous when resources are provided inline,
 * i18n.isInitialized should be true immediately after this call.
 */
beforeAll(() => {
  if (!i18n.isInitialized) {
    i18n.use(initReactI18next).init({
      resources: { en: { translation: en } },
      lng: "en",
      fallbackLng: "en",
      interpolation: { escapeValue: false },
    });
  }
});

// ---------------------------------------------------------------------------
// Every t() key actually used across the codebase, extracted by grepping
// all files that call useTranslation() + t("...").
//
// If a key is added in a component but missing from en.json, this test
// will fail — catching the "untranslated string" bug at CI time.
// ---------------------------------------------------------------------------
const KEYS_USED_IN_APP: string[] = [
  // app/(tabs)/index.tsx
  "common.appName",
  "common.tagline",
  "common.signIn",
  "trips.empty",
  "trips.newTrip",

  // app/(tabs)/settings.tsx
  "settings.account",
  "settings.deleteAccount",
  "settings.deleteWarning",
  "settings.deleteConfirm",
  "settings.typeDelete",
  "settings.deleting",
  "settings.exportData",
  "settings.exported",
  "common.cancel",
  "common.signOut",

  // app/trips/new.tsx
  "tripCreate.whereLabel",
  "tripCreate.wherePlaceholder",
  "tripCreate.descriptionLabel",
  "tripCreate.descriptionPlaceholder",
  "tripCreate.startDate",
  "tripCreate.endDate",
  "tripCreate.submit",

  // app/trips/[tripId]/settings.tsx
  "tripSettings.editTitle",
  "tripSettings.editDescription",
  "tripSettings.editStartDate",
  "tripSettings.editEndDate",
  "tripSettings.save",
  "tripSettings.saving",
  "tripSettings.deleteTrip",
  "tripSettings.deleteWarning",
  "tripSettings.deleteConfirm",
  "tripSettings.deleting",

  // app/waitlist.tsx
  "waitlist.title",
  "waitlist.description",
  "waitlist.emailPlaceholder",
  "waitlist.joinButton",
  "waitlist.joinedTitle",
  "waitlist.joinedDescription",
  "waitlist.positionLabel",
  "waitlist.notifyMessage",
];

describe("i18n - translation key integrity", () => {
  it.each(KEYS_USED_IN_APP)(
    "t(\"%s\") resolves to a non-empty string (not the raw key)",
    (key) => {
      const value = i18n.t(key);
      // i18next returns the key itself when the translation is missing
      expect(value).not.toBe(key);
      expect(value.length).toBeGreaterThan(0);
    },
  );
});

// ---------------------------------------------------------------------------
// Nested key access — verifies the dot-separated key path works for
// deeply nested JSON like trips.status.planning
// ---------------------------------------------------------------------------
describe("i18n - nested key access", () => {
  it("resolves trips.status.planning to 'Planning'", () => {
    expect(i18n.t("trips.status.planning")).toBe("Planning");
  });

  it("resolves trips.status.active to 'Active'", () => {
    expect(i18n.t("trips.status.active")).toBe("Active");
  });

  it("resolves trips.status.completed to 'Completed'", () => {
    expect(i18n.t("trips.status.completed")).toBe("Completed");
  });

  it("returns the key for a non-existent nested path", () => {
    const bogusKey = "trips.status.nonExistent";
    expect(i18n.t(bogusKey)).toBe(bogusKey);
  });
});

// ---------------------------------------------------------------------------
// Synchronous initialization — the I18nProvider relies on
// i18n.isInitialized being true immediately after the module-level init()
// so that children render on the first pass (no flash of null).
// ---------------------------------------------------------------------------
describe("i18n - synchronous initialization", () => {
  it("is initialized immediately when resources are provided inline", () => {
    expect(i18n.isInitialized).toBe(true);
  });

  it("has 'en' as the current language", () => {
    expect(i18n.language).toBe("en");
  });

  it("has the translation resource bundle loaded", () => {
    const bundle = i18n.getResourceBundle("en", "translation");
    expect(bundle).toBeDefined();
    expect(bundle.common).toBeDefined();
    expect(bundle.common.appName).toBe("Toqui");
  });
});

// ---------------------------------------------------------------------------
// en.json structural sanity — no empty strings or null values that would
// silently render blank UI.
// ---------------------------------------------------------------------------
describe("i18n - en.json has no empty or null values", () => {
  function flattenObject(
    obj: Record<string, unknown>,
    prefix = "",
  ): Array<[string, unknown]> {
    const entries: Array<[string, unknown]> = [];
    for (const [key, value] of Object.entries(obj)) {
      const path = prefix ? `${prefix}.${key}` : key;
      if (typeof value === "object" && value !== null && !Array.isArray(value)) {
        entries.push(
          ...flattenObject(value as Record<string, unknown>, path),
        );
      } else {
        entries.push([path, value]);
      }
    }
    return entries;
  }

  const allEntries = flattenObject(en as Record<string, unknown>);

  it.each(allEntries)(
    "key \"%s\" has a non-empty string value",
    (_key, value) => {
      expect(typeof value).toBe("string");
      expect((value as string).trim().length).toBeGreaterThan(0);
    },
  );
});
