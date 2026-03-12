#!/bin/bash
# KB Runner - Test 02: Case Management

KB_RUNNER="./bin/kb-runner"

echo "========================================"
echo " Case Management Test"
echo "========================================"
echo ""

passed=0
failed=0

test_cmd() {
    local name="$1"
    local cmd="$2"
    echo -n "Test: $name ... "
    $cmd > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo "PASS"
        ((passed++))
    else
        echo "FAIL"
        ((failed++))
    fi
}

test_cmd "list" "$KB_RUNNER list"
test_cmd "category filter" "$KB_RUNNER list --category security"
test_cmd "tag filter" "$KB_RUNNER list --tags critical"
test_cmd "search" "$KB_RUNNER list --search check"
test_cmd "show" "$KB_RUNNER show security_check"

echo ""
if [ $failed -eq 0 ]; then
    echo "Passed: $passed, Failed: $failed"
    exit 0
else
    echo "Passed: $passed, Failed: $failed"
    exit 1
fi
