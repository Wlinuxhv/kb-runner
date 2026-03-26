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
    
    if command -v python3 &> /dev/null; then
        local config_data
        config_data=$(python3 << EOF
import yaml
import sys
import json

try:
    with open('$KB_CONFIG_FILE', 'r') as f:
        config = yaml.safe_load(f)
    
    steps = config.get('scoring', {}).get('steps', [])
    for step in steps:
        if step.get('name') == '$step_name':
            result = {
                'weight': step.get('weight', 1.0),
                'expected_status': step.get('expected_status', 'success')
            }
            print(json.dumps(result))
            sys.exit(0)
    
    print(json.dumps({'weight': 1.0, 'expected_status': 'success'}))
except Exception as e:
    print(json.dumps({'weight': 1.0, 'expected_status': 'success', 'error': str(e)}))
EOF
)
        
        if [ -n "$config_data" ]; then
            local weight
            weight=$(echo "$config_data" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('weight', 1.0))" 2>/dev/null)
            local expected
            expected=$(echo "$config_data" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('expected_status', 'success'))" 2>/dev/null)
            
            if [ -n "$weight" ]; then
                KB_STEP_WEIGHT="$weight"
            fi
            if [ -n "$expected" ]; then
                KB_STEP_EXPECTED_STATUS="$expected"
            fi
        fi
    fi
}

# 计算得分
_kb_calculate_score() {
    if ! command -v python3 &> /dev/null || [ ! -f "$KB_RESULT_FILE" ]; then
        echo "0"
        return
    fi
    
    python3 << 'PYTHON_EOF'
import json
import os
import yaml

result_file = os.environ.get('KB_RESULT_FILE', '')
config_file = os.environ.get('KB_CONFIG_FILE', '')
max_score = float(os.environ.get('KB_MAX_SCORE', '100.0'))

try:
    with open(result_file, 'r') as f:
        result = json.load(f)
except:
    print("0")
    exit(0)

steps = result.get('steps', [])
if not steps:
    print("0")
    exit(0)

config_steps = {}
if config_file and os.path.exists(config_file):
    try:
        with open(config_file, 'r') as f:
            config = yaml.safe_load(f)
        for step in config.get('scoring', {}).get('steps', []):
            config_steps[step.get('name')] = {
                'weight': step.get('weight', 1.0),
                'expected_status': step.get('expected_status', 'success')
            }
    except:
        pass

total_possible = 0
total_earned = 0

for step in steps:
    step_name = step.get('name', '')
    step_status = step.get('status', '')
    
    cfg = config_steps.get(step_name, {'weight': 1.0, 'expected_status': 'success'})
    weight = cfg.get('weight', 1.0)
    expected_status = cfg.get('expected_status', 'success')
    
    total_possible += weight
    
    if step_status == expected_status:
        total_earned += weight

if total_possible > 0:
    raw_score = total_earned / total_possible
    final_score = round(raw_score * max_score, 2)
    print(final_score)
else:
    print("0")
PYTHON_EOF
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
        jq ".steps += [$step_json]" "$KB_RESULT_FILE" > "$tmp_file" 2>/dev/null && mv "$tmp_file" "$KB_RESULT_FILE"
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
    
    if command -v python3 &> /dev/null && [ -f "$KB_RESULT_FILE" ]; then
        python3 << 'PYTHON_EOF'
import json
import os

result_file = os.environ.get('KB_RESULT_FILE', '')
max_score = float(os.environ.get('KB_MAX_SCORE', '100.0'))

try:
    with open(result_file, 'r') as f:
        result = json.load(f)
except Exception as e:
    print(f"Error reading result file: {e}")
    exit(1)

steps = result.get('steps', [])
failure_count = sum(1 for s in steps if s.get('status') == 'failure')

total_possible = sum(s.get('max_score', 1.0) for s in steps)
total_earned = sum(s.get('score', 0.0) for s in steps)

if total_possible > 0:
    final_score = round((total_earned / total_possible) * max_score, 2)
else:
    final_score = 0.0

status = 'failure' if failure_count > 0 else 'success'

result['score'] = final_score
result['max_score'] = max_score
result['status'] = status
result['final_score'] = final_score

with open(result_file, 'w', encoding='utf-8') as f:
    json.dump(result, f, ensure_ascii=False, indent=2)

print(f"KB 执行完成：status={status}, score={final_score}/{max_score}")
PYTHON_EOF
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
