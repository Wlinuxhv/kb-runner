#!/bin/bash
# CASE: test_case
# 描述: A test case for kbscript loading

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

source "$PROJECT_ROOT/scripts/bash/api.sh"

kb_init

# 示例步骤1: 检查系统环境
step_start "check_environment"
if [ -f "/etc/os-release" ]; then
    result "os" "linux"
    step_success "系统环境检查通过"
else
    result "os" "windows"
    step_success "系统环境检查通过 (Windows)"
fi

# 示例步骤2: 执行测试
step_start "execute_test"
result "test_result" "success"
result "test_param" "$test_param"
step_success "测试执行完成"

kb_save

echo "CASE执行完成"
