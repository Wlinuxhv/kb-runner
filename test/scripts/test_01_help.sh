#!/bin/bash
# KB Runner - Test 01: Help Commands

KB_RUNNER="./bin/kb-runner"

echo "========================================"
echo " Help Commands Test"
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

test_cmd "help" "$KB_RUNNER --help"
test_cmd "version" "$KB_RUNNER version"
test_cmd "list help" "$KB_RUNNER list --help"
test_cmd "run help" "$KB_RUNNER run --help"
test_cmd "init help" "$KB_RUNNER init --help"
test_cmd "scenario help" "$KB_RUNNER scenario --help"

echo ""
if [ $failed -eq 0 ]; then
    echo "Passed: $passed, Failed: $failed"
    exit 0
else
    echo "Passed: $passed, Failed: $failed"
    exit 1
fi
