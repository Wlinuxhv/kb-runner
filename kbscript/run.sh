#!/bin/bash
# KB Runner - 统一入口脚本
# 所有 KB 脚本通过此脚本统一执行，根据传入的 KB 名称加载对应的配置和脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# 参数解析
KB_NAME="${1:-}"
KB_DIR=""
CONFIG_FILE=""
RUN_SCRIPT=""

# 日志文件
LOG_FILE="${KB_RUNNER_LOG_FILE:-$PROJECT_ROOT/logs/kb-runner.log}"
RESULT_FILE="${KB_RUNNER_RESULT_FILE:-$PROJECT_ROOT/temp/kb_result.json}"

# 打印用法
usage() {
    echo "用法：$0 <kb_name>"
    echo ""
    echo "参数:"
    echo "  kb_name    KB 脚本名称（目录名），例如：test_case"
    echo ""
    echo "示例:"
    echo "  $0 test_case"
    echo "  $0 A100 显卡问题 -32920"
    exit 1
}

# 检查参数
if [ -z "$KB_NAME" ]; then
    usage
fi

# 查找 KB 目录
if [ -d "$SCRIPT_DIR/$KB_NAME" ]; then
    KB_DIR="$SCRIPT_DIR/$KB_NAME"
else
    echo "错误：找不到 KB 目录：$KB_NAME" >&2
    exit 1
fi

# 查找配置文件和脚本
if [ -f "$KB_DIR/case.yaml" ]; then
    CONFIG_FILE="$KB_DIR/case.yaml"
else
    echo "警告：找不到配置文件 case.yaml，使用默认配置"
fi

if [ -f "$KB_DIR/run.sh" ]; then
    RUN_SCRIPT="$KB_DIR/run.sh"
else
    echo "错误：找不到脚本文件 run.sh" >&2
    exit 1
fi

# 检查脚本可执行权限
if [ ! -x "$RUN_SCRIPT" ]; then
    echo "警告：脚本没有执行权限，正在添加..."
    chmod +x "$RUN_SCRIPT"
fi

# 导出环境变量供子脚本使用
export KB_SCRIPT_NAME="$KB_NAME"
export KB_CONFIG_FILE="$CONFIG_FILE"
export KB_RESULT_FILE="$RESULT_FILE"
export KB_LOG_FILE="$LOG_FILE"
export KB_MAX_SCORE="${KB_MAX_SCORE:-100.0}"
export KB_RUN_MODE="${KB_RUN_MODE:-online}"

# 从配置文件中读取 max_score（如果存在）
if [ -f "$CONFIG_FILE" ] && command -v python3 &> /dev/null; then
    MAX_SCORE_FROM_CONFIG=$(python3 -c "
import yaml
import sys
try:
    with open('$CONFIG_FILE', 'r') as f:
        config = yaml.safe_load(f)
    max_score = config.get('scoring', {}).get('max_score', 100.0)
    print(max_score)
except:
    print('100.0')
" 2>/dev/null)
    if [ -n "$MAX_SCORE_FROM_CONFIG" ]; then
        export KB_MAX_SCORE="$MAX_SCORE_FROM_CONFIG"
    fi
fi

# 记录执行日志
echo "[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] 开始执行 KB: $KB_NAME" >> "$LOG_FILE"
echo "[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] 配置文件：$CONFIG_FILE" >> "$LOG_FILE"
echo "[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] 脚本文件：$RUN_SCRIPT" >> "$LOG_FILE"
echo "[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] 结果文件：$RESULT_FILE" >> "$LOG_FILE"

# 执行 KB 脚本
echo "=========================================="
echo "执行 KB: $KB_NAME"
echo "配置文件：$CONFIG_FILE"
echo "脚本文件：$RUN_SCRIPT"
echo "=========================================="
echo ""

# 执行脚本并捕获退出码
EXIT_CODE=0
bash "$RUN_SCRIPT" || EXIT_CODE=$?

# 记录执行结果
echo ""
echo "=========================================="
if [ $EXIT_CODE -eq 0 ]; then
    echo "KB 执行完成：$KB_NAME (成功)"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] KB 执行完成：$KB_NAME (成功)" >> "$LOG_FILE"
else
    echo "KB 执行完成：$KB_NAME (失败，退出码：$EXIT_CODE)"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [ERROR] KB 执行失败：$KB_NAME (退出码：$EXIT_CODE)" >> "$LOG_FILE"
fi
echo "结果文件：$RESULT_FILE"
echo "=========================================="

# 输出结果 JSON（如果存在）
if [ -f "$RESULT_FILE" ]; then
    echo ""
    echo "执行结果:"
    if command -v jq &> /dev/null; then
        jq '.' "$RESULT_FILE" 2>/dev/null || cat "$RESULT_FILE"
    else
        cat "$RESULT_FILE"
    fi
fi

exit $EXIT_CODE
