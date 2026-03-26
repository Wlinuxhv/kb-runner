#!/bin/bash
# 网络连通性检查脚本

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/api.sh"

kb_init

if [ "${KB_RUN_MODE:-online}" = "offline" ]; then
    step_start "offline_mode_skip"
    step_warning "offline mode: skip real network commands (ping/nslookup/ip/etc), analyze logs instead"
    kb_save
    exit 0
fi

step_start "check_localhost"
if ping -c 1 127.0.0.1 > /dev/null 2>&1; then
    result "localhost_ping" "success"
    step_success "本地回环地址可达"
else
    result "localhost_ping" "failure"
    step_failure "本地回环地址不可达"
fi

step_start "check_dns"
if command -v nslookup &> /dev/null || command -v host &> /dev/null; then
    if nslookup localhost > /dev/null 2>&1 || host localhost > /dev/null 2>&1; then
        result "dns_resolution" "success"
        step_success "DNS解析正常"
    else
        result "dns_resolution" "failure"
        step_warning "DNS解析可能存在问题"
    fi
else
    step_skip "DNS工具未安装"
fi

step_start "check_network_interfaces"
if command -v ip &> /dev/null; then
    interfaces=$(ip link show | grep -E '^[0-9]+' | wc -l)
    result "interface_count" "$interfaces"
    step_success "发现 $interfaces 个网络接口"
elif command -v ifconfig &> /dev/null; then
    interfaces=$(ifconfig -a | grep -E '^[a-z]' | wc -l)
    result "interface_count" "$interfaces"
    step_success "发现 $interfaces 个网络接口"
else
    step_skip "无法获取网络接口信息"
fi

kb_save