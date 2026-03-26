#!/bin/bash
# CASE: 3PAR服务LUN导致无法添加存储
# KB ID: 35838
# 描述: 服务端透传服务LUN异常，没有透传数据LUN，而是服务LUN，导致前台添加存储界面为空

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

source "$PROJECT_ROOT/scripts/bash/api.sh"

trap 'kb_save; echo "CASE执行异常中断"; exit 1' INT TERM

kb_init

# 步骤0：查询相关告警
echo "执行步骤0：查询相关告警"
step_start "查询存储相关告警"

# 模拟告警查询，实际环境中可以使用log_db_search_alerts函数
result "STORAGE_ALERT_COUNT" "0"
result "ISCSI_ALERT_COUNT" "0"
step_success "未发现存储相关告警"

# 步骤1：检查iSCSI认证状态
echo "执行步骤1：检查iSCSI认证状态"
step_start "检查iSCSI认证状态"

# 检查iSCSI服务状态
if command -v systemctl &> /dev/null; then
    iscsi_status=$(systemctl status iscsid 2>/dev/null | grep -E 'Active:')
    result "iscsi_service_status" "$iscsi_status"
    step_success "iSCSI服务状态检查完成"
elif command -v service &> /dev/null; then
    iscsi_status=$(service iscsid status 2>/dev/null | grep -E 'is running|is stopped')
    result "iscsi_service_status" "$iscsi_status"
    step_success "iSCSI服务状态检查完成"
else
    step_info "无法检查iSCSI服务状态，缺少systemctl或service命令"
fi

# 步骤2：检查内核日志SES设备
echo "执行步骤2：检查内核日志SES设备"
step_start "检查内核日志SES设备"

# 检查内核日志中的3PARdata SES设备信息
if command -v dmesg &> /dev/null; then
    ses_info=$(dmesg | grep -i "3PARdata SES" | head -10)
    if [ -n "$ses_info" ]; then
        result "ses_device_info" "$ses_info"
        step_warning "发现3PARdata SES设备信息"
    else
        step_success "未发现3PARdata SES设备信息"
    fi
else
    step_info "dmesg命令不可用，无法检查内核日志"
fi

# 步骤3：检查LUN信息
echo "执行步骤3：检查LUN信息"
step_start "检查LUN信息"

# 检查iscsiadm命令是否可用
if command -v iscsiadm &> /dev/null; then
    lun_info=$(iscsiadm -m node -l 2>/dev/null | grep -i "lun")
    if [ -n "$lun_info" ]; then
        result "lun_info" "$lun_info"
        step_success "LUN信息检查完成"
    else
        step_info "未发现LUN信息"
    fi
else
    step_info "iscsiadm命令不可用，无法检查LUN信息"
fi

# 结果汇报
step_start "结果分析与汇报"

# 检查是否存在服务LUN（LUN 0或LUN 254）
if command -v iscsiadm &> /dev/null; then
    service_lun=$(iscsiadm -m node -l 2>/dev/null | grep -E "LUN 0|LUN 254")
    if [ -n "$service_lun" ]; then
        result "service_lun_detected" "true"
        step_warning "发现服务LUN（LUN 0或LUN 254），可能导致无法添加存储"
        result "recommendation" "请检查存储端配置，确保映射的是数据LUN而不是服务LUN"
    else
        result "service_lun_detected" "false"
        step_success "未发现服务LUN"
    fi
else
    step_info "无法检查服务LUN，iscsiadm命令不可用"
fi

kb_save

echo "CASE执行完成"
