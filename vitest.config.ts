import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./vitest.setup.ts"],
    include: ["**/__tests__/**/*.test.{ts,tsx}", "**/*.test.{ts,tsx}"],
    exclude: ["node_modules", "dist", ".expo"],
  },
  resolve: {
    alias: [
      { find: /^@gen\/(.*)/, replacement: path.resolve(__dirname, "src/gen/$1") },
      { find: /^@\/(.*)/, replacement: path.resolve(__dirname, "$1") },
      { find: "react-native", replacement: "react-native-web" },
    ],
  },
});
