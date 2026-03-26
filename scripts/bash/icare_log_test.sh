#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/api.sh"
source "$SCRIPT_DIR/icare_log_api.sh"

kb_init

# 设置日志根路径
export ICARE_LOG_ROOT="${KB_OFFLINE_ICARE_LOG_ROOT:-./workspace/icare_log/logall}"

# 测试Q单号
QNO="${KB_OFFLINE_QNO:-Q2026031201098}"

step_start "init_log"
echo "正在初始化ICare日志适配器..."
icare_log_init "$QNO"
if [ $? -eq 0 ]; then
    step_success "日志适配器初始化成功"
else
    step_failure "日志适配器初始化失败"
    kb_save
    exit 1
fi

step_start "check_status"
echo "检查日志状态..."
status=$(icare_log_status)
result "status" "$status"
step_success "状态检查完成"

step_start "list_hosts"
echo "列出主机列表..."
hosts=$(icare_log_list_hosts)
result "hosts" "$hosts"
step_success "主机列表获取完成"

if [ "${KB_OFFLINE_TEST_LIGHT:-0}" = "1" ]; then
    step_start "light_mode_exit"
    step_success "light mode enabled, skip heavy listing/search"
    kb_save
    exit 0
fi

step_start "list_logs"
echo "列出日志文件..."
logs=$(icare_log_list)
result "logs" "$logs"
step_success "日志文件列表获取完成"

step_start "search_errors"
echo "搜索错误关键字..."
errors=$(icare_log_search "error")
result "error_count" "$(echo "$errors" | grep -c '"file"')"
step_success "错误搜索完成"

kb_save