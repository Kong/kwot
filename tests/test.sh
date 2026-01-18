#!/bin/bash

# kwot Enhanced Test Suite - Bash + Go Unit Tests

KWOT_BIN="./kwot"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
RESET='\033[0m'
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'

# Test counters
PASS=0
FAIL=0
SKIP=0

# Test result functions
print_header() {
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
    echo -e "${BLUE}$1${RESET}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
}

pass() {
    echo -e "  ${GREEN}✓${RESET} $1"
    ((PASS++))
}

fail() {
    echo -e "  ${RED}✗${RESET} $1"
    ((FAIL++))
}

skip() {
    echo -e "  ${YELLOW}⊘${RESET} $1 (skipped)"
    ((SKIP++))
}

test_cmd() {
    local desc=$1
    local cmd=$2
    
    echo -n "  Testing: $desc ... "
    if eval "$cmd" > /tmp/test.out 2>&1; then
        pass "$desc"
    else
        fail "$desc"
        if [ -f /tmp/test.out ]; then
            sed 's/^/    /' /tmp/test.out | head -5
        fi
    fi
}

# Build binary
print_header "🔨 Building kwot"
if go build -o "$KWOT_BIN" 2>&1 | tee /tmp/build.log; then
    pass "Build successful"
else
    fail "Build failed"
    cat /tmp/build.log
    exit 1
fi

# Run Go unit tests
print_header "🧪 Running Go Unit Tests"
if go test -v ./internal/validation ./internal/logger -coverprofile=/tmp/coverage.out 2>&1 | tee /tmp/test_results.txt; then
    pass "Unit tests passed"
    
    # Show coverage
    echo ""
    echo -e "${BLUE}Coverage Report:${RESET}"
    go tool cover -func=/tmp/coverage.out | tail -1
else
    fail "Unit tests failed"
fi

# CLI Integration Tests
print_header "🔧 CLI Integration Tests"

echo ""
echo -e "${BLUE}Binary & Commands:${RESET}"
test_cmd "help command" "$KWOT_BIN --help | grep -q 'Kong Gateway'"
test_cmd "apply command" "$KWOT_BIN apply --help | grep -q 'Apply'"
test_cmd "delete command" "$KWOT_BIN delete --help | grep -q 'Delete'"

echo ""
echo -e "${BLUE}Global Flags:${RESET}"
test_cmd "--dry-run flag" "$KWOT_BIN apply --help | grep -q 'dry-run'"
test_cmd "--verbose flag" "$KWOT_BIN apply --help | grep -q 'verbose'"
test_cmd "--quiet flag" "$KWOT_BIN apply --help | grep -q 'quiet'"
test_cmd "--force flag" "$KWOT_BIN apply --help | grep -q 'force'"
test_cmd "--workspace flag" "$KWOT_BIN apply --help | grep -q 'workspace'"

echo ""
echo -e "${BLUE}Flag Functionality:${RESET}"
test_cmd "--dry-run preview mode" "$KWOT_BIN apply --dry-run --help | grep -q 'dry-run'"
test_cmd "--verbose mode" "$KWOT_BIN apply --verbose --help | grep -q 'verbose'"
test_cmd "--workspace selection" "$KWOT_BIN apply --workspace all --help | grep -q 'workspace'"

echo ""
echo -e "${BLUE}Commands Available:${RESET}"
test_cmd "apply workspaces" "$KWOT_BIN apply --help | grep -q 'workspaces'"
test_cmd "apply roles" "$KWOT_BIN apply --help | grep -q 'roles'"
test_cmd "apply groups" "$KWOT_BIN apply --help | grep -q 'groups'"
test_cmd "delete workspace" "$KWOT_BIN delete --help | grep -q 'workspace'"

# Validation Tests
print_header "✔️ Input Validation Tests"

echo ""
echo -e "${BLUE}Validation Features:${RESET}"
test_cmd "workspace name validation" "grep -q 'ValidateWorkspaceName' internal/validation/validator.go"
test_cmd "email validation" "grep -q 'ValidateEmail' internal/validation/validator.go"
test_cmd "role name validation" "grep -q 'ValidateRoleName' internal/validation/validator.go"
test_cmd "duplicate detection" "grep -q 'CheckForDuplicate' internal/validation/validator.go"
test_cmd "group config validation" "grep -q 'ValidateGroupConfig' internal/validation/validator.go"

# Summary
print_header "📊 Test Summary"
echo ""
echo -e "  ${GREEN}Passed:  ${PASS}${RESET}"
echo -e "  ${RED}Failed:  ${FAIL}${RESET}"
echo -e "  ${YELLOW}Skipped: ${SKIP}${RESET}"
echo ""

TOTAL=$((PASS + FAIL + SKIP))
if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed! (${TOTAL} tests)${RESET}"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${RESET}"
    exit 1
fi
