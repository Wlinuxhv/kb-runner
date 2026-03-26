#!/bin/bash
# ICare日志适配器 - Bash API (简化版)
# 技术支持人员只需提供Q单号，其他自动处理

# 日志根路径
ICARE_LOG_ROOT="${ICARE_LOG_ROOT:-/sf/data/icare_log/logall}"

# 全局变量
_ICARE_QNO=""           # Q单号（如Q2026031700424）
_ICARE_YEARMONTH=""     # 自动从Q单号解析：2603
_ICARE_HOST=""          # 当前主机
_ICARE_HOSTS=()         # 主机列表
_ICARE_EXTRACTED=""     # 解压目录

# ========== 初始化函数 ==========

# 初始化日志适配器（只需传入Q单号）
icare_log_init() {
    local qno="$1"

    if [ -z "$qno" ]; then
        echo "Error: Q单号不能为空"
        return 1
    fi

    # 验证Q单号格式
    if ! [[ "$qno" =~ ^Q[0-9]{12,13}$ ]]; then
        echo "Error: Q单号格式错误，应为Q+12-13位数字，如Q2026031700424或Q2026031201098"
        return 1
    fi

    _ICARE_QNO="$qno"

    # 从Q单号解析年月：Q2026031700424 -> 2603
    # Q单号格式：Q + 年(4位) + 月(2位) + 序号(6位)
    local year="${qno:1:4}"   # 2026
    local month="${qno:5:2}"  # 03
    _ICARE_YEARMONTH="${year:2:2}${month}"  # 2603

    # 自动查找主机
    _icare_find_hosts
}

# 从Q单号解析年月份
_icare_parse_yearmonth() {
    local qno="$1"
    echo "${qno:1:4}-${qno:5:2}"  # 返回如 2026-03
}

# 查找主机列表
_icare_find_hosts() {
    local qno_dir="$ICARE_LOG_ROOT/$_ICARE_YEARMONTH/$_ICARE_QNO"

    # 如果目录不存在，尝试在其他年月份中查找
    if [ ! -d "$qno_dir" ]; then
        # 搜索所有可能的年月份目录
        for ym_dir in "$ICARE_LOG_ROOT"/*/; do
            local test_dir="${ym_dir}${_ICARE_QNO}"
            if [ -d "$test_dir" ]; then
                qno_dir="$test_dir"
                _ICARE_YEARMONTH=$(basename "$ym_dir")
                break
            fi
        done
    fi

    _ICARE_HOSTS=()

    if [ ! -d "$qno_dir" ]; then
        return 1
    fi

    # 列出主机目录（排除特殊文件）
    for item in "$qno_dir"/*; do
        [ -d "$item" ] || continue
        local name=$(basename "$item")

        # 排除特殊文件和常见日志格式文件
        case "$name" in
            _members|cfgmaster_ini|collect_record.txt|log_list.json|*.zip)
                continue
                ;;
        esac

        _ICARE_HOSTS+=("$name")
    done

    # 设置第一个主机为当前主机
    if [ ${#_ICARE_HOSTS[@]} -gt 0 ]; then
        _ICARE_HOST="${_ICARE_HOSTS[0]}"
    fi
}

# 解压ZIP文件（如果存在）
_icare_extract_if_needed() {
    local qno_dir="$ICARE_LOG_ROOT/$_ICARE_YEARMONTH/$_ICARE_QNO"

    if [ ! -d "$qno_dir" ]; then
        return 1
    fi

    # 查找ZIP文件
    local zip_file=""
    for f in "$qno_dir"/*.zip; do
        if [ -f "$f" ]; then
            zip_file="$f"
            break
        fi
    done

    if [ -z "$zip_file" ]; then
        return 0  # 没有ZIP文件
    fi

    # 检查是否已经解压过
    local check_dir="${zip_file%.zip}"
    if [ -d "$check_dir" ]; then
        _ICARE_EXTRACTED="$check_dir"
        return 0  # 已经解压
    fi

    # 解压ZIP文件
    echo "正在解压: $(basename "$zip_file")"
    if command -v unzip &> /dev/null; then
        unzip -o "$zip_file" -d "$qno_dir" > /dev/null 2>&1
        if [ -d "$check_dir" ]; then
            _ICARE_EXTRACTED="$check_dir"
            echo "解压完成"
        fi
    else
        echo "Warning: unzip命令不可用，无法解压"
    fi
}

# ========== 获取信息函数 ==========

# 获取当前Q单号
icare_log_get_qno() {
    echo "$_ICARE_QNO"
}

# 获取解析后的年月份
icare_log_get_yearmonth() {
    echo "$_ICARE_YEARMONTH"
}

# 获取主机列表
icare_log_list_hosts() {
    if [ ${#_ICARE_HOSTS[@]} -eq 0 ]; then
        echo "[]"
        return
    fi

    local first=1
    echo "["
    for host in "${_ICARE_HOSTS[@]}"; do
        if [ $first -eq 1 ]; then
            first=0
        else
            echo ","
        fi
        echo -n "\"$host\""
    done
    echo "]"
}

# 设置当前主机
icare_log_set_host() {
    local host="$1"

    # 验证主机是否存在
    for h in "${_ICARE_HOSTS[@]}"; do
        if [ "$h" = "$host" ]; then
            _ICARE_HOST="$host"
            return 0
        fi
    done

    echo "Error: 主机不存在: $host"
    return 1
}

# 获取日志目录路径
icare_log_get_log_path() {
    # offline 模式：优先使用框架注入的最终日志根目录，避免遍历整个 host 目录导致性能问题
    if [ "${KB_RUN_MODE:-online}" = "offline" ] && [ -n "${KB_OFFLINE_LOG_DIR:-}" ]; then
        echo "$KB_OFFLINE_LOG_DIR"
        return 0
    fi
    echo "$ICARE_LOG_ROOT/$_ICARE_YEARMONTH/$_ICARE_QNO/$_ICARE_HOST"
}

# ========== 文件操作函数 ==========

# 获取日志文件列表
icare_log_list() {
    local log_path
    log_path=$(icare_log_get_log_path)

    if [ ! -d "$log_path" ]; then
        echo "[]"
        return
    fi

    local first=1
    echo "["
    while IFS= read -r -d '' filepath; do
        local relpath="${filepath#$log_path/}"
        local size=$(stat -c%s "$filepath" 2>/dev/null || echo 0)
        local mtime=$(stat -c%Y "$filepath" 2>/dev/null || echo 0)
        local relpath_escaped
        relpath_escaped=$(echo "$relpath" | sed 's/\\/\\\\/g; s/"/\\"/g')

        if [ $first -eq 1 ]; then
            first=0
        else
            echo ","
        fi

        printf '{"path":"%s","size":%d,"mtime":%d}' "$relpath_escaped" "$size" "$mtime"
    done < <(find "$log_path" -type f -print0 2>/dev/null)
    echo "]"
}

# 读取日志文件
icare_log_read() {
    local filepath="$1"
    local log_path
    log_path=$(icare_log_get_log_path)
    local fullpath="$log_path/$filepath"

    if [ -z "$filepath" ]; then
        echo "{\"error\":\"filepath is required\"}"
        return 1
    fi

    if [ ! -f "$fullpath" ]; then
        echo "{\"error\":\"file not found: $filepath\"}"
        return 1
    fi

    cat "$fullpath"
}

# 读取日志文件最后N行
icare_log_tail() {
    local filepath="$1"
    local lines="${2:-100}"
    local log_path
    log_path=$(icare_log_get_log_path)
    local fullpath="$log_path/$filepath"

    if [ ! -f "$fullpath" ]; then
        echo "{\"error\":\"file not found: $filepath\"}"
        return 1
    fi

    tail -n "$lines" "$fullpath"
}

# 搜索日志关键字
icare_log_search() {
    local keyword="$1"
    local log_path
    log_path=$(icare_log_get_log_path)

    if [ -z "$keyword" ]; then
        echo "[]"
        return
    fi

    if [ ! -d "$log_path" ]; then
        echo "[]"
        return
    fi

    local first=1
    echo "["

    while IFS= read -r -d '' filepath; do
        local relpath="${filepath#$log_path/}"
        local line_num=0
        while IFS= read -r line; do
            line_num=$((line_num + 1))
            if echo "$line" | grep -q "$keyword"; then
                local content_escaped
                content_escaped=$(echo "$line" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g')

                if [ $first -eq 1 ]; then
                    first=0
                else
                    echo ","
                fi

                printf '{"file":"%s","line":%d,"content":"%s"}' "$relpath" "$line_num" "$content_escaped"
            fi
        done < "$filepath"
    done < <(find "$log_path" -type f \( -name "*.log" -o -name "*.txt" -o -name "*.info" \) -print0 2>/dev/null)

    echo "]"
}

# 统计关键字出现次数
icare_log_count_keyword() {
    local keyword="$1"
    local log_path
    log_path=$(icare_log_get_log_path)

    if [ -z "$keyword" ] || [ ! -d "$log_path" ]; then
        echo "0"
        return
    fi

    grep -r "$keyword" "$log_path" --include="*.log" --include="*.txt" --include="*.info" 2>/dev/null | wc -l
}

# ========== 便捷函数 ==========

# 一键初始化（只需Q单号）
icare() {
    local qno="$1"
    local host="$2"

    icare_log_init "$qno"

    if [ -n "$host" ]; then
        icare_log_set_host "$host"
    fi

    # 显示状态
    icare_log_status
}

# 显示状态
icare_log_status() {
    echo "========================================"
    echo "  ICare日志适配器状态"
    echo "========================================"
    echo "  Q单号:     $_ICARE_QNO"
    echo "  年月份:    $_ICARE_YEARMONTH ($(icare_log_get_yearmonth))"
    echo "  日志路径:  $(icare_log_get_log_path)"
    echo "  主机数量:  ${#_ICARE_HOSTS[@]}"
    echo "  当前主机:  $_ICARE_HOST"
    echo "========================================"

    if [ ${#_ICARE_HOSTS[@]} -gt 1 ]; then
        echo "可用主机:"
        for h in "${_ICARE_HOSTS[@]}"; do
            echo "  - $h"
        done
        echo ""
        echo "切换主机: icare_log_set_host <主机名>"
    fi
}

# 快速查看日志（带分页）
icare_log_less() {
    local filepath="$1"
    local log_path
    log_path=$(icare_log_get_log_path)
    local fullpath="$log_path/$filepath"

    if [ ! -f "$fullpath" ]; then
        echo "文件不存在: $filepath"
        return 1
    fi

    less -R "$fullpath"
}

# 快速搜索（交互式）
icare_log_find() {
    local keyword="$1"
    local log_path
    log_path=$(icare_log_get_log_path)

    if [ -z "$keyword" ]; then
        echo "用法: icare_log_find <关键字>"
        return 1
    fi

    # 使用grep搜索并高亮显示
    grep -r "$keyword" "$log_path" --include="*.log" --include="*.txt" --include="*.info" \
        --color=always -n 2>/dev/null | head -100
}

# 获取帮助
icare_log_help() {
    cat <<EOF
ICare日志适配器 - 简化版API

使用方法：
    source icare_log_api.sh
    icare <Q单号> [主机名]

示例：
    source scripts/bash/icare_log_api.sh
    icare Q2026031700424              # 初始化并显示状态
    icare Q2026031700424 sp_10.250.0.7  # 指定主机

    # 或者分步操作
    icare_log_init Q2026031700424     # 只需Q单号
    icare_log_set_host sp_10.250.0.7  # 切换主机

API函数：
    icare <qno> [host]       # 一键初始化
    icare_log_init <qno>     # 初始化（只需Q单号）
    icare_log_set_host <host> # 切换主机
    icare_log_list_hosts     # 列出所有主机
    icare_log_list           # 获取日志文件列表
    icare_log_read <path>    # 读取日志文件
    icare_log_tail <path> [n] # 读取最后N行
    icare_log_search <word>  # 搜索关键字
    icare_log_count_keyword <word> # 统计关键字次数
    icare_log_status         # 显示状态
    icare_log_less <path>    # 分页查看日志
    icare_log_find <word>    # 高亮搜索

注意事项：
    - 只需提供Q单号，自动解析年月份
    - 自动查找主机列表
    - 自动解压ZIP文件（如有）
    - 主机名可选，不指定则默认第一个
EOF
}
