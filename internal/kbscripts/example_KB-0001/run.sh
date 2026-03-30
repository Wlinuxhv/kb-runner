#!/bin/bash
# CASE: 示例 KB
# KB ID: 00001
# 描述：演示新的得分计算系统的示例 KB 脚本

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

source "$PROJECT_ROOT/scripts/bash/api.sh"

trap 'kb_save; echo "CASE 执行异常中断"; exit 1' INT TERM

kb_init

# 步骤 1: 检查系统环境（权重 0.3，期望状态 success）
echo "执行步骤 1：检查系统环境"
step_start "检查系统环境"

if [ -f "/etc/os-release" ]; then
    result "os" "linux"
    result "os_release" "$(grep PRETTY_NAME /etc/os-release | cut -d= -f2 | tr -d '"')"
    step_success "系统环境检查通过"
elif [ -f "/etc/redhat-release" ]; then
    result "os" "linux"
    result "release" "$(cat /etc/redhat-release)"
    step_success "系统环境检查通过"
else
    result "os" "unknown"
    step_success "系统环境检查完成（未知系统）"
fi

# 步骤 2: 执行测试（权重 0.4，期望状态 success）
echo "执行步骤 2：执行测试"
step_start "执行测试"

# 模拟一些测试
test_items=0
passed_items=0

# 测试 1：检查临时目录
if [ -d "/tmp" ]; then
    result "test_tmp_dir" "pass"
    passed_items=$((passed_items + 1))
else
    result "test_tmp_dir" "fail"
fi
test_items=$((test_items + 1))

# 测试 2：检查 home 目录
if [ -d "$HOME" ]; then
    result "test_home_dir" "pass"
    passed_items=$((passed_items + 1))
else
    result "test_home_dir" "fail"
fi
test_items=$((test_items + 1))

# 测试 3：检查当前工作目录
if [ -n "$PWD" ]; then
    result "test_pwd" "pass"
    result "pwd" "$PWD"
    passed_items=$((passed_items + 1))
else
    result "test_pwd" "fail"
fi
test_items=$((test_items + 1))

result "test_summary" "passed: $passed_items/$test_items"

if [ $passed_items -eq $test_items ]; then
    step_success "测试执行完成，全部通过"
elif [ $passed_items -gt 0 ]; then
    step_warning "测试执行完成，部分通过"
else
    step_failure "测试执行完成，全部失败"
fi

# 步骤 3: 结果汇报（权重 0.3，期望状态 success）
echo "执行步骤 3：结果汇报"
step_start "结果汇报"

result "execution_time" "$(date '+%Y-%m-%d %H:%M:%S')"
result "test_items" "$test_items"
result "passed_items" "$passed_items"
result "pass_rate" "$(echo "scale=2; $passed_items * 100 / $test_items" | bc 2>/dev/null || echo "N/A")%"

if [ $passed_items -eq $test_items ]; then
    result "conclusion" "所有测试通过，系统正常"
    step_success "结果汇报完成"
else
    result "conclusion" "部分测试未通过，请检查系统"
    step_success "结果汇报完成"
fi

kb_save

echo "CASE 执行完成"
