#!/usr/bin/env bash
#
# challenges/concurrency_describe_challenge.sh
#
# Round-243 deliverable — Concurrency submodule deep-doc + test-matrix
# enrichment (mirror round-220 DocProcessor template).
#
# Drives the full CONST-050(B) "Challenges" leg for the Concurrency
# submodule:
#
#   Step 1: pre-build  -- go vet + go build
#   Step 2: post-build -- go test ./... -count=1 -race
#   Step 3: fixture load -- assert both EN+SR fixtures exist + non-empty
#   Step 4: runtime end-to-end -- run challenges/runner exercising the
#                                  real pool / queue / limiter / breaker
#                                  / semaphore primitives and asserting
#                                  every output line carries the locale-
#                                  correct rendering
#   Step 5: paired anti-bluff mutation -- corrupt one SR fixture entry,
#                                          re-run, expect non-zero exit
#                                          (specifically exit 99 when the
#                                          mutation-only mode is forced)
#
# Anti-bluff invariants (CONST-035 / Article XI §11.9):
#   - every PASS is preceded by a real command + captured output
#   - the mutation leg PROVES the runner's assertions are real (if the
#     runner exits 0 on a corrupted bilingual fixture, the assertions
#     are bluffs and the whole suite fails loud)
#   - the script exits non-zero on the FIRST failure (no quiet skips)
#
# Flags:
#   --anti-bluff-mutate   run ONLY the paired-mutation leg; exit 99 on
#                         expected mutation-rejected, exit 0 only if the
#                         runner FAILED to detect the mutation (= bluff)
#
# Exit codes:
#   0   -- every step green; concurrency primitives confirmed working
#   1   -- pre-build / post-build / fixture / runtime failure
#   99  -- (paired-mutation mode) mutation correctly rejected by runner
#          -- the OPERATOR-facing meaning of 99 is "anti-bluff guard
#           verified": the runner's assertions DID notice the corrupted
#           fixture, so the green run in full-pipeline mode is meaningful

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
EVIDENCE_DIR="${SCRIPT_DIR}/.last-run"
mkdir -p "${EVIDENCE_DIR}"

cd "${REPO_ROOT}"

MODE="full"
if [[ "${1:-}" == "--anti-bluff-mutate" ]]; then
    MODE="mutate-only"
fi

log() { printf '\n=== %s ===\n' "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

EN_FIX="${SCRIPT_DIR}/fixtures/en.yaml"
SR_FIX="${SCRIPT_DIR}/fixtures/sr-Latn.yaml"

mutation_leg() {
    log "Paired anti-bluff mutation (corrupt SR fixture, expect runner FAIL)"
    [[ -s "${SR_FIX}" ]] || fail "missing SR fixture: ${SR_FIX}"

    local backup="${SR_FIX}.bak.$$"
    cp "${SR_FIX}" "${backup}"
    # belt-and-braces: restore on every exit path
    trap 'mv -f "'"${backup}"'" "'"${SR_FIX}"'" 2>/dev/null || true' EXIT

    # Replace the SR banner with a value that collides with the EN banner.
    # The runner's locale-bluff assertion (en banner != sr banner) MUST
    # catch this. If it does not, the bilingual leg is a CONST-046 bluff.
    sed -i 's|=== Concurrency describe izazov (SR) ===|=== Concurrency describe challenge (EN) ===|' "${SR_FIX}"
    grep -q '=== Concurrency describe challenge (EN) ===' "${SR_FIX}" \
        || fail "mutation did not apply to ${SR_FIX}"

    set +e
    go run ./challenges/runner > "${EVIDENCE_DIR}/06-mutation.log" 2>&1
    local rc=$?
    set -e

    # restore explicitly (EXIT trap is the fallback)
    mv -f "${backup}" "${SR_FIX}"
    trap - EXIT

    if [[ ${rc} -eq 0 ]]; then
        fail "paired-mutation leg: runner exited 0 with corrupted SR fixture -- assertions are not real (CONST-035 bluff)"
    fi
    printf 'mutation correctly rejected with exit code %d\n' "${rc}" \
        | tee -a "${EVIDENCE_DIR}/06-mutation.log"

    # In mutate-only mode, signal the anti-bluff guard verified by exiting 99.
    if [[ "${MODE}" == "mutate-only" ]]; then
        log "PASS (mutate-only): runner rejected corrupted fixture -- assertions are real"
        exit 99
    fi
}

if [[ "${MODE}" == "mutate-only" ]]; then
    mutation_leg
fi

# ---------------------------------------------------------------------------
# Step 1 -- pre-build floor
# ---------------------------------------------------------------------------
log "Step 1: go vet + go build (pre-build floor)"
go vet ./... 2>&1 | tee "${EVIDENCE_DIR}/01-vet.log" || fail "go vet"
go build ./... 2>&1 | tee "${EVIDENCE_DIR}/02-build.log" || fail "go build"

# ---------------------------------------------------------------------------
# Step 2 -- post-build floor: full unit suite under race detector
# ---------------------------------------------------------------------------
log "Step 2: go test ./... -count=1 -race (post-build floor)"
go test ./... -count=1 -race 2>&1 | tee "${EVIDENCE_DIR}/03-test.log" || fail "unit suite"

# ---------------------------------------------------------------------------
# Step 3 -- fixture load sanity
# ---------------------------------------------------------------------------
log "Step 3: bilingual fixture load sanity"
[[ -s "${EN_FIX}" ]] || fail "missing or empty fixture: ${EN_FIX}"
[[ -s "${SR_FIX}" ]] || fail "missing or empty fixture: ${SR_FIX}"
grep -q 'challenge.banner' "${EN_FIX}" || fail "en fixture missing challenge.banner"
grep -q 'challenge.banner' "${SR_FIX}" || fail "sr fixture missing challenge.banner"
printf 'fixtures OK: %s + %s\n' "${EN_FIX}" "${SR_FIX}" | tee "${EVIDENCE_DIR}/04-fixtures.log"  # bluff-scan: ok (evidence-echo of state already asserted by grep -q || fail above; set -e active)

# ---------------------------------------------------------------------------
# Step 4 -- runtime end-to-end: real primitives, bilingual rendering
# ---------------------------------------------------------------------------
log "Step 4: runtime end-to-end (real pool/queue/limiter/breaker/semaphore + bilingual emit)"
go run ./challenges/runner 2>&1 | tee "${EVIDENCE_DIR}/05-runtime.log" || fail "runtime round-trip"

# Verify the captured log carries BOTH banners (proof the bilingual emit
# actually happened — not just the EN leg with a stub SR no-op).
grep -q 'Concurrency describe challenge (EN)' "${EVIDENCE_DIR}/05-runtime.log" \
    || fail "EN banner missing from captured runtime log"
grep -q 'Concurrency describe izazov (SR)' "${EVIDENCE_DIR}/05-runtime.log" \
    || fail "SR banner missing from captured runtime log"

# ---------------------------------------------------------------------------
# Step 5 -- paired anti-bluff mutation
# ---------------------------------------------------------------------------
mutation_leg

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
log "PASS: concurrency_describe_challenge.sh -- all 5 steps green"
printf 'evidence directory: %s\n' "${EVIDENCE_DIR}"
ls -la "${EVIDENCE_DIR}"
exit 0
