package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// Header names carried on every delivery. Receivers verify the signature and
// may reject a stale timestamp to defend against replay.
const (
	HeaderSignature = "X-Comax-Signature"
	HeaderTimestamp = "X-Comax-Timestamp"
	HeaderEvent     = "X-Comax-Event"
	HeaderDelivery  = "X-Comax-Delivery"
)

// Sign computes the delivery signature for body at tsUnix using the webhook's
// per-endpoint signing secret. It returns the X-Comax-Signature value
// ("sha256=<hex>") and the X-Comax-Timestamp value (the decimal unix seconds).
//
// The timestamp is bound INTO the signed material as "<ts>.<body>", so a
// replayed request cannot reuse a captured signature with a fresh timestamp:
// changing the timestamp invalidates the HMAC. This is the sender side, so a
// constant-time comparison is irrelevant here — receivers MUST compare with
// hmac.Equal (constant time) when verifying.
func Sign(secret, body []byte, tsUnix int64) (sigHeader, tsHeader string) {
	ts := strconv.FormatInt(tsUnix, 10)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil)), ts
}
