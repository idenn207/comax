import { defineConfig } from "tsup";

// Dual ESM + CJS + .d.ts. Each entry is bundled independently so the
// ./webhook subpath stays tree-shakeable away from the core client for
// Edge consumers that only fetch secrets. .js = ESM, .cjs = CJS (package
// "type": "module" makes bare .js ESM).
export default defineConfig({
  entry: ["src/index.ts", "src/webhook.ts"],
  format: ["esm", "cjs"],
  dts: true,
  clean: true,
  sourcemap: false,
  treeshake: true,
  target: "es2022",
  outExtension({ format }) {
    return { js: format === "cjs" ? ".cjs" : ".js" };
  },
});
