import { useState, useEffect, useCallback } from "react";
import { Platform } from "react-native";

const STORAGE_KEY = "toqui_onboarding_complete";

const onboardingStorage = {
  get(): boolean {
    if (Platform.OS === "web") {
      return sessionStorage.getItem(STORAGE_KEY) === "true";
    }
    // On native, we use a synchronous check via a module-level variable
    // that gets hydrated in the effect below.
    return false;
  },
  async getAsync(): Promise<boolean> {
    if (Platform.OS === "web") {
      return sessionStorage.getItem(STORAGE_KEY) === "true";
    }
    const { getItemAsync } = await import("expo-secure-store");
    const value = await getItemAsync(STORAGE_KEY);
    return value === "true";
  },
  async set(): Promise<void> {
    if (Platform.OS === "web") {
      sessionStorage.setItem(STORAGE_KEY, "true");
      return;
    }
    const { setItemAsync } = await import("expo-secure-store");
    await setItemAsync(STORAGE_KEY, "true");
  },
};

export function useOnboarding() {
  const [isComplete, setIsComplete] = useState<boolean | null>(null);

  useEffect(() => {
    onboardingStorage.getAsync().then((complete) => {
      setIsComplete(complete);
    });
  }, []);

  const completeOnboarding = useCallback(async () => {
    await onboardingStorage.set();
    setIsComplete(true);
  }, []);

  return {
    /** null while loading, then true/false */
    isOnboardingComplete: isComplete,
    /** Whether the check is still loading */
    isLoading: isComplete === null,
    /** Mark onboarding as complete */
    completeOnboarding,
  };
}
