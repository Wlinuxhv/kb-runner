#!/bin/bash
# 步骤1：检查iSCSI认证状态
# KB 35838: 3PAR服务LUN导致无法添加存储

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/bash/api.sh"

PORTAL=$(get_param "portal" "")

kb_init

step_start "检查iSCSI会话状态"

if ! command -v iscsiadm &> /dev/null; then
    step_skip "iscsiadm 命令不存在"
    kb_save
    exit 0
fi

SESSION_OUTPUT=$(iscsiadm -m session 2>&1)
SESSION_COUNT=$(echo "$SESSION_OUTPUT" | grep -c "tcp" 2>/dev/null || echo "0")

result "SESSION_COUNT" "$SESSION_COUNT"

if [ "$SESSION_COUNT" -gt 0 ]; then
    step_output "$(echo "$SESSION_OUTPUT" | head -10)"
    step_success "发现 $SESSION_COUNT 个 iSCSI 会话"
else
    step_warning "无 iSCSI 会话"
fi

step_start "检查节点配置"

NODE_OUTPUT=$(iscsiadm -m node 2>&1)
NODE_COUNT=$(echo "$NODE_OUTPUT" | grep -c ":" 2>/dev/null || echo "0")

result "NODE_COUNT" "$NODE_COUNT"

if [ "$NODE_COUNT" -gt 0 ]; then
    step_output "$(echo "$NODE_OUTPUT" | head -10)"
fi

step_start "测试发现目标"

if [ -n "$PORTAL" ]; then
    DISCOVER_OUTPUT=$(iscsiadm -m discovery -t st -p "$PORTAL" 2>&1)
    DISCOVER_EXIT=$?
    
    step_output "$DISCOVER_OUTPUT"
    result "DISCOVER_EXIT" "$DISCOVER_EXIT"
    
    TARGET_COUNT=$(echo "$DISCOVER_OUTPUT" | grep -c "iqn" 2>/dev/null || echo "0")
    result "TARGET_COUNT" "$TARGET_COUNT"
    
    if [ "$TARGET_COUNT" -gt 0 ]; then
        step_success "发现 $TARGET_COUNT 个 target"
    else
        step_warning "未发现 target，可能存储端未映射数据 LUN"
    fi
else
    step_skip "未指定 portal 参数"
fi

kb_save