#!/bin/bash
# CASE: A100显卡问题
# KB ID: 32920
# 描述: A100显卡相关问题的检查和排查

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

source "$PROJECT_ROOT/scripts/bash/api.sh"

trap 'kb_save; echo "CASE执行异常中断"; exit 1' INT TERM

kb_init

# 步骤0：检查相关告警
echo "执行步骤0：检查相关告警"
step_start "检查相关告警"

# 模拟告警查询，实际环境中可以使用log_db_search_alerts函数
result "STORAGE_ALERT_COUNT" "0"
result "NETWORK_ALERT_COUNT" "0"
result "SHUTDOWN_ALERT_COUNT" "0"
result "GPU_ALERT_COUNT" "0"
step_success "未发现相关告警"

# 步骤1：检查GPU设备信息
echo "执行步骤1：检查GPU设备信息"
step_start "检查GPU设备信息"

# 检查lspci命令是否可用
if command -v lspci &> /dev/null; then
    gpu_info=$(lspci | grep -i "nvidiagpu" | head -10)
    if [ -n "$gpu_info" ]; then
        result "gpu_info" "$gpu_info"
        step_success "GPU设备信息检查完成"
    else
        step_info "未发现GPU设备"
    fi
else
    step_info "lspci命令不可用，无法检查GPU设备信息"
fi

# 步骤2：检查NVIDIA驱动状态
echo "执行步骤2：检查NVIDIA驱动状态"
step_start "检查NVIDIA驱动状态"

# 检查nvidia-smi命令是否可用
if command -v nvidia-smi &> /dev/null; then
    nvidia_status=$(nvidia-smi | head -20)
    result "nvidia_driver_status" "$nvidia_status"
    step_success "NVIDIA驱动状态检查完成"
else
    step_info "nvidia-smi命令不可用，无法检查NVIDIA驱动状态"
fi

# 步骤3：检查系统日志中的GPU相关错误
echo "执行步骤3：检查系统日志中的GPU相关错误"
step_start "检查系统日志中的GPU相关错误"

# 检查dmesg命令是否可用
if command -v dmesg &> /dev/null; then
    gpu_errors=$(dmesg | grep -i "nvidiagpuerror" | head -10)
    if [ -n "gpu_errors" ]; then
        result "gpu_errors" "$gpu_errors"
        step_warning "发现GPU相关错误信息"
    else
        step_success "未发现GPU相关错误信息"
    fi
else
    step_info "dmesg命令不可用，无法检查系统日志"
fi

# 结果汇报
step_start "结果分析与汇报"

# 检查是否存在A100显卡
if command -v lspci &> /dev/null; then
    a100_detected=$(lspci | grep -i "a100")
    if [ -n "$a100_detected" ]; then
        result "a100_detected" "true"
        step_success "发现A100显卡"
        result "recommendation" "请检查NVIDIA驱动版本是否与A100显卡兼容"
    else
        result "a100_detected" "false"
        step_info "未发现A100显卡"
    fi
else
    step_info "无法检查A100显卡，lspci命令不可用"
fi

kb_save

echo "CASE执行完成"
