#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/bash/api.sh"
source "${SCRIPT_DIR}/../scripts/bash/log_db.sh"
kb_init
step_start "检查相关告警"
ALERTS=$(log_db_search_alerts "存储\|网卡\|宕机\|显卡" 2>/dev/null)
[ -n "$ALERTS" ] && result "告警" "发现" && echo "$ALERTS" || result "告警" "未发现"
step_success "完成"
kb_save
