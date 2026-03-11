#!/bin/bash
# 安全检查脚本示例

source ./scripts/bash/api.sh

kb_init

step_start "check_file_permissions"
if [ -r "/etc/passwd" ]; then
    result "passwd_readable" "true"
    step_success "文件权限检查通过"
else
    step_failure "文件权限检查失败: /etc/passwd 不可读"
fi

step_start "check_disk_space"
df_output=$(df -h / 2>/dev/null | tail -1)
if [ -n "$df_output" ]; then
    usage=$(echo "$df_output" | awk '{print $5}' | tr -d '%')
    result "disk_usage" "${usage}%"
    if [ "$usage" -lt 80 ]; then
        step_success "磁盘使用率正常: ${usage}%"
    elif [ "$usage" -lt 90 ]; then
        step_warning "磁盘使用率较高: ${usage}%"
    else
        step_failure "磁盘使用率过高: ${usage}%"
    fi
else
    step_skip "无法获取磁盘信息"
fi

step_start "check_memory"
if command -v free &> /dev/null; then
    mem_info=$(free -m | grep "Mem:")
    total=$(echo "$mem_info" | awk '{print $2}')
    used=$(echo "$mem_info" | awk '{print $3}')
    if [ "$total" -gt 0 ]; then
        usage=$((used * 100 / total))
        result "memory_total_mb" "$total"
        result "memory_used_mb" "$used"
        result "memory_usage" "${usage}%"
        if [ "$usage" -lt 80 ]; then
            step_success "内存使用率正常: ${usage}%"
        else
            step_warning "内存使用率较高: ${usage}%"
        fi
    else
        step_skip "无法计算内存使用率"
    fi
else
    step_skip "free命令不可用"
fi

kb_save
