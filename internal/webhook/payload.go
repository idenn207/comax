// Package webhook delivers signed, metadata-only event notifications to
// operator-registered HTTP endpoints when secrets change.
//
// The package has four parts: the delivery Payload (this file), the HMAC
// Signer (signer.go), the SSRF-hardened URL policy + HTTP client (ssrf.go),
// and the background delivery Worker (worker.go). A secret's plaintext value
// never appears in any of them — the payload carries only metadata, and the
// receiver re-pulls the value with its own authenticated credential.
package webhook

import "encoding/json"

// Payload is the JSON body POSTed to a webhook endpoint. It deliberately has
// NO value field: it names WHICH secret changed (project/env/key/version) and
// HOW (action), but never the secret's contents. A receiver treats it as a
// trigger and re-pulls the value through the authenticated CLI/API.
//
// The per-delivery id is NOT in the body — it travels in the X-Comax-Delivery
// header (mirroring GitHub's X-GitHub-Delivery), because the outbox row's id
// does not exist until it is inserted, and keeping it out of the body lets the
// signed bytes be fixed at enqueue time without a post-insert rewrite. The
// header is the receiver's idempotency key.
//
// Field order is fixed by this struct definition, so json.Marshal produces a
// canonical, stable byte sequence — the exact bytes that get signed and sent.
type Payload struct {
	Action    string `json:"action"`
	Project   string `json:"project"`
	Env       string `json:"env"`
	Key       string `json:"key"`
	Version   int64  `json:"version,omitempty"` // omitted for delete (no surviving version)
	Timestamp int64  `json:"timestamp"`         // unix seconds the event was recorded
}

// Marshal renders the payload as canonical JSON. This is the single
// representation shared by the signer and the worker: the bytes returned here
// are signed as-is and used verbatim as the request body.
func (p Payload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}
