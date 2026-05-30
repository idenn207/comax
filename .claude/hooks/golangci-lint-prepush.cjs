#!/usr/bin/env node
// Pre-push lint guard. Reads the Bash tool_input from stdin and, if the
// command looks like `git push`, runs `make lint` (golangci-lint).
// Exit codes:
//   0 - not a push, or lint passed
//   2 - lint failed; harness blocks the tool call

const { spawnSync } = require("node:child_process");

let raw = "";
process.stdin.setEncoding("utf8");
process.stdin.on("data", (chunk) => (raw += chunk));
process.stdin.on("end", () => {
  let cmd = "";
  try {
    cmd = (JSON.parse(raw)?.tool_input?.command ?? "").toString();
  } catch {
    process.exit(0);
  }

  if (!/(^|\s|;|&&|\|\|)git(\.exe)?\s+push(\s|$)/i.test(cmd)) {
    process.exit(0);
  }

  process.stderr.write("[hook] golangci-lint check before push...\n");

  // Prefer `make lint` so the hook tracks the project's canonical command.
  // Fallback to `golangci-lint run` if make is unavailable (Windows hosts
  // without GNU make).
  const tryRun = (bin, args) =>
    spawnSync(bin, args, { stdio: "inherit", shell: process.platform === "win32" });

  let r = tryRun("make", ["lint"]);
  if (r.error && r.error.code === "ENOENT") {
    r = tryRun("golangci-lint", ["run", "--timeout=5m"]);
  }
  if (r.error && r.error.code === "ENOENT") {
    process.stderr.write(
      "[hook] WARN: neither `make` nor `golangci-lint` on PATH; skipping pre-push lint.\n",
    );
    process.exit(0);
  }
  if (r.status !== 0) {
    process.stderr.write(
      "[hook] BLOCKED: golangci-lint failed. Fix the lint errors before pushing.\n",
    );
    process.exit(2);
  }
  process.stderr.write("[hook] golangci-lint passed.\n");
  process.exit(0);
});
