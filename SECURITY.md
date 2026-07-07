# Security Policy

Comax Secrets is a self-hosted secret-management tool. Its security posture
matters more than most projects, so please read the threat model before
reporting — several "issues" are deliberate design decisions under our threat
model, not vulnerabilities.

## Threat model (read first)

Comax Secrets assumes the **self-host operator owns the database and the master
key**. External exposure of the server is **not** an operating assumption.

- Secrets are stored encrypted at rest with a server-side symmetric master key
  (AES-256-GCM). This is **not** end-to-end / zero-knowledge encryption — an
  operator with filesystem access to the DB and master key can read secrets.
  That is by design (self-host = you own your data).
- The full threat model, including operator obligations (master-key file
  permissions, boot-refusal on weak permissions), lives in
  [`docs/threat-model.md`](docs/threat-model.md).

The following are **out of scope** because they follow from the threat model,
not from a defect:

- An operator (or anyone with host/filesystem access to the DB + master key)
  can decrypt stored secrets.
- Secrets are visible to the child process launched by `secret run` — that is
  the feature.
- The GitHub Action `run` input executes the shell command you supply; the
  caller is responsible for not passing untrusted data into it.

## Reporting a vulnerability

**Do not open a public issue for security reports.**

1. Preferred: use GitHub's **private vulnerability reporting** —
   *Security → Report a vulnerability* on this repository. This keeps the
   report private until a fix ships.
2. Alternative: email the maintainer at **skypark207@gmail.com** with the
   subject line `comax-secrets security`.

Please include: affected version/commit, a reproduction, the impact, and
(if known) a suggested fix. We aim to acknowledge within 5 business days.

## Supported versions

Until the first stable `v1.0.0`, only the latest release line receives security
fixes. After `v1.0.0`, the current major version is supported.

| Version | Supported |
| ------- | --------- |
| latest release | ✅ |
| older pre-1.0 tags | ❌ |

## Disclosure

We follow coordinated disclosure: a fix (or mitigation + advisory) is prepared
privately, released, and then the advisory is published. Reporters are credited
unless they prefer to remain anonymous.
