// Comax Secrets website — Next.js App Router config.
//
// MDX is compiled at build time via next-mdx-remote/rsc (see lib/mdx.ts), NOT
// @next/mdx page routes, so no pageExtensions/withMDX wiring is needed here.
// This file stays minimal: security headers + strict mode.

const isProd = process.env.NODE_ENV === 'production';

const securityHeaders = [
  { key: 'X-Content-Type-Options', value: 'nosniff' },
  { key: 'X-Frame-Options', value: 'DENY' },
  { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
  {
    key: 'Permissions-Policy',
    value: 'camera=(), microphone=(), geolocation=()',
  },
];

// HSTS + CSP only in production: dev (`next dev`) needs eval for HMR and would
// break on upgrade-insecure-requests over http://localhost. This is a static
// (SSG) site with no per-request nonce, so the inline theme no-flash script and
// the JSON-LD block require script-src 'unsafe-inline'; the remaining
// directives (frame-ancestors, base-uri, object-src, form-action, and the
// self-scoped default/img/font/connect sources) are pure hardening. All site
// assets are same-origin (no CDN), verified before adding this. Nonce/hash-based
// script-src tightening is tracked as a follow-up.
if (isProd) {
  securityHeaders.push(
    {
      key: 'Strict-Transport-Security',
      value: 'max-age=63072000; includeSubDomains; preload',
    },
    {
      key: 'Content-Security-Policy',
      value: [
        "default-src 'self'",
        "base-uri 'self'",
        "object-src 'none'",
        "frame-ancestors 'none'",
        "form-action 'self'",
        "img-src 'self' data:",
        "font-src 'self'",
        "style-src 'self' 'unsafe-inline'",
        "script-src 'self' 'unsafe-inline'",
        "connect-src 'self'",
        'upgrade-insecure-requests',
      ].join('; '),
    },
  );
}

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,
  async headers() {
    return [{ source: '/:path*', headers: securityHeaders }];
  },
};

export default nextConfig;
