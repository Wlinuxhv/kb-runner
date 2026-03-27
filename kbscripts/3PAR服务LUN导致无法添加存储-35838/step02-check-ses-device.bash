#!/bin/bash
# 步骤2：检查内核日志SES设备
# KB 35838: 3PAR服务LUN导致无法添加存储

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/bash/api.sh"

kb_init

step_start "检查内核日志3PAR SES设备"

KERNEL_LOG="/sf/log/today/kernel.log"
DMESG_LOG="/sf/log/today/dmesg"

SES_LOGS=""

if [ -f "$KERNEL_LOG" ]; then
    SES_LOGS=$(grep -iE "3PARdata SES|Enclosure.*3PAR|ses.*Attached" "$KERNEL_LOG" 2>/dev/null | tail -20)
fi

if [ -z "$SES_LOGS" ] && [ -f "$DMESG_LOG" ]; then
    SES_LOGS=$(grep -iE "3PARdata SES|Enclosure.*3PAR|ses.*Attached" "$DMESG_LOG" 2>/dev/null | tail -20)
fi

SES_COUNT=$(echo "$SES_LOGS" | grep -c . 2>/dev/null || echo "0")
result "SES_LOG_COUNT" "$SES_COUNT"

if [ "$SES_COUNT" -gt 0 ]; then
    step_output "$SES_LOGS"
    step_warning "发现 3PARdata SES 设备日志"
else
    step_success "未发现 SES 设备日志"
fi

step_start "检查LUN 0/254"

LUN_LOGS=""

if [ -f "$KERNEL_LOG" ]; then
    LUN_LOGS=$(grep -E "3:0:0:(0|254)|LUN.*(0|254)" "$KERNEL_LOG" 2>/dev/null | tail -20)
fi

if [ -n "$LUN_LOGS" ]; then
    step_output "$LUN_LOGS"
    
    if echo "$LUN_LOGS" | grep -q "SES"; then
        step_warning "检测到 LUN 0 或 LUN 254 为 SES 服务设备，非数据 LUN"
        result "SERVICE_LUN_DETECTED" "true"
    fi
fi

step_start "检查SCSI设备列表"

SCSI_DEVICES=$(lsscsi 2>/dev/null | grep -iE "3PAR|enclosure" | head -20)

if [ -n "$SCSI_DEVICES" ]; then
    step_output "$SCSI_DEVICES"
    
    ENCLOSURE_COUNT=$(echo "$SCSI_DEVICES" | grep -ci "enclosure" 2>/dev/null || echo "0")
    result "ENCLOSURE_COUNT" "$ENCLOSURE_COUNT"
    
    if [ "$ENCLOSURE_COUNT" -gt 0 ]; then
        step_warning "检测到 Enclosure 设备，可能是服务 LUN"
    fi
fi

step_start "问题分析"

log_info "============================================"
log_info "注意："
log_info "1. LUN 0 或 LUN 254 可能是 3PAR 的服务 LUN"
log_info "2. SES (SCSI Enclosure Services) 是机箱服务设备"
log_info "3. 服务端应映射数据 LUN 而非服务 LUN"
log_info "============================================"
log_info "解决方案："
log_info "服务端检查配置，确保 IQN 匹配，映射正确的数据 LUN"
log_info "============================================"

step_success "SES 设备检查完成"

kb_save