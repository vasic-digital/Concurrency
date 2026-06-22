# Concurrency — Test-Type Coverage Matrix

**Authority**: CONST-050(B) "100%-Test-Type-Coverage" mandate (cascaded from HelixConstitution submodule §11.4.27).
**Scope**: this document is the Concurrency submodule's coverage ledger. It enumerates every test type CONST-050(B) recognises and records the current status against Concurrency's surface (`pkg/pool`, `pkg/queue`, `pkg/limiter`, `pkg/breaker`, `pkg/semaphore`, `pkg/monitor`, `pkg/bulkhead`, `pkg/lazyloader`, `pkg/retry`, `pkg/safe`, `pkg/gin`).

A row may be `covered`, `planned`, or `n/a (out of scope for a library of this shape)`. `n/a` rows MUST justify themselves — silent omission is a CONST-048 violation per §11.4.25.

---

## Coverage Ledger

| Test type        | Status   | Artefact / location                                                                                                                          | Notes |
|------------------|----------|----------------------------------------------------------------------------------------------------------------------------------------------|-------|
| Unit             | covered  | `pkg/<name>/<name>_test.go`, `pkg/pool/pool_edge_test.go`, etc.                                                                              | Mocks permitted per CONST-050(A); race-detector enforced; per-primitive edge tests exist for pool / queue / limiter / breaker / semaphore. |
| Integration      | covered  | `tests/integration/*_test.go`                                                                                                                | Multi-component scenarios exercising the primitives together (pool + limiter, breaker + queue, etc.) against real implementations — no mocks. |
| E2E              | covered  | `challenges/concurrency_describe_challenge.sh` + `challenges/runner/main.go`                                                                 | Bash-orchestrated end-to-end exercise of every primitive with bilingual (EN+SR) human-readable summary; paired anti-bluff mutation included. |
| Full automation  | covered  | `challenges/scripts/concurrency_compile_challenge.sh`, `concurrency_functionality_challenge.sh`, `concurrency_unit_challenge.sh`             | Pre-existing scripts confirm package layout, exported symbols, and the unit suite. Round-243 added the describe runner + paired-mutation gate. |
| Security         | covered  | `tests/security/*_test.go`                                                                                                                   | Race-detector enforced on every test invocation; concurrent-access fuzz on pool and semaphore. |
| DDoS             | covered  | `challenges/scripts/ddos_health_flood_challenge.sh`                                                                                          | Request-flood resilience tested against the gin handler surface (`pkg/gin`). |
| Scaling          | covered  | `challenges/scripts/scaling_horizontal_challenge.sh`                                                                                         | Horizontal-scale behaviour of the worker pool under linear load growth. |
| Chaos            | covered  | `challenges/scripts/chaos_failure_injection_challenge.sh`                                                                                    | Controlled failure injection into pool tasks + breaker probe path. |
| Stress           | covered  | `tests/stress/*_test.go` + `challenges/scripts/stress_sustained_load_challenge.sh`                                                           | Sustained load above the advertised tier on pool, queue, and limiter. |
| Performance      | planned  | recommend: `BenchmarkWorkerPool_Submit`, `BenchmarkPriorityQueue_PushPop`, `BenchmarkTokenBucket_Allow` with `b.ReportAllocs()` + p95 baseline | `tests/benchmark/` directory exists but currently empty of benchmark functions per `[no tests to run]` output. Owed for a follow-up round. |
| Benchmarking     | planned  | recommend: micro-benchmarks listed above + macro-benchmark inside the project's profile-run                                                      | Macro tier lives outside Concurrency (CONST-051(B)). |
| UI               | covered  | `challenges/scripts/ui_terminal_interaction_challenge.sh`                                                                                    | Terminal-surface assertions (Concurrency ships no graphical UI). |
| UX               | covered  | `challenges/scripts/ux_end_to_end_flow_challenge.sh` + bilingual leg of `challenges/concurrency_describe_challenge.sh`                       | UX dimension Concurrency actually owns: locale-aware rendering of the describe summary. Asserted EN/SR distinct banners. |
| Challenges       | covered  | `challenges/concurrency_describe_challenge.sh` (round 243) + 11 pre-existing `challenges/scripts/*.sh`                                       | Incorporates the `vasic-digital/Challenges` pattern; captures stdout/stderr as wire evidence per §11.4.2; paired mutation per §1.1 / CONST-055 meta-test. |
| HelixQA          | planned  | recommend: register Concurrency as a target in HelixQA's autonomous QA bank                                                                  | HelixQA submodule (`HelixDevelopment/HelixQA`) is incorporated at the project root per CONST-050; Concurrency enrolment is a project-meta-repo task, not a Concurrency-internal task. |

---

## Anti-Bluff Posture

Every `covered` row above carries captured runtime evidence:

- **Unit / Integration / Security / Stress**: `go test ./... -count=1 -race` exits 0; captured in `challenges/.last-run/03-test.log` after every Challenge run.
- **E2E (Challenge)**: `challenges/concurrency_describe_challenge.sh` writes `challenges/.last-run/` artefacts containing the vet/build log, the test log, the fixture-load log, the runtime stdout, and the mutation-rejection proof.
- **UX**: the Challenge's bilingual leg captures the actual EN vs SR rendered output and diff-asserts both differ from each other AND from the verbatim message-id, ruling out a CONST-046 hardcoded-English regression.
- **Paired mutation**: `challenges/concurrency_describe_challenge.sh --anti-bluff-mutate` runs ONLY the corruption leg and exits 99 when the runner correctly detects the corrupted SR fixture. Exit 0 from this mode would mean the runner's assertions are bluffs and the whole Challenge would fail loudly.

Rows marked `planned` are **deliverables for future rounds**, NOT bluffs — CONST-048 (Six Invariants) tolerates documented gaps in the ledger only when the gap is explicit, dated, and owner-assigned. This document is the explicit register; future rounds will flip rows from `planned` to `covered` with the matching artefact.

---

## Reproduction Recipe

```bash
cd dependencies/vasic-digital/Concurrency
# Full 5-step pipeline:
bash challenges/concurrency_describe_challenge.sh

# Anti-bluff guard alone (expect exit 99):
bash challenges/concurrency_describe_challenge.sh --anti-bluff-mutate
echo "exit=$?"   # 99

# Single-leg runtime exercise:
go run ./challenges/runner
```

Evidence lands in `challenges/.last-run/` (gitignored — re-generated by every run).
