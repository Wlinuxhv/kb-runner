#!/bin/bash
# 步骤0：查询相关告警
# KB 35838: 3PAR服务LUN导致无法添加存储

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/bash/api.sh"
source "${SCRIPT_DIR}/../scripts/bash/log_db.sh"

kb_init

step_start "查询存储相关告警"

STORAGE_RESULT=$(log_db_search_alerts "存储" 20)

if echo "$STORAGE_RESULT" | grep -q '"success": true'; then
    STORAGE_COUNT=$(echo "$STORAGE_RESULT" | grep -o '"count": [0-9]*' | grep -o '[0-9]*')
    result "STORAGE_ALERT_COUNT" "$STORAGE_COUNT"
    
    if [ "$STORAGE_COUNT" -gt 0 ]; then
        step_output "$(echo "$STORAGE_RESULT" | head -c 500)"
        step_warning "发现 $STORAGE_COUNT 条存储相关告警"
    else
        step_success "未发现存储相关告警"
    fi
fi

step_start "查询iSCSI相关告警"

ISCSI_RESULT=$(log_db_search_alerts "iscsi" 20)

if echo "$ISCSI_RESULT" | grep -q '"success": true'; then
    ISCSI_COUNT=$(echo "$ISCSI_RESULT" | grep -o '"count": [0-9]*' | grep -o '[0-9]*')
    result "ISCSI_ALERT_COUNT" "$ISCSI_COUNT"
    
    if [ "$ISCSI_COUNT" -gt 0 ]; then
        step_output "$(echo "$ISCSI_RESULT" | head -c 500)"
        step_warning "发现 $ISCSI_COUNT 条 iSCSI 相关告警"
    else
        step_success "未发现 iSCSI 相关告警"
    fi
fi

kb_save