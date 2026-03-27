#!/bin/bash
# KB Runner - 批量执行脚本
# 用于批量执行多个 KB 脚本并生成带排名的 JSON 结果文件

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# 配置
KB_DIR="${KB_DIR:-$SCRIPT_DIR}"
RESULT_DIR="${RESULT_DIR:-$PROJECT_ROOT/temp}"
LOG_DIR="${LOG_DIR:-$PROJECT_ROOT/logs}"
MAX_SCORE="${MAX_SCORE:-100.0}"

# 确保目录存在
mkdir -p "$RESULT_DIR" "$LOG_DIR"

# 生成执行 ID
EXEC_ID="exec-$(date +%s)-$$"
RESULT_FILE="$RESULT_DIR/${EXEC_ID}_result.json"
LOG_FILE="$LOG_DIR/${EXEC_ID}.log"

# 打印用法
usage() {
    echo "用法：$0 [KB 名称列表]"
    echo ""
    echo "参数:"
    echo "  KB 名称列表    空格分隔的 KB 名称列表，如果不提供则执行所有 KB"
    echo ""
    echo "示例:"
    echo "  $0 test_case \"A100 显卡问题 -32920\""
    echo "  $0  # 执行所有 KB"
    exit 1
}

# 获取所有 KB 目录
get_all_kb_dirs() {
    find "$KB_DIR" -maxdepth 1 -mindepth 1 -type d -exec basename {} \; 2>/dev/null | sort
}

# 执行单个 KB
execute_kb() {
    local kb_name="$1"
    local kb_dir="$KB_DIR/$kb_name"
    local kb_result_file="$RESULT_DIR/${kb_name}_result.json"
    
    if [ ! -d "$kb_dir" ]; then
        echo "跳过：$kb_name (目录不存在)"
        return 1
    fi
    
    if [ ! -f "$kb_dir/run.sh" ]; then
        echo "跳过：$kb_name (缺少 run.sh)"
        return 1
    fi
    
    echo ""
    echo "=========================================="
    echo "执行 KB: $kb_name"
    echo "=========================================="
    
    # 设置环境变量
    export KB_SCRIPT_NAME="$kb_name"
    export KB_RESULT_FILE="$kb_result_file"
    export KB_LOG_FILE="$LOG_FILE"
    export KB_MAX_SCORE="$MAX_SCORE"
    
    # 执行脚本
    local exit_code=0
    bash "$kb_dir/run.sh" || exit_code=$?
    
    if [ $exit_code -eq 0 ] && [ -f "$kb_result_file" ]; then
        echo "KB 完成：$kb_name (结果：$kb_result_file)"
        return 0
    else
        echo "KB 失败：$kb_name (退出码：$exit_code)"
        return 1
    fi
}

# 合并结果并生成排名
merge_results() {
    local kb_names=("$@")
    local temp_results=()
    
    echo ""
    echo "=========================================="
    echo "合并结果并生成排名"
    echo "=========================================="
    
    # 收集所有结果
    for kb_name in "${kb_names[@]}"; do
        local kb_result_file="$RESULT_DIR/${kb_name}_result.json"
        if [ -f "$kb_result_file" ]; then
            temp_results+=("$kb_result_file")
        fi
    done
    
    if [ ${#temp_results[@]} -eq 0 ]; then
        echo "错误：没有生成任何结果文件"
        exit 1
    fi
    
    # 使用 jq 合并结果并生成排名（需要 jq 支持）
    if command -v jq &> /dev/null; then
        # 创建临时文件数组
        local temp_array="[]"
        for rf in "${temp_results[@]}"; do
            temp_array=$(echo "$temp_array" | jq --slurpfile r "$rf" '. + $r')
        done
        
        # 生成排名和统计
        jq -n \
            --arg exec_id "$EXEC_ID" \
            --arg timestamp "$(date -Iseconds)" \
            --arg kb_dir "$KB_DIR" \
            --arg result_dir "$RESULT_DIR" \
            --arg log_file "$LOG_FILE" \
            --argjson results "$temp_array" \
            '
            # 按分数排序
            ($results | sort_by(-.score)) as $sorted |
            
            # 添加排名
            ($sorted | to_entries | map(.value + {rank: (.key + 1)})) as $ranked |
            
            # 计算统计
            {
                execution_id: $exec_id,
                timestamp: $timestamp,
                total_kbs: ($ranked | length),
                success_count: ([$ranked[] | select(.status == "success")] | length),
                failure_count: ([$ranked[] | select(.status == "failure")] | length),
                average_score: (if ($ranked | length) > 0 then ([$ranked[].score] | add) / ($ranked | length) else 0 end),
                ranked_results: $ranked,
                extensions: {
                    kb_directory: $kb_dir,
                    result_directory: $result_dir,
                    log_file: $log_file
                }
            }
            ' > "$RESULT_FILE"
        
        echo "结果已写入：$RESULT_FILE"
        echo ""
        echo "排名结果:"
        echo "------------------------------------------------------------"
        printf "%-6s %-40s %-10s %-10s\n" "排名" "KB 名称" "得分" "状态"
        echo "------------------------------------------------------------"
        jq -r '.ranked_results[] | "\(.rank)\t\(.name)\t\(.score)\t\(.status)"' "$RESULT_FILE" | \
            while IFS=$'\t' read -r rank name score status; do
                printf "%-6s %-40s %-10s %-10s\n" "$rank" "${name:0:40}" "$score" "$status"
            done
        echo "------------------------------------------------------------"
    else
        # 没有 jq，使用简单的 JSON 合并
        echo "{" > "$RESULT_FILE"
        echo "  \"execution_id\": \"$EXEC_ID\"," >> "$RESULT_FILE"
        echo "  \"timestamp\": \"$(date -Iseconds)\"," >> "$RESULT_FILE"
        echo "  \"kbs\": [" >> "$RESULT_FILE"
        
        local first=true
        for kb_result_file in "${temp_results[@]}"; do
            if [ "$first" = true ]; then
                first=false
            else
                echo "," >> "$RESULT_FILE"
            fi
            cat "$kb_result_file" >> "$RESULT_FILE"
        done
        
        echo "" >> "$RESULT_FILE"
        echo "  ]" >> "$RESULT_FILE"
        echo "}" >> "$RESULT_FILE"
        
        echo "结果已写入：$RESULT_FILE (简化版，需要安装 jq 获得完整功能)"
    fi
}

# 主程序
main() {
    local kb_names=("$@")
    
    # 如果没有提供 KB 名称，使用所有 KB
    if [ ${#kb_names[@]} -eq 0 ]; then
        echo "未指定 KB 名称，将执行所有 KB..."
        kb_names=($(get_all_kb_dirs))
    fi
    
    echo "=========================================="
    echo "KB Runner - 批量执行"
    echo "执行 ID: $EXEC_ID"
    echo "KB 目录：$KB_DIR"
    echo "结果目录：$RESULT_DIR"
    echo "日志文件：$LOG_FILE"
    echo "=========================================="
    echo ""
    
    local executed=0
    local succeeded=0
    local failed=0
    
    for kb_name in "${kb_names[@]}"; do
        if execute_kb "$kb_name"; then
            succeeded=$((succeeded + 1))
        else
            failed=$((failed + 1))
        fi
        executed=$((executed + 1))
    done
    
    echo ""
    echo "=========================================="
    echo "执行统计"
    echo "=========================================="
    echo "总 KB 数：$executed"
    echo "成功：$succeeded"
    echo "失败：$failed"
    echo "=========================================="
    
    # 合并结果
    merge_results "${kb_names[@]}"
    
    echo ""
    echo "执行完成！"
}

# 显示帮助
if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    usage
fi

# 执行
main "$@"
