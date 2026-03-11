#!/bin/bash
# KB脚本执行框架 - Bash API
# 提供日志输出和结果记录功能

# 日志级别常量
KB_LOG_DEBUG="DEBUG"
KB_LOG_INFO="INFO"
KB_LOG_WARN="WARN"
KB_LOG_ERROR="ERROR"

# 步骤状态常量
KB_STATUS_SUCCESS="success"
KB_STATUS_FAILURE="failure"
KB_STATUS_WARNING="warning"
KB_STATUS_SKIPPED="skipped"

# 配置（由框架注入）
KB_RESULT_FILE="${KB_RESULT_FILE:-/tmp/kb_result.json}"
KB_LOG_FILE="${KB_LOG_FILE:-/tmp/kb.log}"
KB_SCRIPT_NAME="${KB_SCRIPT_NAME:-unknown}"
KB_LOG_LEVEL="${KB_LOG_LEVEL:-INFO}"

# 当前步骤信息
KB_CURRENT_STEP=""
KB_STEP_START_TIME=0
KB_STEP_OUTPUT=""

# ========== 内部函数 ==========

# 日志级别检查
_kb_should_log() {
    local level="$1"
    case "$KB_LOG_LEVEL" in
        DEBUG) return 0 ;;
        INFO)  [ "$level" != "DEBUG" ] && return 0 ;;
        WARN)  [ "$level" = "WARN" ] || [ "$level" = "ERROR" ] && return 0 ;;
        ERROR) [ "$level" = "ERROR" ] && return 0 ;;
    esac
    return 1
}

# 内部日志函数
_kb_log() {
    local level="$1"
    local message="$2"
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    local prefix="[$timestamp] [$level]"
    if [ -n "$KB_CURRENT_STEP" ]; then
        prefix="$prefix [$KB_SCRIPT_NAME:$KB_CURRENT_STEP]"
    else
        prefix="$prefix [$KB_SCRIPT_NAME]"
    fi
    
    echo "$prefix $message" >> "$KB_LOG_FILE"
    
    if [ "$level" = "$KB_LOG_ERROR" ]; then
        echo "$prefix $message" >&2
    fi
}

# 结束步骤（内部函数）
_kb_end_step() {
    local status="$1"
    local message="${2:-}"
    
    if [ -z "$KB_CURRENT_STEP" ]; then
        return 0
    fi
    
    local end_time
    end_time=$(date +%s%N)
    local duration_ms=$(( (end_time - KB_STEP_START_TIME) / 1000000 ))
    
    local step_json
    step_json=$(cat <<EOF
{
  "name": "$KB_CURRENT_STEP",
  "status": "$status",
  "message": "$message",
  "output": "$KB_STEP_OUTPUT",
  "duration_ms": $duration_ms
}
EOF
)
    
    if command -v jq &> /dev/null; then
        local tmp_file
        tmp_file=$(mktemp)
        jq ".steps += [$step_json]" "$KB_RESULT_FILE" > "$tmp_file" 2>/dev/null && mv "$tmp_file" "$KB_RESULT_FILE"
    fi
    
    _kb_log "$KB_LOG_INFO" "步骤结束: $KB_CURRENT_STEP ($status, ${duration_ms}ms)"
    KB_CURRENT_STEP=""
}

# ========== 初始化函数 ==========

# 初始化结果文件
kb_init() {
    cat > "$KB_RESULT_FILE" <<EOF
{
  "script_name": "$KB_SCRIPT_NAME",
  "status": "running",
  "steps": [],
  "results": {}
}
EOF
    _kb_log "$KB_LOG_INFO" "脚本初始化完成"
}

# 保存最终结果
kb_save() {
    if [ -n "$KB_CURRENT_STEP" ]; then
        _kb_end_step "$KB_STATUS_WARNING" "步骤未正确关闭"
    fi
    
    local status="success"
    local score=1.0
    
    if command -v jq &> /dev/null; then
        local failure_count
        failure_count=$(jq '[.steps[].status | select(. == "failure")] | length' "$KB_RESULT_FILE" 2>/dev/null || echo "0")
        local warning_count
        warning_count=$(jq '[.steps[].status | select(. == "warning")] | length' "$KB_RESULT_FILE" 2>/dev/null || echo "0")
        
        if [ "$failure_count" -gt 0 ]; then
            status="failure"
            score=0.0
        elif [ "$warning_count" -gt 0 ]; then
            status="warning"
            score=$(echo "scale=2; 1 - $warning_count * 0.1" | bc 2>/dev/null || echo "0.9")
        fi
        
        local tmp_file
        tmp_file=$(mktemp)
        jq ".status = \"$status\" | .score = $score" "$KB_RESULT_FILE" > "$tmp_file" 2>/dev/null && mv "$tmp_file" "$KB_RESULT_FILE"
    fi
    
    _kb_log "$KB_LOG_INFO" "脚本执行完成: status=$status, score=$score"
}

# ========== 日志API ==========

# DEBUG级别日志
log_debug() {
    _kb_should_log "$KB_LOG_DEBUG" && _kb_log "$KB_LOG_DEBUG" "$1"
}

# INFO级别日志
log_info() {
    _kb_should_log "$KB_LOG_INFO" && _kb_log "$KB_LOG_INFO" "$1"
}

# WARN级别日志
log_warn() {
    _kb_should_log "$KB_LOG_WARN" && _kb_log "$KB_LOG_WARN" "$1"
}

# ERROR级别日志
log_error() {
    _kb_should_log "$KB_LOG_ERROR" && _kb_log "$KB_LOG_ERROR" "$1"
}

# ========== 步骤API ==========

# 步骤开始
step_start() {
    local step_name="$1"
    
    if [ -n "$KB_CURRENT_STEP" ]; then
        _kb_end_step "$KB_STATUS_WARNING" "步骤未正确关闭"
    fi
    
    KB_CURRENT_STEP="$step_name"
    KB_STEP_START_TIME=$(date +%s%N)
    KB_STEP_OUTPUT=""
    
    _kb_log "$KB_LOG_INFO" "步骤开始: $step_name"
}

# 步骤成功
step_success() {
    local message="${1:-执行成功}"
    _kb_end_step "$KB_STATUS_SUCCESS" "$message"
}

# 步骤失败
step_failure() {
    local message="${1:-执行失败}"
    _kb_end_step "$KB_STATUS_FAILURE" "$message"
}

# 步骤警告
step_warning() {
    local message="${1:-执行警告}"
    _kb_end_step "$KB_STATUS_WARNING" "$message"
}

# 步骤跳过
step_skip() {
    local message="${1:-跳过执行}"
    _kb_end_step "$KB_STATUS_SKIPPED" "$message"
}

# 记录步骤输出
step_output() {
    KB_STEP_OUTPUT="$1"
}

# ========== 结果API ==========

# 记录键值对结果
result() {
    local key="$1"
    local value="$2"
    
    if command -v jq &> /dev/null; then
        local tmp_file
        tmp_file=$(mktemp)
        jq ".results.$key = \"$value\"" "$KB_RESULT_FILE" > "$tmp_file" 2>/dev/null && mv "$tmp_file" "$KB_RESULT_FILE"
    fi
}

# ========== 参数获取 ==========

# 获取参数值
get_param() {
    local key="$1"
    local default="${2:-}"
    local var_name="KB_PARAM_$(echo "$key" | tr '[:lower:]' '[:upper:]')"
    eval "echo \${$var_name:-$default}"
}
