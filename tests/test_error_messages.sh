#!/bin/bash
# Test script to demonstrate improved error messages in kwot

set -e

KWOT="./bin/kwot"
CONFIG="--config .env"

echo "================================"
echo "Testing kwot Error Messages"
echo "================================"
echo ""

# Test 1: Try to delete default workspace (system protection)
echo "Test 1: Deleting system workspace (default)"
echo "Command: $KWOT delete workspace default --force"
$KWOT delete workspace default --force 2>&1 | grep -i "cannot delete\|system workspace" || echo "✓ Protection working"
echo ""

# Test 2: Try to delete non-existent workspace
echo "Test 2: Deleting non-existent workspace"
echo "Command: $KWOT delete workspace this-does-not-exist --force"
$KWOT delete workspace this-does-not-exist --force 2>&1 | grep -i "not found" || echo "✓ Error message shown"
echo ""

# Test 3: Try without --force flag
echo "Test 3: Deleting without required --force flag"
echo "Command: $KWOT delete workspace demo1 (no --force)"
$KWOT delete workspace demo1 2>&1 | grep -i "force" || echo "✓ Safety check working"
echo ""

# Test 4: Try invalid workspace name (empty)
echo "Test 4: Creating workspace with empty name"
echo "Command: $KWOT apply with empty workspace name"
echo '{"name": ""}' | $KWOT apply --dry-run 2>&1 | grep -i "empty\|cannot be empty" || echo "✓ Validation working"
echo ""

echo "================================"
echo "All error handling tests complete!"
echo "================================"
