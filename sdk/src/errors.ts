/**
 * Typed errors for the Comax Secrets SDK.
 *
 * Mirrors `pkg/client/client.go`'s `APIError` + sentinel errors: the server
 * returns a machine-readable `code` on every failure, so callers switch on
 * `error.code` (or `instanceof`) instead of parsing message strings.
 *
 * INVARIANT: a secret's plaintext value NEVER appears in an error message.
 * Failures carry only the server `code`, the HTTP `status`, and the server's
 * generic message. This mirrors the server's "no secrets in logs" rule.
 */

/** Machine-readable failure codes. Superset of `internal/server/response.go`. */
export type ComaxErrorCode =
  | "unauthorized"
  | "forbidden"
  | "not_found"
  | "version_not_found"
  | "conflict"
  | "already_bootstrapped"
  | "bad_request"
  | "csrf_mismatch"
  | "internal"
  | "timeout"
  | "network"
  | "invalid_response";

/** Base error for every SDK failure. Inspect `.code` / `.status`. */
export class ComaxError extends Error {
  readonly code: ComaxErrorCode;
  readonly status: number;

  constructor(code: ComaxErrorCode, message: string, status = 0) {
    super(message);
    this.name = "ComaxError";
    this.code = code;
    this.status = status;
    // Preserve the prototype chain so `instanceof` works after transpilation
    // to older targets (a well-known TS/Babel class-extends-Error gotcha).
    Object.setPrototypeOf(this, new.target.prototype);
  }
}

/** 401/403 — missing, malformed, unknown, or non-privileged bearer token. */
export class ComaxAuthError extends ComaxError {
  constructor(message = "unauthorized", status = 401, code: ComaxErrorCode = "unauthorized") {
    super(code, message, status);
    this.name = "ComaxAuthError";
  }
}

/** 404 — project, env, secret, or version not found. */
export class ComaxNotFoundError extends ComaxError {
  constructor(message = "not found", status = 404, code: ComaxErrorCode = "not_found") {
    super(code, message, status);
    this.name = "ComaxNotFoundError";
  }
}

/** 409 — resource already exists / already bootstrapped. */
export class ComaxConflictError extends ComaxError {
  constructor(message = "conflict", status = 409, code: ComaxErrorCode = "conflict") {
    super(code, message, status);
    this.name = "ComaxConflictError";
  }
}

/**
 * Maps a server (status, code, message) triple to the most specific error
 * subclass. Mirrors `APIError.Is()` in the Go client: the same code strings
 * drive both. Unknown codes fall back to a generic `ComaxError`.
 */
export function toComaxError(status: number, code: string, message: string): ComaxError {
  switch (code) {
    case "unauthorized":
      return new ComaxAuthError(message, status, "unauthorized");
    case "forbidden":
      return new ComaxAuthError(message, status, "forbidden");
    case "not_found":
      return new ComaxNotFoundError(message, status, "not_found");
    case "version_not_found":
      return new ComaxNotFoundError(message, status, "version_not_found");
    case "conflict":
      return new ComaxConflictError(message, status, "conflict");
    case "already_bootstrapped":
      return new ComaxConflictError(message, status, "already_bootstrapped");
    default:
      return new ComaxError(isKnownCode(code) ? code : "internal", message, status);
  }
}

const KNOWN_CODES = new Set<string>([
  "unauthorized",
  "forbidden",
  "not_found",
  "version_not_found",
  "conflict",
  "already_bootstrapped",
  "bad_request",
  "csrf_mismatch",
  "internal",
  "timeout",
  "network",
  "invalid_response",
]);

function isKnownCode(code: string): code is ComaxErrorCode {
  return KNOWN_CODES.has(code);
}
