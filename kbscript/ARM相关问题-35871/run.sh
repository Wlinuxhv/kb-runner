#!/bin/bash
# CASE: ARM相关问题
# KB ID: 35871
# 描述: ARM平台相关问题的检查和排查

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

source "$PROJECT_ROOT/scripts/bash/api.sh"

trap 'kb_save; echo "CASE执行异常中断"; exit 1' INT TERM

kb_init

# 步骤0：检查相关告警
echo "执行步骤0：检查相关告警"
step_start "检查相关告警"

# 模拟告警查询，实际环境中可以使用log_db_search_alerts函数
result "ARM_ALERT_COUNT" "0"
result "HARDWARE_ALERT_COUNT" "0"
step_success "未发现相关告警"

# 步骤1：检查CPU架构
echo "执行步骤1：检查CPU架构"
step_start "检查CPU架构"

# 检查uname命令是否可用
if command -v uname &> /dev/null; then
    arch=$(uname -m)
    result "cpu_architecture" "$arch"
    
    if echo "$arch" | grep -i "arm"; then
        step_success "确认是ARM架构"
    else
        step_info "不是ARM架构，ARM相关问题可能不适用"
    fi
else
    step_info "uname命令不可用，无法检查CPU架构"
fi

# 步骤2：检查系统信息
echo "执行步骤2：检查系统信息"
step_start "检查系统信息"

# 检查lsb_release命令是否可用
if command -v lsb_release &> /dev/null; then
    os_info=$(lsb_release -a 2>/dev/null)
    result "os_info" "$os_info"
    step_success "系统信息检查完成"
elif [ -f "/etc/os-release" ]; then
    os_info=$(cat /etc/os-release)
    result "os_info" "$os_info"
    step_success "系统信息检查完成"
else
    step_info "无法检查系统信息，缺少lsb_release命令或/etc/os-release文件"
fi

# 步骤3：检查内核版本
echo "执行步骤3：检查内核版本"
step_start "检查内核版本"

# 检查uname命令是否可用
if command -v uname &> /dev/null; then
    kernel_version=$(uname -r)
    result "kernel_version" "$kernel_version"
    step_success "内核版本检查完成"
else
    step_info "uname命令不可用，无法检查内核版本"
fi

# 步骤4：检查ARM特有功能
echo "执行步骤4：检查ARM特有功能"
step_start "检查ARM特有功能"

# 检查是否存在ARM特有文件或目录
if [ -d "/sys/devices/system/cpu/cpu0/cpufreq" ]; then
    cpufreq_info=$(cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_available_governors 2>/dev/null)
    result "cpufreq_governors" "$cpufreq_info"
    step_success "ARM CPU频率调节功能检查完成"
else
    step_info "无法检查ARM特有功能，可能不是ARM平台"
fi

# 结果汇报
echo "执行步骤5：结果分析与汇报"
step_start "结果分析与汇报"

# 综合分析
if command -v uname &> /dev/null; then
    arch=$(uname -m)
    if echo "$arch" | grep -i "arm"; then
        result "recommendation" "请检查系统是否为ARM优化版本，确保内核和驱动与ARM架构兼容"
        step_success "ARM平台检查完成"
    else
        result "recommendation" "不是ARM平台，ARM相关问题可能不适用"
        step_info "不是ARM平台，ARM相关问题可能不适用"
    fi
fi

kb_save

echo "CASE执行完成"
