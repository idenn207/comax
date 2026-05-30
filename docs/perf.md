# Performance baseline

Single source of truth for the latency budgets the plan calls out, and
where they are enforced.

## Cold start (`secret run -- <cmd>`)

- **Budget (PRD/plan)**: p95 ≤ 300 ms end-to-end (CLI exec → child reads
  the secret).
- **Gate**: `TestSecretRunColdStartP95Budget` in `cmd/cli/bench_test.go`
  fails CI if 20 samples (sorted) exceed the budget at the 95th
  percentile. Override the budget for slow hardware via
  `COMAX_BENCH_BUDGET_MS`.
- **Bench**: `go test -bench=BenchmarkSecretRunColdStart -run=^$
  -benchtime=10x ./cmd/cli` reports `ns/op` for plotting trends.

### Local baseline (recorded 2026-05-31)

| Host | OS | CPU | Median | p95 | bench ns/op |
|---|---|---|---|---|---|
| dev box | Windows 11 | i5-13600KF | ~72 ms | ~80 ms | 75 ms/op |

CI on `ubuntu-latest` is expected to land in the same range or faster
(no Defender, no Windows process spawn overhead). The 300 ms budget
therefore has comfortable headroom — about a 3.7× margin — which is the
right shape: deliberate, not "just barely passing".

## What is measured vs not

End-to-end timing covers:

1. `fork+exec` of `secret`
2. Go runtime init + Cobra subcommand resolution
3. `credentials.json` load (16 KB max in practice)
4. HTTP round-trip to the local server (loopback)
5. AES-256-GCM decrypt of one secret
6. `fork+exec` of a trivial child (`true` on Unix, `cmd /c ver` on
   Windows)

It does NOT cover:

- The first-ever `go build` of the CLI (amortised in the bench setup)
- Cross-machine network latency
- Server-side cold start (the server is a long-running process in
  production)

If the bench regresses on CI, the most likely culprits are:

- A new import added to `cmd/cli/main.go` with heavy `init()` work.
- An accidental synchronous network call during root-cmd construction.
- A new dependency that pulls in CGO (which would also break the
  cross-compile matrix — easier to catch).
