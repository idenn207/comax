import js from "@eslint/js";
import tseslint from "typescript-eslint";

// Flat config, mirrors web/dashboard's eslint 9 + typescript-eslint setup.
export default tseslint.config(
  { ignores: ["dist/**", "coverage/**"] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    rules: {
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
    },
  },
);
