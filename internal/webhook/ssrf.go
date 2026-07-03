package webhook

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// SSRF policy errors. Callers match with errors.Is.
var (
	// ErrInvalidURL is returned when a webhook URL is malformed or uses a
	// scheme other than http/https.
	ErrInvalidURL = errors.New("webhook: invalid url")
	// ErrResolve is returned when a URL host cannot be resolved to any IP.
	ErrResolve = errors.New("webhook: dns resolution failed")
	// ErrBlockedAddress is returned when a resolved IP is blocked by the SSRF
	// policy (link-local / cloud-metadata by default, or an operator deny rule).
	ErrBlockedAddress = errors.New("webhook: address blocked by policy")
	// ErrRedirect is returned by SafeClient when a response tries to redirect;
	// webhooks never follow redirects.
	ErrRedirect = errors.New("webhook: redirects are not followed")
)

// Policy is the SSRF guard for webhook URLs. The webhook use case is calling
// INTERNAL services (Docker overlay networks are RFC1918), so private and
// loopback ranges are intentionally allowed. What is NOT a legitimate webhook
// target is a link-local / cloud-metadata address (169.254.0.0/16, fe80::/10,
// notably 169.254.169.254) — those are blocked by default.
//
// Operators tune the boundary with two CIDR/IP lists: Allow overrides the
// default block (to permit a specific link-local target), and Deny adds ranges
// to reject (to fence off internal control planes). Allow is evaluated first.
type Policy struct {
	Allow []*net.IPNet // explicit allow — overrides the default link-local block
	Deny  []*net.IPNet // explicit deny — rejected even if otherwise allowed
}

// EnvAllowVar and EnvDenyVar name the environment variables parsed by
// PolicyFromEnv into Policy.Allow / Policy.Deny.
const (
	EnvAllowVar = "COMAX_WEBHOOK_ALLOW"
	EnvDenyVar  = "COMAX_WEBHOOK_DENY"
)

// PolicyFromEnv builds a Policy from COMAX_WEBHOOK_ALLOW / COMAX_WEBHOOK_DENY.
// Each variable is a comma-separated list of CIDRs ("10.0.0.0/8") or bare IPs
// ("169.254.169.254", widened to /32 or /128). An unparseable entry is a hard
// error so a typo never silently widens or narrows the boundary.
func PolicyFromEnv() (Policy, error) {
	allow, err := parseCIDRList(os.Getenv(EnvAllowVar))
	if err != nil {
		return Policy{}, fmt.Errorf("%s: %w", EnvAllowVar, err)
	}
	deny, err := parseCIDRList(os.Getenv(EnvDenyVar))
	if err != nil {
		return Policy{}, fmt.Errorf("%s: %w", EnvDenyVar, err)
	}
	return Policy{Allow: allow, Deny: deny}, nil
}

// parseCIDRList parses a comma-separated list of CIDRs or bare IPs into nets.
func parseCIDRList(raw string) ([]*net.IPNet, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []*net.IPNet
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if _, n, err := net.ParseCIDR(tok); err == nil {
			out = append(out, n)
			continue
		}
		// Bare IP → single-host net.
		ip := net.ParseIP(tok)
		if ip == nil {
			return nil, fmt.Errorf("%q is not a CIDR or IP", tok)
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	return out, nil
}

// allows reports whether ip may be contacted under this policy, with a short
// reason when not. Order: explicit Allow wins, then explicit Deny, then the
// built-in link-local/metadata block. Everything else (loopback, RFC1918,
// public) is permitted — the webhook use case needs internal targets.
func (p Policy) allows(ip net.IP) (bool, string) {
	for _, n := range p.Allow {
		if n.Contains(ip) {
			return true, ""
		}
	}
	for _, n := range p.Deny {
		if n.Contains(ip) {
			return false, "denied by COMAX_WEBHOOK_DENY policy"
		}
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false, "link-local / cloud-metadata address blocked"
	}
	if ip.IsUnspecified() {
		return false, "unspecified address blocked"
	}
	return true, ""
}

// ValidateURL enforces the SSRF policy on a webhook URL: it must be http/https,
// have a non-empty host, resolve to at least one IP, and EVERY resolved IP must
// be allowed by policy. A literal IP host short-circuits DNS.
//
// This is the registration/pre-flight check. Because DNS can change between
// this call and an actual delivery, SafeClient re-validates at dial time — the
// two together close the DNS-rebinding TOCTOU window.
func ValidateURL(ctx context.Context, raw string, policy Policy) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: scheme %q (only http/https allowed)", ErrInvalidURL, u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%w: missing host", ErrInvalidURL)
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrResolve, host)
	}
	if len(ips) == 0 {
		return fmt.Errorf("%w: %s resolved to no addresses", ErrResolve, host)
	}
	for _, ip := range ips {
		if ok, reason := policy.allows(ip); !ok {
			return fmt.Errorf("%w: %s -> %s: %s", ErrBlockedAddress, host, ip, reason)
		}
	}
	return nil
}

// SafeClient returns an *http.Client hardened for outbound webhook delivery:
//
//   - CheckRedirect refuses every redirect (a 3xx can't bounce a request onto a
//     blocked internal address after the initial check passed).
//   - DialContext re-resolves the host, re-validates EVERY candidate IP against
//     the policy, and dials the validated IP explicitly — so even if DNS
//     rebinds to 169.254.169.254 between ValidateURL and the dial, the
//     connection is refused (DNS-rebinding TOCTOU defense).
//
// timeout bounds the whole request (connect + headers + body).
func SafeClient(policy Policy, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		Proxy: nil, // never route webhook delivery through a proxy
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("%w: %s", ErrResolve, host)
			}
			var lastErr error
			for _, ip := range ips {
				if ok, reason := policy.allows(ip); !ok {
					// One blocked IP fails the whole dial: a rebinding attack
					// that returns [good, bad] must not succeed via the good one.
					return nil, fmt.Errorf("%w: %s -> %s: %s", ErrBlockedAddress, host, ip, reason)
				}
			}
			for _, ip := range ips {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return ErrRedirect
		},
	}
}
