#!/bin/bash
# CASE: test_bash
# 描述: TODO: 在这里填写CASE的描述信息

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

source "$PROJECT_ROOT/scripts/bash/api.sh"

kb_init

# ============================================
# 在这里编写你的检查逻辑
# ============================================

# 示例步骤1: 检查系统环境
step_start "check_environment"
if [ -f "/etc/os-release" ]; then
    result "os" "linux"
    step_success "系统环境检查通过"
else
    step_warning "无法确定操作系统类型"
fi

# 示例步骤2: 执行检查
step_start "execute_check"
# TODO: 在这里添加你的检查逻辑
result "status" "ok"
step_success "检查执行完成"

# ============================================
# 保存结果
# ============================================

kb_save

echo "CASE执行完成"
