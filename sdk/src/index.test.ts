import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { createClient, createClientFromEnv, SecretsClient, ComaxError } from "./index";

describe("createClient", () => {
  it("builds a SecretsClient", () => {
    const client = createClient({ baseUrl: "https://s", token: "t", project: "api", env: "prod" });
    expect(client).toBeInstanceOf(SecretsClient);
  });

  it("validates the baseUrl eagerly", () => {
    expect(() => createClient({ baseUrl: "nope", token: "t", project: "api", env: "prod" })).toThrow(ComaxError);
  });
});

describe("createClientFromEnv", () => {
  const KEYS = ["COMAX_URL", "COMAX_TOKEN", "COMAX_PROJECT", "COMAX_ENV"] as const;
  let saved: Record<string, string | undefined>;

  beforeEach(() => {
    saved = {};
    for (const k of KEYS) {
      saved[k] = process.env[k];
      delete process.env[k];
    }
  });
  afterEach(() => {
    for (const k of KEYS) {
      if (saved[k] === undefined) delete process.env[k];
      else process.env[k] = saved[k];
    }
  });

  it("throws listing every missing variable", () => {
    try {
      createClientFromEnv();
      throw new Error("should have thrown");
    } catch (e) {
      expect(e).toBeInstanceOf(ComaxError);
      const msg = (e as ComaxError).message;
      for (const k of KEYS) expect(msg).toContain(k);
    }
  });

  it("reads all four variables from the environment", () => {
    process.env.COMAX_URL = "https://s";
    process.env.COMAX_TOKEN = "t";
    process.env.COMAX_PROJECT = "api";
    process.env.COMAX_ENV = "prod";
    expect(createClientFromEnv()).toBeInstanceOf(SecretsClient);
  });

  it("lets explicit overrides win over the environment", () => {
    const client = createClientFromEnv({ baseUrl: "https://o", token: "t", project: "p", env: "e" });
    expect(client).toBeInstanceOf(SecretsClient);
  });
});
