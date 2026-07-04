import { describe, it, expect } from "vitest";
import { verifyWebhookSignature } from "./webhook";
import vectorsRaw from "../test/fixtures/webhook-vectors.json";

interface Vector {
  name: string;
  secret: string;
  timestamp: string;
  body: string;
  signature: string;
}
const vectors = vectorsRaw as Vector[];

describe("verifyWebhookSignature — Go signer golden vectors (contract parity)", () => {
  for (const v of vectors) {
    it(`accepts the Go-signed vector "${v.name}"`, async () => {
      const res = await verifyWebhookSignature({
        secret: v.secret,
        signatureHeader: v.signature,
        timestampHeader: v.timestamp,
        body: v.body,
        nowSec: Number(v.timestamp),
      });
      expect(res.valid).toBe(true);
    });
  }
});

describe("verifyWebhookSignature — rejections", () => {
  const v = vectors[0]!;

  it("rejects a tampered body", async () => {
    const res = await verifyWebhookSignature({
      secret: v.secret,
      signatureHeader: v.signature,
      timestampHeader: v.timestamp,
      body: v.body + "x",
      nowSec: Number(v.timestamp),
    });
    expect(res.valid).toBe(false);
    expect(res.reason).toMatch(/signature mismatch/);
  });

  it("rejects a tampered timestamp (bound into the signed material)", async () => {
    const badTs = String(Number(v.timestamp) + 1);
    const res = await verifyWebhookSignature({
      secret: v.secret,
      signatureHeader: v.signature,
      timestampHeader: badTs,
      body: v.body,
      nowSec: Number(badTs),
    });
    expect(res.valid).toBe(false);
    expect(res.reason).toMatch(/signature mismatch/);
  });

  it("rejects a stale timestamp beyond tolerance (replay defence)", async () => {
    const res = await verifyWebhookSignature({
      secret: v.secret,
      signatureHeader: v.signature,
      timestampHeader: v.timestamp,
      body: v.body,
      nowSec: Number(v.timestamp) + 10_000,
      toleranceSec: 300,
    });
    expect(res.valid).toBe(false);
    expect(res.reason).toMatch(/tolerance/);
  });

  it("accepts a stale timestamp when tolerance is disabled", async () => {
    const res = await verifyWebhookSignature({
      secret: v.secret,
      signatureHeader: v.signature,
      timestampHeader: v.timestamp,
      body: v.body,
      nowSec: Number(v.timestamp) + 10_000,
      toleranceSec: 0,
    });
    expect(res.valid).toBe(true);
  });

  it("rejects missing headers", async () => {
    const res = await verifyWebhookSignature({
      secret: v.secret,
      signatureHeader: null,
      timestampHeader: v.timestamp,
      body: v.body,
    });
    expect(res.valid).toBe(false);
    expect(res.reason).toMatch(/missing signature/);
  });

  it("rejects a non-numeric timestamp", async () => {
    const res = await verifyWebhookSignature({
      secret: v.secret,
      signatureHeader: v.signature,
      timestampHeader: "not-a-number",
      body: v.body,
    });
    expect(res.valid).toBe(false);
    expect(res.reason).toMatch(/invalid timestamp/);
  });

  it("rejects an empty secret", async () => {
    const res = await verifyWebhookSignature({
      secret: "",
      signatureHeader: v.signature,
      timestampHeader: v.timestamp,
      body: v.body,
      nowSec: Number(v.timestamp),
    });
    expect(res.valid).toBe(false);
    expect(res.reason).toMatch(/missing signing secret/);
  });
});
