package auth

import (
	"errors"
	"testing"
)

func TestGenerateCSRF_ProducesDistinctUniqueTokens(t *testing.T) {
	a, err := GenerateCSRF()
	if err != nil {
		t.Fatalf("GenerateCSRF a: %v", err)
	}
	b, err := GenerateCSRF()
	if err != nil {
		t.Fatalf("GenerateCSRF b: %v", err)
	}
	if a == "" || a == b {
		t.Errorf("token a=%q b=%q (need distinct, non-empty)", a, b)
	}
	if len(a) < 32 {
		t.Errorf("token length=%d; want >= 32 (32 random bytes → 43-char base64url)", len(a))
	}
}

func TestVerifyCSRF_MatchesOnEqualPlaintext(t *testing.T) {
	tok, err := GenerateCSRF()
	if err != nil {
		t.Fatalf("GenerateCSRF: %v", err)
	}
	if err := VerifyCSRF(tok, HashCSRF(tok)); err != nil {
		t.Errorf("VerifyCSRF on matching pair returned %v", err)
	}
}

func TestVerifyCSRF_RejectsMismatch(t *testing.T) {
	cases := []struct {
		name      string
		presented string
		expected  []byte
	}{
		{"empty presented", "", HashCSRF("anything")},
		{"empty expected", "anything", nil},
		{"different plaintext", "token-A", HashCSRF("token-B")},
	}
	for _, c := range cases {
		if err := VerifyCSRF(c.presented, c.expected); !errors.Is(err, ErrCSRFMismatch) {
			t.Errorf("%s: err=%v; want ErrCSRFMismatch", c.name, err)
		}
	}
}

func TestIPPrefix(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"v4 with port", "10.0.0.45:12345", "10.0.0.0/24"},
		{"v4 no port", "192.168.1.7", "192.168.1.0/24"},
		{"v6 with port", "[2001:db8:abcd:0012::1]:443", "2001:db8:abcd::/48"},
		{"empty", "", ""},
		{"unparseable", "not-an-ip", ""},
	}
	for _, c := range cases {
		got := IPPrefix(c.in)
		if got != c.want {
			t.Errorf("%s: IPPrefix(%q) = %q; want %q", c.name, c.in, got, c.want)
		}
	}
}
