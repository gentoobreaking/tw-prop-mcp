#!/usr/bin/env bash
# verify.sh — tw-prop-mcp v2.0 verification script
# Spec reference: VERIFICATION_MANUAL.md §75
#
# Executes all verification steps in order:
#   1. specification coverage
#   2. formatting
#   3. static analysis
#   4. unit tests
#   5. integration tests
#   6. database integrity
#   7. ingestion tests
#   8. GIS tests
#   9. comparable tests
#   10. valuation tests
#   11. MCP contract tests
#   12. security tests
#   13. reproducibility tests
#   14. artifact locking tests
#   15. E2E tests
#   16. golden tests
#
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0
FAIL_MESSAGES=()

report_pass() {
    echo -e "${GREEN}✓ PASS${NC}: $1"
    PASS=$((PASS + 1))
}

report_fail() {
    echo -e "${RED}✗ FAIL${NC}: $1"
    FAIL=$((FAIL + 1))
    FAIL_MESSAGES+=("$1")
}

report_info() {
    echo -e "${YELLOW}  → $1${NC}"
}

echo "============================================"
echo "tw-prop-mcp v2.0 Verification"
echo "Commit: $(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
echo "Date: $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
echo "============================================"
echo ""

# -------------------------------------------------
# Step 1: Specification coverage
# -------------------------------------------------
echo "--- Step 1: Specification Coverage ---"
if [ -f "tests/verification/spec_coverage.yaml" ]; then
    report_pass "spec_coverage.yaml exists"
else
    report_fail "spec_coverage.yaml missing"
fi

# -------------------------------------------------
# Step 2: Formatting
# -------------------------------------------------
echo ""
echo "--- Step 2: Formatting ---"
if git diff --name-only --diff-filter=M 2>/dev/null | xargs gofmt -l 2>/dev/null | grep -v node_modules | grep -q .; then
    report_fail "gofmt drift in modified files"
    git diff --name-only --diff-filter=M 2>/dev/null | xargs gofmt -l 2>/dev/null | grep -v node_modules | head -5
else
    report_pass "All modified Go files formatted"
fi

# Frontend formatting
if [ -d "frontend" ]; then
    if (cd frontend && npx prettier --check "src/**/*.{ts,tsx}" 2>/dev/null); then
        report_pass "Frontend files formatted"
    else
        report_fail "Frontend formatting issues"
    fi
fi

# -------------------------------------------------
# Step 3: Static analysis
# -------------------------------------------------
echo ""
echo "--- Step 3: Static Analysis ---"
if go build ./... 2>&1; then
    report_pass "go build"
else
    report_fail "go build"
fi

if go vet -tags=e2e ./... 2>&1; then
    report_pass "go vet"
else
    report_fail "go vet"
fi

# -------------------------------------------------
# Step 4: Unit tests
# -------------------------------------------------
echo ""
echo "--- Step 4: Unit Tests ---"
if go test $(go list ./internal/... | grep -v '/internal/config') -count=1 -timeout 60s 2>&1; then
    report_pass "unit tests (all internal packages except config — requires PostgreSQL container)"
else
    report_fail "unit tests (some packages failed)"
fi

# -------------------------------------------------
# Step 5: Integration tests
# -------------------------------------------------
echo ""
echo "--- Step 5: Integration Tests ---"
if go test -tags=integration ./tests/artifact_lock/... -count=1 -timeout 120s 2>&1; then
    report_pass "artifact lock integration tests"
else
    report_fail "artifact lock integration tests"
fi

# -------------------------------------------------
# Step 6: Database integrity
# -------------------------------------------------
echo ""
echo "--- Step 6: Database Integrity ---"
if [ -d "tests/integration" ]; then
    if go test -tags=integration ./tests/integration/... -count=1 -timeout 60s 2>&1; then
        report_pass "database integrity tests"
    else
        report_fail "database integrity tests (may need PostgreSQL container)"
    fi
fi

# -------------------------------------------------
# Step 7-10: Domain tests
# -------------------------------------------------
echo ""
echo "--- Step 7: Ingestion Tests ---"
if go test ./internal/downloader/... ./internal/parser/... ./internal/normalizer/... ./internal/validator/... ./internal/importpipeline/... -count=1 -timeout 60s 2>&1; then
    report_pass "ingestion tests (download, parse, normalize, validate, pipeline)"
else
    report_fail "ingestion tests"
fi

echo ""
echo "--- Step 8: GIS Tests ---"
if go test ./internal/gis/... ./internal/domain/... -count=1 -timeout 60s 2>&1; then
    report_pass "GIS + domain tests"
else
    report_fail "GIS + domain tests"
fi

echo ""
echo "--- Step 9: Comparable Tests ---"
if go test ./internal/comparable/... ./internal/statistics/... -count=1 -timeout 60s 2>&1; then
    report_pass "comparable + statistics tests"
else
    report_fail "comparable + statistics tests"
fi

echo ""
echo "--- Step 10: Valuation Tests ---"
if go test ./internal/valuation/... -count=1 -timeout 60s 2>&1; then
    report_pass "valuation tests"
else
    report_fail "valuation tests"
fi

# -------------------------------------------------
# Step 11: MCP contract tests
# -------------------------------------------------
echo ""
echo "--- Step 11: MCP Contract Tests ---"
if go test ./tests/contract/... -count=1 -timeout 60s 2>&1; then
    report_pass "MCP contract tests"
else
    report_fail "MCP contract tests"
fi

# -------------------------------------------------
# Step 12: Security tests (AI isolation)
# -------------------------------------------------
echo ""
echo "--- Step 12: Security / AI Isolation Tests ---"
if go test ./tests/isolation/... -count=1 -timeout 60s 2>&1; then
    report_pass "AI isolation tests"
else
    report_fail "AI isolation tests"
fi

# -------------------------------------------------
# Step 13: Reproducibility tests
# -------------------------------------------------
echo ""
echo "--- Step 13: Reproducibility Tests ---"
if go test ./tests/reproducibility/... -count=1 -timeout 60s 2>&1; then
    report_pass "reproducibility tests"
else
    report_fail "reproducibility tests"
fi

# -------------------------------------------------
# Step 14: Artifact locking tests
# -------------------------------------------------
echo ""
echo "--- Step 14: Artifact Lock Tests ---"
if go test -tags=integration ./tests/artifact_lock/... -count=1 -timeout 120s 2>&1; then
    report_pass "artifact lock tests"
else
    report_fail "artifact lock tests"
fi

# -------------------------------------------------
# Step 15: E2E tests
# -------------------------------------------------
echo ""
echo "--- Step 15: E2E Acceptance Tests ---"
if go test -tags=e2e ./tests/e2e/... -count=1 -timeout 60s 2>&1; then
    report_pass "E2E acceptance tests"
else
    report_fail "E2E acceptance tests"
fi

# -------------------------------------------------
# Step 16: Golden tests
# -------------------------------------------------
echo ""
echo "--- Step 16: Golden Tests ---"
if [ -d "tests/golden" ]; then
    report_pass "golden test directory exists"
else
    report_info "tests/golden/ does not exist — skipping golden tests"
    report_fail "golden tests directory missing (TEST-GOLDEN-001)"
fi

# -------------------------------------------------
# Benchmarks (informational)
# -------------------------------------------------
echo ""
echo "--- Benchmarks (real MOI data) ---"
if go test -bench=. -benchmem -run=^$ ./internal/importpipeline/... -benchtime=1s -timeout 120s 2>&1 | tail -10; then
    report_pass "benchmarks pass"
else
    report_fail "benchmarks"
fi

# -------------------------------------------------
# Frontend type check
# -------------------------------------------------
if [ -d "frontend" ]; then
    echo ""
    echo "--- Frontend TypeScript ---"
    if (cd frontend && npx tsc --noEmit 2>&1); then
        report_pass "frontend TypeScript type check"
    else
        report_fail "frontend TypeScript errors"
    fi
fi

# -------------------------------------------------
# Summary
# -------------------------------------------------
echo ""
echo "============================================"
echo "Verification Summary"
echo "============================================"
echo -e "Passed: ${GREEN}${PASS}${NC}"
echo -e "Failed: ${RED}${FAIL}${NC}"

if [ ${FAIL} -gt 0 ]; then
    echo ""
    echo -e "${RED}FAILED TESTS:${NC}"
    for msg in "${FAIL_MESSAGES[@]}"; do
        echo -e "  ${RED}✗ ${msg}${NC}"
    done
    echo ""
    echo -e "${RED}VERIFICATION: FAIL${NC}"
    exit 1
else
    echo ""
    echo -e "${GREEN}VERIFICATION: PASS${NC}"
    echo "All verification steps completed successfully."
    exit 0
fi
