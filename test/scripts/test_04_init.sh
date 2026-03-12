#!/bin/bash
# KB Runner - Test 04: Case Init

KB_RUNNER="./bin/kb-runner"

echo "========================================"
echo " Case Init Test"
echo "========================================"
echo ""

passed=0
failed=0
TEST_DIR="./test_output/test_init"

rm -rf "$TEST_DIR"
mkdir -p "$TEST_DIR"

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

test_file() {
    local name="$1"
    local path="$2"
    echo -n "Test: $name ... "
    if [ -f "$path" ]; then
        echo "PASS"
        ((passed++))
    else
        echo "FAIL"
        ((failed++))
    fi
}

test_cmd "init bash" "$KB_RUNNER init test_bash -o $TEST_DIR"
test_file "bash yaml" "$TEST_DIR/test_bash/case.yaml"
test_file "bash sh" "$TEST_DIR/test_bash/run.sh"

test_cmd "init python" "$KB_RUNNER init test_python -l python -o $TEST_DIR"
test_file "python yaml" "$TEST_DIR/test_python/case.yaml"
test_file "python py" "$TEST_DIR/test_python/run.py"

echo ""
if [ $failed -eq 0 ]; then
    echo "Passed: $passed, Failed: $failed"
    exit 0
else
    echo "Passed: $passed, Failed: $failed"
    exit 1
fi
