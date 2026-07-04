/**
 * Webhook signature verification. Exposed at the `@comax-secrets/sdk/webhook`
 * subpath so Edge bundles that only fetch secrets can tree-shake it away.
 *
 * Mirrors `internal/webhook/signer.go` exactly:
 *   signature = "sha256=" + hex(HMAC-SHA256(secret, "<ts>.<body>"))
 *
 * The timestamp is bound INTO the signed material ("<ts>.<body>"), so a
 * replayed request cannot reuse a captured signature with a fresh timestamp.
 * Verification recomputes the HMAC and compares in constant time, then
 * rejects timestamps outside a tolerance window (replay defence).
 *
 * Uses Web Crypto (`crypto.subtle`) only — available in Node 18+, Edge,
 * Workers, Deno, and Bun — so it runs identically everywhere.
 */

/** Delivery header names. Mirror `signer.go`. */
export const HEADER_SIGNATURE = "X-Comax-Signature";
export const HEADER_TIMESTAMP = "X-Comax-Timestamp";
export const HEADER_EVENT = "X-Comax-Event";
export const HEADER_DELIVERY = "X-Comax-Delivery";

export interface VerifyWebhookInput {
  /** The webhook's signing secret (shown once at creation). */
  secret: string;
  /** `X-Comax-Signature` header value ("sha256=<hex>"). */
  signatureHeader: string | null | undefined;
  /** `X-Comax-Timestamp` header value (decimal unix seconds). */
  timestampHeader: string | null | undefined;
  /** Raw request body bytes as received (must not be re-serialized). */
  body: string;
  /** Max age in seconds before a delivery is rejected as a replay. 0 disables. Default 300. */
  toleranceSec?: number;
  /** Current unix seconds. Injectable for deterministic tests. */
  nowSec?: number;
}

export interface VerifyResult {
  valid: boolean;
  /** Present when `valid` is false: why verification failed. */
  reason?: string;
}

/**
 * Verify a webhook delivery. Returns `{ valid: true }` only when the
 * signature matches AND (when tolerance > 0) the timestamp is within the
 * window. Never throws for a bad signature — inspect `.valid`.
 */
export async function verifyWebhookSignature(input: VerifyWebhookInput): Promise<VerifyResult> {
  const { secret, signatureHeader, timestampHeader, body } = input;
  const tolerance = input.toleranceSec ?? 300;

  if (!secret) return { valid: false, reason: "missing signing secret" };
  if (!signatureHeader || !timestampHeader) {
    return { valid: false, reason: "missing signature or timestamp header" };
  }

  const ts = Number(timestampHeader);
  if (!Number.isInteger(ts) || ts <= 0) {
    return { valid: false, reason: "invalid timestamp" };
  }

  if (tolerance > 0) {
    const now = input.nowSec ?? Math.floor(Date.now() / 1000);
    if (Math.abs(now - ts) > tolerance) {
      return { valid: false, reason: "timestamp outside tolerance (possible replay)" };
    }
  }

  const expected = await computeSignature(secret, timestampHeader, body);
  if (!constantTimeEqual(expected, signatureHeader)) {
    return { valid: false, reason: "signature mismatch" };
  }
  return { valid: true };
}

/** "sha256=" + hex(HMAC-SHA256(secret, "<ts>.<body>")). Mirrors `signer.Sign`. */
async function computeSignature(secret: string, ts: string, body: string): Promise<string> {
  const enc = new TextEncoder();
  const key = await crypto.subtle.importKey(
    "raw",
    enc.encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const mac = await crypto.subtle.sign("HMAC", key, enc.encode(`${ts}.${body}`));
  return "sha256=" + toHex(new Uint8Array(mac));
}

function toHex(bytes: Uint8Array): string {
  let out = "";
  for (const b of bytes) out += b.toString(16).padStart(2, "0");
  return out;
}

/** Length-checked constant-time string comparison over UTF-8 bytes. */
function constantTimeEqual(a: string, b: string): boolean {
  const enc = new TextEncoder();
  const ab = enc.encode(a);
  const bb = enc.encode(b);
  if (ab.length !== bb.length) return false;
  let diff = 0;
  for (let i = 0; i < ab.length; i++) {
    diff |= (ab[i] as number) ^ (bb[i] as number);
  }
  return diff === 0;
}
