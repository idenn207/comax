import { describe, it, expect, vi } from "vitest";
import { createClient } from "./index";
import type { FetchLike } from "./http";

// Enforces CLAUDE.md's "secrets never in logs" rule at the SDK boundary:
// a secret value or the bearer token must never surface in a thrown error
// or a console write.
const TOKEN = "svc_tok_MUST_NOT_LEAK";
const SECRET_VALUE = "super-secret-db-password-9f3a";

describe("no secret/token leakage", () => {
  it("thrown errors never contain the token or a secret value", async () => {
    const fetchMock: FetchLike = async () =>
      new Response(JSON.stringify({ ok: false, error: { code: "unauthorized", message: "unknown bearer token" } }), {
        status: 401,
      });
    const client = createClient({ baseUrl: "https://s", token: TOKEN, project: "api", env: "prod", fetch: fetchMock });

    let caught: unknown;
    try {
      await client.get("DB_URL");
    } catch (e) {
      caught = e;
    }
    expect(caught).toBeInstanceOf(Error);
    const err = caught as Error;
    const surface = `${String(err)}|${err.message}|${err.stack ?? ""}`;
    expect(surface).not.toContain(TOKEN);
    expect(surface).not.toContain(SECRET_VALUE);
  });

  it("does not write secrets to the console during a successful fetch", async () => {
    const methods = ["log", "info", "warn", "error", "debug"] as const;
    const spies = methods.map((m) => vi.spyOn(console, m).mockImplementation(() => {}));

    const fetchMock: FetchLike = async () =>
      new Response(
        JSON.stringify({ ok: true, data: { key: "DB_URL", value: SECRET_VALUE, version: 1, updated_at: "x" } }),
        { status: 200 },
      );
    const client = createClient({ baseUrl: "https://s", token: TOKEN, project: "api", env: "prod", fetch: fetchMock });

    const value = await client.get("DB_URL");
    expect(value).toBe(SECRET_VALUE);
    for (const spy of spies) expect(spy).not.toHaveBeenCalled();
    for (const spy of spies) spy.mockRestore();
  });
});
