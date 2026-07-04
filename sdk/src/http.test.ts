import { describe, it, expect, vi } from "vitest";
import { Http, type FetchLike } from "./http";
import { ComaxError, ComaxAuthError, ComaxNotFoundError, ComaxConflictError } from "./errors";

function jsonResponse(status: number, obj: unknown): Response {
  return new Response(JSON.stringify(obj), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("Http constructor", () => {
  it("requires a baseUrl", () => {
    expect(() => new Http({ baseUrl: "", token: "t" })).toThrow(/baseUrl is required/);
  });

  it("rejects a non-http(s) scheme", () => {
    expect(() => new Http({ baseUrl: "ftp://x", token: "t" })).toThrow(/http or https/);
  });

  it("rejects a malformed URL", () => {
    expect(() => new Http({ baseUrl: "not a url", token: "t" })).toThrow(ComaxError);
  });
});

describe("Http.get", () => {
  it("returns envelope data and sends the bearer token", async () => {
    const fetchMock = vi.fn<FetchLike>(async (_url, init) => {
      const headers = init?.headers as Record<string, string>;
      expect(headers["Authorization"]).toBe("Bearer tok");
      expect(headers["Accept"]).toBe("application/json");
      return jsonResponse(200, { ok: true, data: { key: "A", value: "v", version: 1, updated_at: "x" } });
    });
    const http = new Http({ baseUrl: "https://s.example.com/", token: "tok", fetch: fetchMock });
    const data = await http.get<{ key: string }>("/api/v1/x");
    expect(data.key).toBe("A");
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("omits Authorization when token is empty", async () => {
    const fetchMock = vi.fn<FetchLike>(async (_url, init) => {
      const headers = init?.headers as Record<string, string>;
      expect(headers["Authorization"]).toBeUndefined();
      return jsonResponse(200, { ok: true, data: null });
    });
    const http = new Http({ baseUrl: "https://s", token: "", fetch: fetchMock });
    await http.get("/healthz");
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("maps 401 to ComaxAuthError", async () => {
    const http = new Http({
      baseUrl: "https://s",
      token: "t",
      fetch: async () => jsonResponse(401, { ok: false, error: { code: "unauthorized", message: "unknown bearer token" } }),
    });
    await expect(http.get("/x")).rejects.toBeInstanceOf(ComaxAuthError);
  });

  it("maps 404 to ComaxNotFoundError", async () => {
    const http = new Http({
      baseUrl: "https://s",
      token: "t",
      fetch: async () => jsonResponse(404, { ok: false, error: { code: "not_found", message: "resource not found" } }),
    });
    await expect(http.get("/x")).rejects.toBeInstanceOf(ComaxNotFoundError);
  });

  it("maps 409 to ComaxConflictError", async () => {
    const http = new Http({
      baseUrl: "https://s",
      token: "t",
      fetch: async () => jsonResponse(409, { ok: false, error: { code: "conflict", message: "resource already exists" } }),
    });
    await expect(http.get("/x")).rejects.toBeInstanceOf(ComaxConflictError);
  });

  it("carries the server code on a generic error", async () => {
    const http = new Http({
      baseUrl: "https://s",
      token: "t",
      fetch: async () => jsonResponse(400, { ok: false, error: { code: "bad_request", message: "name is required" } }),
    });
    await expect(http.get("/x")).rejects.toMatchObject({ code: "bad_request", status: 400 });
  });

  it("throws invalid_response on a malformed envelope", async () => {
    const http = new Http({
      baseUrl: "https://s",
      token: "t",
      fetch: async () => new Response("<html>oops</html>", { status: 200 }),
    });
    await expect(http.get("/x")).rejects.toMatchObject({ code: "invalid_response" });
  });

  it("maps an AbortError to code timeout", async () => {
    const http = new Http({
      baseUrl: "https://s",
      token: "t",
      fetch: async () => {
        throw Object.assign(new Error("aborted"), { name: "AbortError" });
      },
    });
    await expect(http.get("/x")).rejects.toMatchObject({ code: "timeout" });
  });

  it("maps a network failure to code network without leaking the cause", async () => {
    const http = new Http({
      baseUrl: "https://s",
      token: "sekret-token",
      fetch: async () => {
        throw new Error("ECONNREFUSED https://s with sekret-token");
      },
    });
    await expect(http.get("/x")).rejects.toMatchObject({ code: "network" });
    await expect(http.get("/x")).rejects.not.toThrowError(/sekret-token/);
  });
});
