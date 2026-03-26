import { createContext, useContext, useState, useEffect, useCallback, useMemo } from "react";
import { useColorScheme, Platform } from "react-native";

export interface ThemeColors {
  surface: string;
  surfaceSecondary: string;
  surfaceTertiary: string;
  border: string;
  borderStrong: string;
  textPrimary: string;
  textSecondary: string;
  textTertiary: string;
  accent: string;
  accentHover: string;
  accentSoft: string;
  error: string;
  errorBg: string;
  success: string;
  successBg: string;
  userBubble: string;
  userBubbleText: string;
  assistantBubble: string;
  assistantBubbleText: string;
  assistantBubbleBorder: string;
  inputBg: string;
  inputBorder: string;
}

const lightColors: ThemeColors = {
  surface: "#ffffff",
  surfaceSecondary: "#f9fafb",
  surfaceTertiary: "#f3f4f6",
  border: "#e5e7eb",
  borderStrong: "#d1d5db",
  textPrimary: "#111827",
  textSecondary: "#4b5563",
  textTertiary: "#5f6673",
  accent: "#e8654a",
  accentHover: "#c44a32",
  accentSoft: "#fef2f0",
  error: "#dc2626",
  errorBg: "#fef2f2",
  success: "#16a34a",
  successBg: "#f0fdf4",
  userBubble: "#e8654a",
  userBubbleText: "#ffffff",
  assistantBubble: "#ffffff",
  assistantBubbleText: "#1f2937",
  assistantBubbleBorder: "#e5e7eb",
  inputBg: "#ffffff",
  inputBorder: "#d1d5db",
};

const darkColors: ThemeColors = {
  surface: "#1a1a2e",
  surfaceSecondary: "#16162a",
  surfaceTertiary: "#222240",
  border: "#2d2d50",
  borderStrong: "#3d3d66",
  textPrimary: "#e8e8f0",
  textSecondary: "#9ca3b8",
  textTertiary: "#9299ad",
  accent: "#f29b85",
  accentHover: "#e8654a",
  accentSoft: "#2a1f1e",
  error: "#f87171",
  errorBg: "#2a1f1f",
  success: "#4ade80",
  successBg: "#1a2e1f",
  userBubble: "#e8654a",
  userBubbleText: "#ffffff",
  assistantBubble: "#222240",
  assistantBubbleText: "#e8e8f0",
  assistantBubbleBorder: "#2d2d50",
  inputBg: "#222240",
  inputBorder: "#3d3d66",
};

type ThemeMode = "light" | "dark" | "system";

interface ThemeContextValue {
  colors: ThemeColors;
  mode: ThemeMode;
  isDark: boolean;
  setMode: (mode: ThemeMode) => void;
}

const ThemeContext = createContext<ThemeContextValue>({
  colors: lightColors,
  mode: "system",
  isDark: false,
  setMode: () => {},
});

export function useTheme() {
  return useContext(ThemeContext);
}

const STORAGE_KEY = "toqui_theme";

async function loadPersistedMode(): Promise<ThemeMode | null> {
  if (Platform.OS === "web") {
    return (localStorage.getItem(STORAGE_KEY) as ThemeMode) ?? null;
  }
  const { getItemAsync } = await import("expo-secure-store");
  return (await getItemAsync(STORAGE_KEY)) as ThemeMode | null;
}

async function persistMode(mode: ThemeMode): Promise<void> {
  if (Platform.OS === "web") {
    localStorage.setItem(STORAGE_KEY, mode);
    return;
  }
  const { setItemAsync } = await import("expo-secure-store");
  await setItemAsync(STORAGE_KEY, mode);
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const systemScheme = useColorScheme();
  const [mode, setModeState] = useState<ThemeMode>("system");
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    loadPersistedMode().then((m) => {
      if (m) setModeState(m);
      setLoaded(true);
    });
  }, []);

  const setMode = useCallback((m: ThemeMode) => {
    setModeState(m);
    void persistMode(m);
  }, []);

  const isDark =
    mode === "dark" || (mode === "system" && systemScheme === "dark");
  const colors = isDark ? darkColors : lightColors;

  const value = useMemo(
    () => ({ colors, mode, isDark, setMode }),
    [colors, mode, isDark, setMode],
  );

  if (!loaded) return null;

  return (
    <ThemeContext.Provider value={value}>
      {children}
    </ThemeContext.Provider>
  );
}
