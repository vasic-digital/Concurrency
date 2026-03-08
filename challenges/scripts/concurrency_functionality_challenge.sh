#!/usr/bin/env bash
# concurrency_functionality_challenge.sh - Validates Concurrency module core functionality
# Checks worker pools, rate limiters, circuit breakers, semaphores, and key exported symbols
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MODULE_NAME="Concurrency"

PASS=0
FAIL=0
TOTAL=0

pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }

echo "=== ${MODULE_NAME} Functionality Challenge ==="
echo ""

# --- Section 1: Required packages ---
echo "Section 1: Required packages (6)"

for pkg in pool queue limiter breaker semaphore monitor; do
    echo "Test: Package pkg/${pkg} exists"
    if [ -d "${MODULE_DIR}/pkg/${pkg}" ]; then
        pass "Package pkg/${pkg} exists"
    else
        fail "Package pkg/${pkg} missing"
    fi
done

# --- Section 2: Worker pool types ---
echo ""
echo "Section 2: Worker pool"

echo "Test: WorkerPool struct exists"
if grep -q "type WorkerPool struct" "${MODULE_DIR}/pkg/pool/"*.go 2>/dev/null; then
    pass "WorkerPool struct exists"
else
    fail "WorkerPool struct missing"
fi

echo "Test: Task interface exists"
if grep -q "type Task interface" "${MODULE_DIR}/pkg/pool/"*.go 2>/dev/null; then
    pass "Task interface exists"
else
    fail "Task interface missing"
fi

echo "Test: PoolConfig struct exists"
if grep -q "type PoolConfig struct" "${MODULE_DIR}/pkg/pool/"*.go 2>/dev/null; then
    pass "PoolConfig struct exists"
else
    fail "PoolConfig struct missing"
fi

echo "Test: PoolMetrics struct exists"
if grep -q "type PoolMetrics struct" "${MODULE_DIR}/pkg/pool/"*.go 2>/dev/null; then
    pass "PoolMetrics struct exists"
else
    fail "PoolMetrics struct missing"
fi

# --- Section 3: Rate limiters ---
echo ""
echo "Section 3: Rate limiters"

echo "Test: RateLimiter interface exists"
if grep -q "type RateLimiter interface" "${MODULE_DIR}/pkg/limiter/"*.go 2>/dev/null; then
    pass "RateLimiter interface exists"
else
    fail "RateLimiter interface missing"
fi

echo "Test: TokenBucket struct exists"
if grep -q "type TokenBucket struct" "${MODULE_DIR}/pkg/limiter/"*.go 2>/dev/null; then
    pass "TokenBucket struct exists"
else
    fail "TokenBucket struct missing"
fi

echo "Test: SlidingWindow struct exists"
if grep -q "type SlidingWindow struct" "${MODULE_DIR}/pkg/limiter/"*.go 2>/dev/null; then
    pass "SlidingWindow struct exists"
else
    fail "SlidingWindow struct missing"
fi

# --- Section 4: Circuit breaker ---
echo ""
echo "Section 4: Circuit breaker"

echo "Test: CircuitBreaker struct exists"
if grep -q "type CircuitBreaker struct" "${MODULE_DIR}/pkg/breaker/"*.go 2>/dev/null; then
    pass "CircuitBreaker struct exists"
else
    fail "CircuitBreaker struct missing"
fi

# --- Section 5: Semaphore ---
echo ""
echo "Section 5: Semaphore"

echo "Test: Semaphore struct exists"
if grep -q "type Semaphore struct" "${MODULE_DIR}/pkg/semaphore/"*.go 2>/dev/null; then
    pass "Semaphore struct exists"
else
    fail "Semaphore struct missing"
fi

# --- Section 6: Resource monitor ---
echo ""
echo "Section 6: Resource monitor"

echo "Test: ResourceMonitor struct exists"
if grep -q "type ResourceMonitor struct" "${MODULE_DIR}/pkg/monitor/"*.go 2>/dev/null; then
    pass "ResourceMonitor struct exists"
else
    fail "ResourceMonitor struct missing"
fi

echo "Test: SystemResources struct exists"
if grep -q "type SystemResources struct" "${MODULE_DIR}/pkg/monitor/"*.go 2>/dev/null; then
    pass "SystemResources struct exists"
else
    fail "SystemResources struct missing"
fi

# --- Section 7: Source structure completeness ---
echo ""
echo "Section 7: Source structure"

echo "Test: Each package has non-test Go source files"
all_have_source=true
for pkg in pool queue limiter breaker semaphore monitor; do
    non_test=$(find "${MODULE_DIR}/pkg/${pkg}" -name "*.go" ! -name "*_test.go" -type f 2>/dev/null | wc -l)
    if [ "$non_test" -eq 0 ]; then
        fail "Package pkg/${pkg} has no non-test Go files"
        all_have_source=false
    fi
done
if [ "$all_have_source" = true ]; then
    pass "All packages have non-test Go source files"
fi

echo ""
echo "=== Results: ${PASS}/${TOTAL} passed, ${FAIL} failed ==="
[ "${FAIL}" -eq 0 ] && exit 0 || exit 1
