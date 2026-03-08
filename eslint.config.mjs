import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import tseslint from "typescript-eslint";

const eslintConfig = defineConfig([
  ...nextVitals,

  // TypeScript type-aware rules (recommended level — catches real bugs without noise)
  ...tseslint.configs.recommendedTypeChecked.map((config) => ({
    ...config,
    files: ["src/**/*.ts", "src/**/*.tsx"],
  })),

  {
    files: ["src/**/*.ts", "src/**/*.tsx"],
    languageOptions: {
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      // -- Tune recommended rules for Next.js ergonomics --

      // Allow void for fire-and-forget async (e.g., event handlers)
      "@typescript-eslint/no-floating-promises": ["error", { ignoreVoid: true }],
      // Promise-returning functions passed as callbacks (onClick, etc.) are common in React
      "@typescript-eslint/no-misused-promises": [
        "error",
        { checksVoidReturn: { attributes: false } },
      ],
      // Don't flag unused vars prefixed with _ (destructuring patterns)
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
      // Require-await is too strict for interface compliance
      "@typescript-eslint/require-await": "off",
      // Too many false positives with third-party libs (maplibre, protobuf, etc.)
      "@typescript-eslint/no-unsafe-member-access": "off",
      "@typescript-eslint/no-unsafe-assignment": "off",
      "@typescript-eslint/no-unsafe-argument": "off",
      "@typescript-eslint/no-unsafe-call": "off",
      "@typescript-eslint/no-unsafe-return": "off",

      // -- Extra rules we opt into --

      // Enforce consistent type imports (treeshaking, clarity)
      "@typescript-eslint/consistent-type-imports": [
        "warn",
        { prefer: "type-imports", fixStyle: "inline-type-imports" },
      ],
      // Prefer nullish coalescing over logical OR for safety
      "@typescript-eslint/prefer-nullish-coalescing": "warn",
    },
  },

  globalIgnores([
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
    "src/gen/**",
  ]),
]);

export default eslintConfig;
