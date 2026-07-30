// @ts-check
import tseslint from "typescript-eslint";
import reactHooks from "eslint-plugin-react-hooks";

export default tseslint.config(
  {
    ignores: [
      "node_modules/**",
      "src/gen/**", // generated protobuf bindings
      "backend/**", // Go service
      "dist/**",
      ".expo/**",
      "expo-env.d.ts",
      "ios/**", // expo prebuild output
      "android/**",
      "coverage/**",
      "**/*.js", // untyped config scripts (metro.config.js, plugins/)
      "**/*.mjs",
    ],
  },
  ...tseslint.configs.recommendedTypeChecked,
  {
    // Classic react-hooks rules only. The React Compiler-powered rules that
    // ship in v7's recommended preset (static-components, refs,
    // set-state-in-effect, ...) flag ~60 sites that need real refactors —
    // adopt them deliberately, not as part of bootstrapping lint.
    plugins: { "react-hooks": reactHooks },
    rules: {
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",
    },
  },
  {
    languageOptions: {
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_", caughtErrors: "none" },
      ],
      // RN event handlers and effects routinely pass async functions where
      // void is expected; TanStack Query APIs return floating promises by
      // design. Enforcing these would mean void-wrapping half the app.
      "@typescript-eslint/no-misused-promises": [
        "error",
        { checksVoidReturn: false },
      ],
      "@typescript-eslint/no-floating-promises": "off",
      // The ConnectRPC error paths and i18next interop lean on `any` at the
      // boundaries; the unsafe-* family flags every touch point. Type safety
      // inside the app is tsc --strict's job.
      "@typescript-eslint/no-unsafe-assignment": "off",
      "@typescript-eslint/no-unsafe-member-access": "off",
      "@typescript-eslint/no-unsafe-argument": "off",
      "@typescript-eslint/no-unsafe-call": "off",
      "@typescript-eslint/no-unsafe-return": "off",
      "@typescript-eslint/no-require-imports": "off", // RN asset require()
    },
  },
  {
    files: ["**/__tests__/**", "tests/**", "vitest.setup.ts", "vitest.config.ts"],
    rules: {
      "@typescript-eslint/no-explicit-any": "off",
      "@typescript-eslint/unbound-method": "off",
      "@typescript-eslint/require-await": "off",
    },
  },
);
