#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/bash/api.sh"
source "${SCRIPT_DIR}/../scripts/bash/log_db.sh"

kb_init

step_start "检查虚拟机相关告警"

ALERTS=$(log_db_search_alerts "虚拟机" 2>/dev/null)

if [ -n "$ALERTS" ]; then
    result "虚拟机告警" "发现虚拟机相关告警"
    echo "$ALERTS"
else
    result "虚拟机告警" "未发现虚拟机相关告警"
fi

echo ""
echo "检查HA相关告警..."
ALERTS2=$(log_db_search_alerts "HA" 2>/dev/null)

if [ -n "$ALERTS2" ]; then
    result "HA告警" "发现HA相关告警"
    echo "$ALERTS2"
else
    result "HA告警" "未发现HA相关告警"
fi

step_success "告警检查完成"

kb_save