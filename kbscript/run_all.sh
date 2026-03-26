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
    
    # 使用 Python 合并结果（如果可用）
    if command -v python3 &> /dev/null; then
        python3 << EOF
import json
import os
from datetime import datetime

result_files = ${temp_results[@]+"$(printf '"%s",' "${temp_results[@]}" | sed 's/,$//')"}
results = []

for f in result_files:
    try:
        with open(f, 'r') as file:
            data = json.load(file)
            results.append({
                'name': data.get('script_name', os.path.basename(f).replace('_result.json', '')),
                'score': data.get('score', 0),
                'max_score': data.get('max_score', 100.0),
                'status': data.get('status', 'unknown'),
                'steps': data.get('steps', []),
                'results': data.get('results', {}),
                'start_time': data.get('start_time', ''),
                'end_time': data.get('end_time', '')
            })
    except Exception as e:
        print(f"警告：读取 {f} 失败：{e}")

# 按分数排序
results.sort(key=lambda x: x['score'], reverse=True)

# 添加排名
for i, r in enumerate(results):
    r['rank'] = i + 1

# 生成最终结果
final_result = {
    'execution_id': '$EXEC_ID',
    'timestamp': datetime.now().isoformat(),
    'total_kbs': len(results),
    'success_count': sum(1 for r in results if r['status'] == 'success'),
    'failure_count': sum(1 for r in results if r['status'] == 'failure'),
    'average_score': sum(r['score'] for r in results) / len(results) if results else 0,
    'ranked_results': results,
    'extensions': {
        'kb_directory': '$KB_DIR',
        'result_directory': '$RESULT_DIR',
        'log_file': '$LOG_FILE'
    }
}

# 写入结果文件
output_file = '$RESULT_FILE'
with open(output_file, 'w', encoding='utf-8') as f:
    json.dump(final_result, f, ensure_ascii=False, indent=2)

print(f"结果已写入：{output_file}")
print("")
print("排名结果:")
print("-" * 60)
print(f"{'排名':<6}{'KB 名称':<40}{'得分':<10}{'状态':<10}")
print("-" * 60)
for r in results:
    print(f"{r['rank']:<6}{r['name'][:38]:<40}{r['score']:<10.2f}{r['status']:<10}")
print("-" * 60)
EOF
    else
        # 没有 Python，使用简单的 JSON 合并
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
        
        echo "结果已写入：$RESULT_FILE (简化版)"
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
