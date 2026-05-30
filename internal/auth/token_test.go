package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"

	"github.com/idenn207/comax-secrets/internal/store"
)

func TestGenerateToken_FreshEachCall(t *testing.T) {
	a, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken #1: %v", err)
	}
	b, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken #2: %v", err)
	}
	if a == b {
		t.Errorf("GenerateToken returned duplicate %q (collision implies broken RNG)", a)
	}
	// 32 random bytes → 43-char unpadded base64url.
	if len(a) != 43 {
		t.Errorf("len(GenerateToken()) = %d; want 43", len(a))
	}
}

func TestHashToken_Deterministic(t *testing.T) {
	want := sha256.Sum256([]byte("hello"))
	got := HashToken("hello")
	if string(got) != string(want[:]) {
		t.Errorf("HashToken not equal to crypto/sha256 reference")
	}
}

func TestParseBearer(t *testing.T) {
	cases := []struct {
		name    string
		header  string
		wantTok string
		wantErr error
	}{
		{"happy path", "Bearer xyz", "xyz", nil},
		{"case-insensitive scheme", "bearer xyz", "xyz", nil},
		{"trims trailing whitespace", "Bearer xyz   ", "xyz", nil},
		{"empty header", "", "", ErrMissingBearer},
		{"no scheme prefix", "xyz", "", ErrInvalidBearer},
		{"wrong scheme", "Basic xyz", "", ErrInvalidBearer},
		{"scheme only", "Bearer ", "", ErrInvalidBearer},
		{"scheme + whitespace only", "Bearer    ", "", ErrInvalidBearer},
		{"too short to be Bearer", "Bear", "", ErrInvalidBearer},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tok, err := ParseBearer(tc.header)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("err = %v; want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tok != tc.wantTok {
				t.Errorf("tok = %q; want %q", tok, tc.wantTok)
			}
		})
	}
}

// newTestDB opens an isolated SQLite file under t.TempDir and migrates
// the schema. Mirror of internal/store's helper, duplicated here so the
// auth package does not import store's test helpers (which would force
// _test.go visibility coupling).
func newTestDB(t *testing.T) *store.TokenRepo {
	t.Helper()
	dbPath := t.TempDir() + "/auth.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("store.Migrate: %v", err)
	}
	return store.NewTokenRepo(db)
}

func TestVerify_HappyPath(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()
	plain, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if _, err := repo.Create(ctx, "ci", HashToken(plain)); err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	tok, err := Verify(ctx, repo, plain)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if tok.Name != "ci" {
		t.Errorf("Verify returned tok.Name = %q; want ci", tok.Name)
	}
}

func TestVerify_RejectsEmptyPlaintext(t *testing.T) {
	_, err := Verify(context.Background(), newTestDB(t), "")
	if !errors.Is(err, ErrMissingBearer) {
		t.Errorf("err = %v; want %v", err, ErrMissingBearer)
	}
}

func TestVerify_UnknownTokenIsErrUnknown(t *testing.T) {
	_, err := Verify(context.Background(), newTestDB(t), "definitely-not-a-token")
	if !errors.Is(err, ErrUnknownToken) {
		t.Errorf("err = %v; want %v", err, ErrUnknownToken)
	}
}

func TestContext_RoundTrip(t *testing.T) {
	in := store.ServiceToken{ID: 42, Name: "ci"}
	ctx := WithToken(context.Background(), in)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext returned ok=false on a context that was stamped")
	}
	if got.ID != 42 || got.Name != "ci" {
		t.Errorf("FromContext returned %+v; want id=42 name=ci", got)
	}
}

func TestContext_BlankWhenUnstamped(t *testing.T) {
	if _, ok := FromContext(context.Background()); ok {
		t.Error("FromContext returned ok=true on a blank context")
	}
}

func TestGenerateToken_IsURLSafe(t *testing.T) {
	// base64url uses [-A-Za-z0-9_]; spot-check across multiple draws.
	for i := 0; i < 10; i++ {
		tok, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}
		if strings.ContainsAny(tok, "+/=") {
			t.Errorf("GenerateToken returned %q containing non-URL-safe chars", tok)
		}
	}
}
