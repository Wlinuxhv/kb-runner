#!/bin/bash
# KB Runner - Test 03: Scenario Management

KB_RUNNER="./bin/kb-runner"

echo "========================================"
echo " Scenario Management Test"
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

test_cmd "scenario list" "$KB_RUNNER scenario list"
test_cmd "scenario show" "$KB_RUNNER scenario show daily_check"

echo ""
if [ $failed -eq 0 ]; then
    echo "Passed: $passed, Failed: $failed"
    exit 0
else
    echo "Passed: $passed, Failed: $failed"
    exit 1
fi
