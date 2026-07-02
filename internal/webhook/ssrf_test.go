package webhook

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func mustCIDR(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("ParseCIDR(%q): %v", s, err)
	}
	return n
}

func TestValidateURL_SchemeAndHost(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		url  string
	}{
		{"ftp scheme", "ftp://example.com/x"},
		{"file scheme", "file:///etc/passwd"},
		{"no scheme", "example.com/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateURL(ctx, tc.url, Policy{}); !errors.Is(err, ErrInvalidURL) {
				t.Errorf("ValidateURL(%q) = %v; want ErrInvalidURL", tc.url, err)
			}
		})
	}
}

func TestValidateURL_BlocksLinkLocalAndMetadata(t *testing.T) {
	ctx := context.Background()
	blocked := []string{
		"http://169.254.169.254/latest/meta-data/", // AWS/GCP metadata
		"http://169.254.1.1/",                       // IPv4 link-local
		"http://[fe80::1]/",                         // IPv6 link-local
	}
	for _, u := range blocked {
		if err := ValidateURL(ctx, u, Policy{}); !errors.Is(err, ErrBlockedAddress) {
			t.Errorf("ValidateURL(%q) = %v; want ErrBlockedAddress", u, err)
		}
	}
}

func TestValidateURL_AllowsInternalAndLoopback(t *testing.T) {
	ctx := context.Background()
	// The webhook use case is internal services: loopback + RFC1918 are allowed.
	allowed := []string{
		"http://127.0.0.1:8080/hook",
		"http://10.0.0.5/hook",
		"https://192.168.1.10/hook",
		"http://172.16.0.1/hook",
	}
	for _, u := range allowed {
		if err := ValidateURL(ctx, u, Policy{}); err != nil {
			t.Errorf("ValidateURL(%q) = %v; want nil (internal target allowed)", u, err)
		}
	}
}

func TestValidateURL_AllowOverridesDefaultBlock(t *testing.T) {
	ctx := context.Background()
	policy := Policy{Allow: []*net.IPNet{mustCIDR(t, "169.254.0.0/16")}}
	if err := ValidateURL(ctx, "http://169.254.169.254/", policy); err != nil {
		t.Errorf("explicit allow did not override default block: %v", err)
	}
}

func TestValidateURL_DenyBlocksAllowedRange(t *testing.T) {
	ctx := context.Background()
	policy := Policy{Deny: []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}}
	if err := ValidateURL(ctx, "http://10.1.2.3/", policy); !errors.Is(err, ErrBlockedAddress) {
		t.Errorf("deny rule did not block 10.1.2.3: %v", err)
	}
	// A non-denied internal address still passes.
	if err := ValidateURL(ctx, "http://192.168.0.1/", policy); err != nil {
		t.Errorf("unrelated address blocked by deny rule: %v", err)
	}
}

func TestPolicyFromEnv(t *testing.T) {
	t.Setenv(EnvAllowVar, "169.254.0.0/16, 203.0.113.5")
	t.Setenv(EnvDenyVar, "10.0.0.0/8")
	p, err := PolicyFromEnv()
	if err != nil {
		t.Fatalf("PolicyFromEnv: %v", err)
	}
	if len(p.Allow) != 2 {
		t.Errorf("Allow len = %d; want 2 (CIDR + bare IP)", len(p.Allow))
	}
	if len(p.Deny) != 1 {
		t.Errorf("Deny len = %d; want 1", len(p.Deny))
	}
	// The bare IP was widened to a single-host net that contains itself.
	if !p.Allow[1].Contains(net.ParseIP("203.0.113.5")) {
		t.Error("bare IP allow entry does not contain its own address")
	}

	t.Setenv(EnvAllowVar, "not-an-ip")
	if _, err := PolicyFromEnv(); err == nil {
		t.Error("PolicyFromEnv accepted an invalid entry; want error")
	}
}

func TestValidateURL_ResolveFailure(t *testing.T) {
	// .invalid is reserved (RFC 2606) and never resolves — offline-safe.
	err := ValidateURL(context.Background(), "http://nonexistent.invalid/hook", Policy{})
	if !errors.Is(err, ErrResolve) {
		t.Errorf("ValidateURL(unresolvable) = %v; want ErrResolve", err)
	}
}

func TestSafeClient_RefusesRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.1:1/other", http.StatusFound)
	}))
	defer srv.Close()

	client := SafeClient(Policy{}, 5*time.Second)
	resp, err := client.Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected redirect to be refused, got nil error")
	}
	if !errors.Is(err, ErrRedirect) {
		t.Errorf("redirect error = %v; want ErrRedirect", err)
	}
}

func TestSafeClient_DialRejectsBlockedIP(t *testing.T) {
	client := SafeClient(Policy{}, 3*time.Second)
	// The literal metadata IP is rejected at dial time (no network needed).
	_, err := client.Get("http://169.254.169.254/latest/")
	if !errors.Is(err, ErrBlockedAddress) {
		t.Errorf("dial to metadata IP = %v; want ErrBlockedAddress", err)
	}
}

func TestSafeClient_DeliversToLoopback(t *testing.T) {
	var hit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := SafeClient(Policy{}, 5*time.Second)
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("loopback delivery failed: %v", err)
	}
	_ = resp.Body.Close()
	if !hit {
		t.Error("receiver was not hit")
	}
}
