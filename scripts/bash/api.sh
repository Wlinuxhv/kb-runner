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
KB_CONFIG_FILE="${KB_CONFIG_FILE:-}"
KB_MAX_SCORE="${KB_MAX_SCORE:-100.0}"

# Offline 模式配置（由框架注入）
KB_RUN_MODE="${KB_RUN_MODE:-online}"
KB_OFFLINE_QNO="${KB_OFFLINE_QNO:-}"
KB_OFFLINE_ICARE_LOG_ROOT="${KB_OFFLINE_ICARE_LOG_ROOT:-}"
KB_OFFLINE_HOST="${KB_OFFLINE_HOST:-}"
KB_OFFLINE_HOSTS_JSON="${KB_OFFLINE_HOSTS_JSON:-[]}"
KB_OFFLINE_LOG_DIR="${KB_OFFLINE_LOG_DIR:-}"
KB_OFFLINE_LOG_DIRS_JSON="${KB_OFFLINE_LOG_DIRS_JSON:-[]}"

# 当前步骤信息
KB_CURRENT_STEP=""
KB_STEP_START_TIME=0
KB_STEP_OUTPUT=""
KB_STEP_WEIGHT=1.0
KB_STEP_EXPECTED_STATUS="success"

# 得分计算相关
KB_TOTAL_SCORE=0
KB_STEPS_CONFIGURED=false

# ========== 内部函数 ==========

# 加载步骤配置
_kb_load_step_config() {
    local step_name="$1"
    
    if [ -z "$KB_CONFIG_FILE" ] || [ ! -f "$KB_CONFIG_FILE" ]; then
        KB_STEP_WEIGHT=1.0
        KB_STEP_EXPECTED_STATUS="success"
        return
    fi
    
    KB_STEP_WEIGHT=1.0
    KB_STEP_EXPECTED_STATUS="success"
    
    # 使用 grep 和 sed 解析 YAML 配置（避免 Python 依赖）
    local in_step=false
    
    while IFS= read -r line; do
        # 跳过注释和空行
        [[ "$line" =~ ^[[:space:]]*# ]] && continue
        [[ -z "$line" ]] && continue
        
        # 检查是否进入步骤配置
        if echo "$line" | grep -q "name:[[:space:]]*[\"']\?$step_name[\"']\?$"; then
            in_step=true
            continue
        fi
        
        # 如果已找到步骤，读取权重和期望状态
        if [ "$in_step" = true ]; then
            # 检查是否离开当前步骤块
            if echo "$line" | grep -q "^[[:space:]]*-[[:space:]]*name:"; then
                break
            fi
            
            # 读取 weight（排除注释）
            if echo "$line" | grep -q "weight:"; then
                KB_STEP_WEIGHT=$(echo "$line" | sed 's/.*weight:[[:space:]]*//' | sed 's/#.*//' | tr -d ' "')
            fi
            
            # 读取 expected_status（排除注释）
            if echo "$line" | grep -q "expected_status:"; then
                KB_STEP_EXPECTED_STATUS=$(echo "$line" | sed 's/.*expected_status:[[:space:]]*//' | sed 's/#.*//' | tr -d ' "')
            fi
        fi
    done < "$KB_CONFIG_FILE"
}

# 计算得分
_kb_calculate_score() {
    if ! command -v jq &> /dev/null || [ ! -f "$KB_RESULT_FILE" ]; then
        echo "0"
        return
    fi
    
    local steps_count
    steps_count=$(jq '.steps | length' "$KB_RESULT_FILE" 2>/dev/null || echo "0")
    
    if [ "$steps_count" -eq 0 ]; then
        echo "0"
        return
    fi
    
    local total_possible=0
    local total_earned=0
    
    # 从配置文件中读取所有步骤的权重
    declare -A step_weights
    declare -A step_expected
    
    if [ -n "$KB_CONFIG_FILE" ] && [ -f "$KB_CONFIG_FILE" ]; then
        # 解析 YAML 配置（简化版，假设配置格式规范）
        local current_step=""
        while IFS= read -r line; do
            if echo "$line" | grep -q "^\s*- name:"; then
                current_step=$(echo "$line" | sed 's/.*name:[[:space:]]*//' | tr -d '"')
            elif [ -n "$current_step" ]; then
                if echo "$line" | grep -q "weight:"; then
                    local w=$(echo "$line" | sed 's/.*weight:[[:space:]]*//' | tr -d ' ')
                    step_weights["$current_step"]="$w"
                fi
                if echo "$line" | grep -q "expected_status:"; then
                    local e=$(echo "$line" | sed 's/.*expected_status:[[:space:]]*//' | tr -d ' "')
                    step_expected["$current_step"]="$e"
                fi
            fi
        done < "$KB_CONFIG_FILE"
    fi
    
    # 遍历所有步骤计算得分
    for ((i=0; i<steps_count; i++)); do
        local step_name
        step_name=$(jq -r ".steps[$i].name" "$KB_RESULT_FILE" 2>/dev/null)
        local step_status
        step_status=$(jq -r ".steps[$i].status" "$KB_RESULT_FILE" 2>/dev/null)
        
        # 获取配置的权重和期望状态
        local weight="${step_weights[$step_name]:-1.0}"
        local expected="${step_expected[$step_name]:-success}"
        
        # 使用 bc 进行浮点运算
        total_possible=$(echo "$total_possible + $weight" | bc -l 2>/dev/null || echo "$total_possible")
        
        if [ "$step_status" = "$expected" ]; then
            total_earned=$(echo "$total_earned + $weight" | bc -l 2>/dev/null || echo "$total_earned")
        fi
    done
    
    # 计算最终得分
    if [ "$(echo "$total_possible > 0" | bc -l 2>/dev/null || echo "0")" -eq 1 ]; then
        local raw_score
        raw_score=$(echo "scale=4; $total_earned / $total_possible" | bc -l 2>/dev/null || echo "0")
        local final_score
        final_score=$(echo "scale=2; $raw_score * $KB_MAX_SCORE" | bc -l 2>/dev/null || echo "0")
        echo "$final_score"
    else
        echo "0"
    fi
}

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
    
    local step_score=0
    local step_max_score="$KB_STEP_WEIGHT"
    
    if [ "$status" = "$KB_STEP_EXPECTED_STATUS" ]; then
        step_score="$KB_STEP_WEIGHT"
    fi
    
    local step_json
    step_json=$(cat <<EOF
{
  "name": "$KB_CURRENT_STEP",
  "status": "$status",
  "message": "$message",
  "output": "$KB_STEP_OUTPUT",
  "duration_ms": $duration_ms,
  "weight": $KB_STEP_WEIGHT,
  "expected_status": "$KB_STEP_EXPECTED_STATUS",
  "score": $step_score,
  "max_score": $step_max_score
}
EOF
)
    
    if command -v jq &> /dev/null; then
        local tmp_file
        tmp_file=$(mktemp)
        # 使用 --argjson 传递 JSON 对象，避免字符串转义问题
        jq --argjson step "$step_json" '.steps += [$step]' "$KB_RESULT_FILE" > "$tmp_file" 2>/dev/null && mv "$tmp_file" "$KB_RESULT_FILE"
    fi
    
    _kb_log "$KB_LOG_INFO" "步骤结束：$KB_CURRENT_STEP ($status, ${duration_ms}ms, score: $step_score/$step_max_score)"
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
    
    if command -v jq &> /dev/null && [ -f "$KB_RESULT_FILE" ]; then
        local final_score=0
        final_score=$(_kb_calculate_score)
        
        # 统计失败步骤数量
        local failure_count
        failure_count=$(jq '[.steps[].status | select(. == "failure")] | length' "$KB_RESULT_FILE" 2>/dev/null || echo "0")
        
        local status="success"
        if [ "$failure_count" -gt 0 ]; then
            status="failure"
        fi
        
        # 更新结果文件
        local tmp_file
        tmp_file=$(mktemp)
        jq --argjson score "$final_score" \
           --arg status "$status" \
           --argjson max_score "$KB_MAX_SCORE" \
           '.score = $score | .status = $status | .max_score = $max_score | .final_score = $score' \
           "$KB_RESULT_FILE" > "$tmp_file" 2>/dev/null && mv "$tmp_file" "$KB_RESULT_FILE"
        
        echo "KB 执行完成：status=$status, score=$final_score/$KB_MAX_SCORE"
    fi
    
    _kb_log "$KB_LOG_INFO" "脚本执行完成"
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
    
    _kb_load_step_config "$step_name"
    
    _kb_log "$KB_LOG_INFO" "步骤开始：$step_name (weight: $KB_STEP_WEIGHT, expected: $KB_STEP_EXPECTED_STATUS)"
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

# info 语义上更像“非失败的提示”，这里按 success 计入得分。
step_info() {
    local message="${1:-执行信息}"
    _kb_end_step "$KB_STATUS_SUCCESS" "$message"
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

# ========== 离线环境信息 ==========
# 由 kb-runner 在 offline 模式注入：
#   KB_OFFLINE_LOG_DIR, KB_OFFLINE_HOST, KB_OFFLINE_HOSTS_JSON 等
kb_offline_log_dir() {
    echo "${KB_OFFLINE_LOG_DIR:-}"
}

kb_offline_hosts_json() {
    echo "${KB_OFFLINE_HOSTS_JSON:-[]}"
}

kb_offline_log_dirs_json() {
    echo "${KB_OFFLINE_LOG_DIRS_JSON:-[]}"
}

kb_offline_host() {
    echo "${KB_OFFLINE_HOST:-}"
}

# ========== 受控命令执行 ==========
#
# 目的：在 offline（日志包分析）模式下禁止执行真实环境采集命令。
# 使用方式：
#   kb_exec "ip a"
#   kb_exec "systemctl status xxx"
#
# 环境变量：
#   KB_RUN_MODE=online|offline (默认 online)
kb_exec() {
    local cmd="$1"
    local mode="${KB_RUN_MODE:-online}"

    if [ -z "$cmd" ]; then
        log_warn "kb_exec: empty command"
        return 2
    fi

    if [ "$mode" = "offline" ]; then
        # 记录一个 warning，但不让脚本因为禁止执行而整体失败
        if [ -n "$KB_CURRENT_STEP" ]; then
            step_output "offline mode: blocked command: $cmd"
        fi
        log_warn "offline mode blocked command: $cmd"
        return 3
    fi

    eval "$cmd"
}

# ========== 帮助函数 ==========

# 显示 KB 脚本帮助信息
kb_help() {
    cat << EOF
KB 脚本执行框架 - Bash API 帮助

使用方法:
    source \$PROJECT_ROOT/scripts/bash/api.sh
    
    kb_init                              # 初始化
    step_start "步骤名称"                 # 开始步骤
    result "key" "value"                 # 记录结果
    step_success "成功消息"               # 步骤成功
    kb_save                              # 保存结果

环境变量（由框架自动注入）:
    基本配置:
        KB_RESULT_FILE          - 结果文件路径
        KB_LOG_FILE             - 日志文件路径
        KB_SCRIPT_NAME          - 脚本名称
        KB_CONFIG_FILE          - 配置文件路径
        KB_MAX_SCORE            - 满分分数（默认 100.0）
    
    Offline 模式:
        KB_RUN_MODE             - 运行模式 (online/offline)
        KB_OFFLINE_QNO          - Q 单号
        KB_OFFLINE_ICARE_LOG_ROOT - ICare 日志根目录
        KB_OFFLINE_HOST         - 当前主机
        KB_OFFLINE_LOG_DIR      - 日志目录
    
    步骤配置:
        KB_STEP_WEIGHT          - 当前步骤权重
        KB_STEP_EXPECTED_STATUS - 当前步骤期望状态

API 函数:
    初始化:
        kb_init()               - 初始化结果文件
        kb_save()               - 保存最终结果
    
    日志:
        log_debug "消息"         - DEBUG 级别日志
        log_info "消息"          - INFO 级别日志
        log_warn "消息"          - WARN 级别日志
        log_error "消息"         - ERROR 级别日志
    
    步骤:
        step_start "名称"        - 开始步骤
        step_success "消息"      - 步骤成功
        step_failure "消息"      - 步骤失败
        step_warning "消息"      - 步骤警告
        step_skip "消息"         - 跳过步骤
        step_info "消息"         - 步骤信息（按成功计）
        step_output "内容"       - 设置步骤输出
    
    结果:
        result "key" "value"     - 记录键值对结果
    
    Offline:
        kb_offline_log_dir()     - 获取日志目录
        kb_offline_host()        - 获取当前主机
        kb_offline_hosts_json()  - 获取主机列表 JSON
        kb_offline_log_dirs_json() - 获取日志目录列表 JSON
    
    受控执行:
        kb_exec "命令"           - 受控执行命令（offline 模式禁止）

示例:
    #!/bin/bash
    source "\$PROJECT_ROOT/scripts/bash/api.sh"
    
    kb_init
    
    step_start "检查环境"
    if [ -f "/etc/os-release" ]; then
        result "os" "linux"
        step_success "环境检查通过"
    else
        step_failure "环境检查失败"
    fi
    
    kb_save

EOF
}

# 显示版本信息
kb_version() {
    echo "KB Runner Bash API v1.0.0"
}
