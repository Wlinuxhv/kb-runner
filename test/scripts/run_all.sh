#!/bin/bash
# KB Runner - Test Runner for Linux/Mac

echo ""
echo "========================================"
echo " KB Runner - Test Runner"
echo "========================================"
echo ""

total_passed=0
total_failed=0

run_test() {
    local script_path="$1"
    local script_name="$2"
    
    echo ""
    echo ">>> Running: $script_name"
    
    bash "$script_path"
    exit_code=$?
    
    if [ $exit_code -eq 0 ]; then
        ((total_passed++))
    else
        ((total_failed++))
    fi
    
    return $exit_code
}

# Linux test scripts
run_test "./test/scripts/test_01_help.sh" "Help Test"
run_test "./test/scripts/test_02_case.sh" "Case Test"
run_test "./test/scripts/test_03_scenario.sh" "Scenario Test"
run_test "./test/scripts/test_04_init.sh" "Init Test"

echo ""
echo "========================================"
echo " Summary"
echo "========================================"
echo ""

total_tests=4

if [ $total_failed -eq 0 ]; then
    echo "All tests passed! ($total_passed/$total_tests)"
    exit 0
else
    echo "Failed tests: $total_failed"
    exit 1
fi
