import { defineConfig } from "vitest/config";

// Coverage bar mirrors the repo-wide 80% rule (CLAUDE.md). index.ts is a
// thin factory + re-export barrel; the logic it wires (http/secrets/errors/
// webhook) is covered directly.
export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
    coverage: {
      provider: "v8",
      include: ["src/**/*.ts"],
      exclude: ["src/**/*.test.ts"],
      thresholds: {
        lines: 80,
        functions: 80,
        branches: 75,
        statements: 80,
      },
    },
  },
});
