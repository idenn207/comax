package crypto

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"testing"
)

// freshKey returns a random 32-byte key for test use.
func freshKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return k
}

func TestSealOpenRoundTrip(t *testing.T) {
	key := freshKey(t)
	cases := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", []byte{}},
		{"short", []byte("hi")},
		{"medium", []byte("postgres://user:pass@host:5432/db?sslmode=require")},
		{"binary", []byte{0x00, 0x01, 0x02, 0xfe, 0xff}},
		{"large", bytes.Repeat([]byte("a"), 4096)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sealed, err := Seal(key, tc.plaintext)
			if err != nil {
				t.Fatalf("Seal: %v", err)
			}
			plain, err := Open(key, sealed)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if !bytes.Equal(plain, tc.plaintext) {
				t.Errorf("round-trip mismatch: got %q want %q", plain, tc.plaintext)
			}
		})
	}
}

func TestSealEmitsLayoutHeader(t *testing.T) {
	// The sealed blob must start with a 12-byte nonce and grow by
	// NonceSize + TagSize relative to the plaintext.
	key := freshKey(t)
	plain := []byte("DB_URL=postgres://...")

	sealed, err := Seal(key, plain)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if got, want := len(sealed), len(plain)+NonceSize+TagSize; got != want {
		t.Errorf("sealed len = %d; want %d", got, want)
	}
}

func TestSealGeneratesUniqueNoncePerCall(t *testing.T) {
	// Two seals of the same plaintext under the same key must differ in
	// their nonce prefix (and therefore the whole blob). This is the
	// invariant that makes GCM safe.
	key := freshKey(t)
	plain := []byte("identical")

	a, err := Seal(key, plain)
	if err != nil {
		t.Fatalf("Seal a: %v", err)
	}
	b, err := Seal(key, plain)
	if err != nil {
		t.Fatalf("Seal b: %v", err)
	}
	if bytes.Equal(a[:NonceSize], b[:NonceSize]) {
		t.Error("two Seal calls produced identical nonces; nonce randomness is broken")
	}
	if bytes.Equal(a, b) {
		t.Error("two Seal calls produced identical ciphertexts")
	}
}

func TestOpenRejectsWrongKey(t *testing.T) {
	key := freshKey(t)
	wrongKey := freshKey(t)
	sealed, _ := Seal(key, []byte("secret"))

	_, err := Open(wrongKey, sealed)
	if err == nil {
		t.Fatal("expected auth failure with wrong key; got nil")
	}
}

func TestOpenRejectsTamperedCiphertext(t *testing.T) {
	key := freshKey(t)
	sealed, _ := Seal(key, []byte("hello"))

	// Flip a single bit in the ciphertext region (skip the nonce prefix).
	tampered := make([]byte, len(sealed))
	copy(tampered, sealed)
	tampered[NonceSize] ^= 0x01

	_, err := Open(key, tampered)
	if err == nil {
		t.Fatal("expected auth failure on tampered ciphertext; got nil")
	}
}

func TestOpenRejectsShortInput(t *testing.T) {
	key := freshKey(t)
	cases := [][]byte{
		nil,
		{},
		make([]byte, NonceSize-1),         // can't even hold a nonce
		make([]byte, NonceSize+TagSize-1), // can hold nonce but not tag
	}
	for _, sealed := range cases {
		_, err := Open(key, sealed)
		if !errors.Is(err, ErrCiphertextTooShort) {
			t.Errorf("len=%d: err = %v; want ErrCiphertextTooShort", len(sealed), err)
		}
	}
}

func TestSealOpenRejectInvalidKeySizes(t *testing.T) {
	cases := [][]byte{
		nil,
		make([]byte, 0),
		make([]byte, 16), // AES-128 size — also not allowed
		make([]byte, 24), // AES-192 size — also not allowed
		make([]byte, 33),
	}
	for _, k := range cases {
		if _, err := Seal(k, []byte("x")); !errors.Is(err, ErrInvalidKeySize) {
			t.Errorf("Seal len=%d: err = %v; want ErrInvalidKeySize", len(k), err)
		}
		if _, err := Open(k, make([]byte, NonceSize+TagSize)); !errors.Is(err, ErrInvalidKeySize) {
			t.Errorf("Open len=%d: err = %v; want ErrInvalidKeySize", len(k), err)
		}
	}
}

// TestSealOpenPropertyRoundTrip is the plan's "round-trip property test"
// — fuzz random plaintexts through Seal/Open and assert equality.
func TestSealOpenPropertyRoundTrip(t *testing.T) {
	key := freshKey(t)
	for i := 0; i < 100; i++ {
		// length uniformly in [0, 1024]
		var lenBuf [2]byte
		if _, err := io.ReadFull(rand.Reader, lenBuf[:]); err != nil {
			t.Fatalf("rand len: %v", err)
		}
		n := int(lenBuf[0])<<8 | int(lenBuf[1])
		n %= 1025

		plain := make([]byte, n)
		if _, err := io.ReadFull(rand.Reader, plain); err != nil {
			t.Fatalf("rand plain: %v", err)
		}

		sealed, err := Seal(key, plain)
		if err != nil {
			t.Fatalf("iter %d Seal len=%d: %v", i, n, err)
		}
		got, err := Open(key, sealed)
		if err != nil {
			t.Fatalf("iter %d Open len=%d: %v", i, n, err)
		}
		if !bytes.Equal(got, plain) {
			t.Errorf("iter %d round-trip mismatch", i)
		}
	}
}
