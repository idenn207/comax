package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestPayload_MarshalCanonicalNoValue(t *testing.T) {
	p := Payload{
		Action:    "secret.upsert",
		Project:   "app",
		Env:       "prod",
		Key:       "DB_URL",
		Version:   3,
		Timestamp: 1700000000,
	}
	b, err := p.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(b)

	// Metadata keys present…
	for _, k := range []string{`"action"`, `"project"`, `"env"`, `"key"`, `"version"`, `"timestamp"`} {
		if !strings.Contains(got, k) {
			t.Errorf("payload missing %s: %s", k, got)
		}
	}
	// …and NO value field can exist — the struct has none.
	if strings.Contains(got, `"value"`) {
		t.Errorf("payload leaked a value field: %s", got)
	}

	// Deterministic: a re-marshal is byte-identical (stable key order).
	b2, _ := p.Marshal()
	if got != string(b2) {
		t.Errorf("marshal not deterministic:\n%s\n%s", got, b2)
	}

	// version omitted for a delete (zero value).
	del := Payload{Action: "secret.delete", Project: "app", Env: "prod", Key: "OLD", Timestamp: 1}
	db, _ := del.Marshal()
	if strings.Contains(string(db), `"version"`) {
		t.Errorf("delete payload should omit version: %s", db)
	}
	// sanity: it's valid JSON
	var sink map[string]any
	if err := json.Unmarshal(b, &sink); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
}

func TestSign_FormatAndDeterminism(t *testing.T) {
	secret := []byte("signing-secret")
	body := []byte(`{"delivery_id":1}`)

	sig, ts := Sign(secret, body, 1700000000)
	if ts != "1700000000" {
		t.Errorf("timestamp header = %q; want 1700000000", ts)
	}
	if !strings.HasPrefix(sig, "sha256=") {
		t.Errorf("signature missing sha256= prefix: %q", sig)
	}
	if hexPart := strings.TrimPrefix(sig, "sha256="); len(hexPart) != sha256.Size*2 {
		t.Errorf("signature hex len = %d; want %d", len(hexPart), sha256.Size*2)
	}

	// Deterministic for identical inputs.
	sig2, _ := Sign(secret, body, 1700000000)
	if sig != sig2 {
		t.Errorf("Sign not deterministic: %q vs %q", sig, sig2)
	}
}

func TestSign_BindsTimestampAndBody(t *testing.T) {
	secret := []byte("k")
	body := []byte("payload")

	base, _ := Sign(secret, body, 100)
	if diffTs, _ := Sign(secret, body, 101); diffTs == base {
		t.Error("signature did not change with timestamp — replay protection broken")
	}
	if diffBody, _ := Sign(secret, []byte("payload2"), 100); diffBody == base {
		t.Error("signature did not change with body")
	}
	if diffKey, _ := Sign([]byte("k2"), body, 100); diffKey == base {
		t.Error("signature did not change with key")
	}
}

// TestSign_MatchesManualHMAC pins the exact signed material ("<ts>.<body>") so
// a receiver implementing the documented scheme verifies successfully.
func TestSign_MatchesManualHMAC(t *testing.T) {
	secret := []byte("shared")
	body := []byte(`{"x":1}`)
	tsUnix := int64(1700000000)

	sig, _ := Sign(secret, body, tsUnix)

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(strconv.FormatInt(tsUnix, 10) + "."))
	mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if sig != want {
		t.Errorf("Sign = %q; manual HMAC = %q", sig, want)
	}
	if !hmac.Equal([]byte(strings.TrimPrefix(sig, "sha256=")), []byte(strings.TrimPrefix(want, "sha256="))) {
		t.Error("constant-time verify failed")
	}
}
