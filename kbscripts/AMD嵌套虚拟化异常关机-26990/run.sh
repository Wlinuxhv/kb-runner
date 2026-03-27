#!/bin/bash
# CASE: AMD嵌套虚拟化异常关机
# KB ID: 26990
# 描述: 在AMD平台和海光硬件平台上使用嵌套虚拟化可能会导致虚拟机内部关机

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# 设置ICARE_LOG_ROOT环境变量指向工作目录中的ICare日志目录
export ICARE_LOG_ROOT="${KB_OFFLINE_ICARE_LOG_ROOT:-$PROJECT_ROOT/workspace/icare_log/logall}"

# 离线时使用框架注入的 Q 单号；在线/未注入时回退到脚本内置示例
QNO="${KB_OFFLINE_QNO:-Q2026031201098}"
OFF_HOST="${KB_OFFLINE_HOST:-}"

source "$PROJECT_ROOT/scripts/bash/api.sh"
source "$PROJECT_ROOT/scripts/bash/icare_log_api.sh"

trap 'kb_save; echo "CASE执行异常中断"; exit 1' INT TERM

kb_init

# 步骤0：检查相关告警
echo "执行步骤0：检查相关告警"
step_start "检查相关告警"

# 使用ICare日志适配器查询相关告警
icare_log_init "$QNO"

if [ "${KB_RUN_MODE:-online}" = "offline" ] && [ -n "$OFF_HOST" ]; then
    icare_log_set_host "$OFF_HOST" >/dev/null 2>&1 || true
fi

# 搜索虚拟机关机相关告警
vm_shutdown_alerts=$(icare_log_count_keyword "虚拟机内部关机")
result "VM_SHUTDOWN_ALERT_COUNT" "$vm_shutdown_alerts"

# 搜索HA恢复相关告警
ha_recovery_alerts=$(icare_log_count_keyword "HA尝试恢复")
result "HA_RECOVERY_ALERT_COUNT" "$ha_recovery_alerts"

if [ "$vm_shutdown_alerts" -gt 0 ] || [ "$ha_recovery_alerts" -gt 0 ]; then
    step_warning "发现虚拟机关机或HA恢复相关告警"
else
    step_success "未发现相关告警"
fi

# offline 模式下：仅基于 icare 日志适配器做分析，跳过 lscpu/sysfs/实时文件系统采集
if [ "${KB_RUN_MODE:-online}" = "offline" ]; then
    step_start "offline_rely_on_logs"
    step_warning "offline mode: skip realtime commands/files; rely on collected icare logs"
    kb_save
    exit 0
fi

# 步骤1：检查平台类型
echo "执行步骤1：检查平台类型"
step_start "检查平台类型"

# 检查是否为AMD平台
if command -v lscpu &> /dev/null; then
    cpu_info=$(lscpu | grep -i "vendor_id")
    if echo "$cpu_info" | grep -i "amd"; then
        result "platform_type" "AMD"
        step_success "确认是AMD平台"
    else
        result "platform_type" "Non-AMD"
        step_info "不是AMD平台，嵌套虚拟化问题可能不适用"
    fi
else
    step_info "lscpu命令不可用，无法检查平台类型"
fi

# 步骤2：检查嵌套虚拟化配置
echo "执行步骤2：检查嵌套虚拟化配置"
step_start "检查嵌套虚拟化配置"

# 检查AMD嵌套虚拟化设置
if [ -f "/sys/module/kvm_amd/parameters/nested" ]; then
    nested_status=$(cat /sys/module/kvm_amd/parameters/nested)
    result "nested_virtualization_status" "$nested_status"
    
    if [ "$nested_status" = "1" ] || [ "$nested_status" = "Y" ]; then
        step_warning "AMD嵌套虚拟化已开启，可能导致虚拟机异常关机"
    else
        step_success "AMD嵌套虚拟化已关闭"
    fi
else
    step_info "无法检查AMD嵌套虚拟化状态，可能不是AMD平台"
fi

# 步骤3：检查虚拟机嵌套配置
echo "执行步骤3：检查虚拟机嵌套配置"
step_start "检查虚拟机嵌套配置"

# 检查是否存在开启嵌套虚拟化的虚拟机
if command -v grep &> /dev/null; then
    if [ -d "/cfs" ]; then
        nested_vms=$(grep "nested_virtualization: 1" -rn /cfs/ 2>/dev/null | wc -l)
        result "nested_vms_count" "$nested_vms"
        
        if [ "$nested_vms" -gt 0 ]; then
            step_warning "发现 $nested_vms 个开启嵌套虚拟化的虚拟机"
        else
            step_success "未发现开启嵌套虚拟化的虚拟机"
        fi
    else
        step_info "/cfs目录不存在，无法检查虚拟机嵌套配置"
    fi
else
    step_info "grep命令不可用，无法检查虚拟机嵌套配置"
fi

# 步骤4：检查qemu日志
echo "执行步骤4：检查qemu日志"
step_start "检查qemu日志"

# 检查是否存在qemu日志目录
if [ -d "/sf/log" ]; then
    # 查找最近的日志目录
    latest_log_dir=$(ls -1d /sf/log/* 2>/dev/null | sort -r | head -1)
    if [ -n "$latest_log_dir" ]; then
        # 检查是否有包含KVM_EXIT_SHUTDOWN的日志
        kvm_shutdown=$(grep -r "KVM_EXIT_SHUTDOWN" "$latest_log_dir" 2>/dev/null | head -5)
        if [ -n "$kvm_shutdown" ]; then
            result "kvm_shutdown_detected" "true"
            step_warning "在qemu日志中发现KVM_EXIT_SHUTDOWN，可能与嵌套虚拟化问题相关"
        else
            result "kvm_shutdown_detected" "false"
            step_success "未在qemu日志中发现KVM_EXIT_SHUTDOWN"
        fi
    else
        step_info "未找到日志目录"
    fi
else
    step_info "/sf/log目录不存在，无法检查qemu日志"
fi

# 结果汇报
echo "执行步骤5：结果分析与汇报"
step_start "结果分析与汇报"

# 综合分析
if [ -f "/sys/module/kvm_amd/parameters/nested" ]; then
    nested_status=$(cat /sys/module/kvm_amd/parameters/nested)
    if [ "$nested_status" = "1" ] || [ "$nested_status" = "Y" ]; then
        result "recommendation" "建议关闭AMD嵌套虚拟化功能，修改配置文件/sf/modules/loadmod.conf，将kvm_amd nested=1改为kvm_amd nested=0，然后重启主机"
        step_warning "检测到AMD嵌套虚拟化已开启，可能导致虚拟机异常关机"
    else
        result "recommendation" "AMD嵌套虚拟化已关闭，无需操作"
        step_success "AMD嵌套虚拟化已关闭，不会导致虚拟机异常关机"
    fi
fi

kb_save

echo "CASE执行完成"
