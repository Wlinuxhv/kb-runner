#!/bin/bash
# 智能日志适配器 - 简化版 Bash API
# 技术支持只需提供Q单号，其他自动处理

# 日志根路径
SMART_LOG_ROOT="${SMART_LOG_ROOT:-/sf/data/icare_log/logall}"

# ========== 核心函数 ==========

# 初始化（只需Q单号）
smart_log_init() {
    local qno="$1"

    if [[ -z "$qno" ]]; then
        echo "Error: Q单号不能为空"
        return 1
    fi

    # 验证Q单号格式
    if ! [[ "$qno" =~ ^Q[0-9]{13}$ ]]; then
        echo "Error: Q单号格式错误，应为Q+13位数字"
        return 1
    fi

    # 解析年月份
    local year="${qno:1:4}"
    local month="${qno:5:2}"
    _SMART_YM="${year:2:2}${month}"

    # 查找主机目录
    local qno_dir="$SMART_LOG_ROOT/$_SMART_YM/$qno"

    # 尝试在不同年月份中查找
    if [[ ! -d "$qno_dir" ]]; then
        for ym in "$SMART_LOG_ROOT"/*/; do
            if [[ -d "${ym}${qno}" ]]; then
                qno_dir="${ym}${qno}"
                _SMART_YM=$(basename "$ym")
                break
            fi
        done
    fi

    if [[ ! -d "$qno_dir" ]]; then
        echo "Error: Q单号目录不存在: $_SMART_YM/$qno"
        return 1
    fi

    # 设置全局变量
    export _SMART_QNO="$qno"
    export _SMART_YM="$_SMART_YM"

    # 查找主机（排除特殊文件）
    local host=""
    for item in "$qno_dir"/*; do
        [[ -d "$item" ]] || continue
        local name=$(basename "$item")
        # 排除特殊文件和目录
        [[ "$name" == "_members" ]] && continue
        [[ "$name" == "cfgmaster_ini" ]] && continue
        [[ "$name" == "log_list.json" ]] && continue
        [[ "$name" == *.zip ]] && continue
        host="$name"
        break
    done

    export _SMART_HOST="$host"
    export _SMART_LOG_DIR="$qno_dir/$_SMART_HOST/log"

    # 自动解压
    _smart_extract

    # 打印状态
    smart_log_info
}

# 检查命令是否存在
_check_cmd() {
    command -v "$1" >/dev/null 2>&1
}

# 自动解压
_smart_extract() {
    # 检查是否有zstd命令
    local has_zstd=0
    _check_cmd zstd && has_zstd=1

    # 解压tar.gz (递归)
    find "$_SMART_LOG_DIR" -name "*.tar.gz" -type f 2>/dev/null | while read -r f; do
        local dir="${f%.tar.gz}"
        if [[ ! -d "$dir" ]]; then
            echo "[自动解压] $(basename "$f")"
            tar -xzf "$f" -C "$(dirname "$f")" 2>/dev/null &
        fi
    done

    # 解压tar.zst (根目录)
    if [[ $has_zstd -eq 1 ]]; then
        find "$_SMART_LOG_DIR" -maxdepth 1 -name "*.tar.zst" -type f 2>/dev/null | while read -r f; do
            local dir="${f%.tar.zst}"
            if [[ ! -d "$dir" ]]; then
                echo "[自动解压] $(basename "$f")"
                tar -I zstd -xf "$f" -C "$(dirname "$f")" 2>/dev/null &
            fi
        done
    fi

    # 解压zip (递归)
    find "$_SMART_LOG_DIR" -name "*.zip" -type f 2>/dev/null | while read -r f; do
        local dir="${f%.zip}"
        if [[ ! -d "$dir" ]]; then
            echo "[自动解压] $(basename "$f")"
            unzip -o "$f" -d "$(dirname "$f")" 2>/dev/null &
        fi
    done

    wait

    # 递归处理子目录中的压缩文件 (如 blackbox/20260307/)
    _smart_extract_recursive
}

# 递归解压子目录
_smart_extract_recursive() {
    # 继续处理子目录中的压缩文件 (确保解压出来的文件也被处理)
    local changed=1
    local count=0

    while [[ $changed -eq 1 && $count -lt 3 ]]; do
        changed=0
        count=$((count + 1))

        # 处理新的zip文件
        find "$_SMART_LOG_DIR" -name "*.zip" -type f 2>/dev/null | while read -r f; do
            local dir="${f%.zip}"
            if [[ ! -d "$dir" ]]; then
                echo "[自动解压] $(basename "$f")"
                unzip -o "$f" -d "$(dirname "$f")" 2>/dev/null &
                changed=1
            fi
        done
        wait

        # 处理新的tar.gz文件
        find "$_SMART_LOG_DIR" -name "*.tar.gz" -type f 2>/dev/null | while read -r f; do
            local dir="${f%.tar.gz}"
            if [[ ! -d "$dir" ]]; then
                echo "[自动解压] $(basename "$f")"
                tar -xzf "$f" -C "$(dirname "$f")" 2>/dev/null &
                changed=1
            fi
        done
        wait
    done
}

# ========== 查询函数 ==========

# 获取日志目录
smart_log_dir() {
    echo "$_SMART_LOG_DIR"
}

# 获取主机
smart_log_host() {
    echo "$_SMART_HOST"
}

# 列出文件
smart_log_ls() {
    local subdir="${1:-}"
    local dir="$_SMART_LOG_DIR/$subdir"

    if [[ -d "$dir" ]]; then
        ls -la "$dir"
    else
        echo "目录不存在: $dir"
    fi
}

# 搜索日志
smart_log_grep() {
    local keyword="$1"
    local subdir="${2:-blackbox}"

    if [[ -z "$keyword" ]]; then
        echo "用法: smart_log_grep <关键字> [子目录]"
        return 1
    fi

    local dir="$_SMART_LOG_DIR/$subdir"
    if [[ ! -d "$dir" ]]; then
        echo "目录不存在: $dir"
        return 1
    fi

    find "$dir" -type f \( -name "*.txt" -o -name "*.log" \) 2>/dev/null | while read -r f; do
        grep -H "$keyword" "$f" 2>/dev/null | head -3
    done | head -50
}

# 读取日志
smart_log_cat() {
    local filepath="$1"

    if [[ -z "$filepath" ]]; then
        echo "用法: smart_log_cat <文件路径>"
        return 1
    fi

    local fullpath="$_SMART_LOG_DIR/$filepath"
    if [[ -f "$fullpath" ]]; then
        cat "$fullpath"
    else
        echo "文件不存在: $filepath"
    fi
}

# tail日志
smart_log_tail() {
    local filepath="$1"
    local lines="${2:-100}"

    local fullpath="$_SMART_LOG_DIR/$filepath"
    if [[ -f "$fullpath" ]]; then
        tail -n "$lines" "$fullpath"
    else
        echo "文件不存在: $filepath"
    fi
}

# ========== 数据库函数（使用Python） ==========

# 获取数据库路径
_smart_db_path() {
    echo "$_SMART_LOG_DIR/log_new.db"
}

# SQL查询（使用Python）
smart_log_sql() {
    local sql="$1"
    local db="$(_smart_db_path)"

    if [[ ! -f "$db" ]]; then
        echo "数据库不存在"
        return 1
    fi

    python3 -c "
import sqlite3
import sys
import json

try:
    conn = sqlite3.connect('$db')
    conn.row_factory = sqlite3.Row
    cursor = conn.cursor()
    cursor.execute('$sql')

    rows = cursor.fetchall()
    if not rows:
        print('[]')
    else:
        results = []
        for row in rows:
            results.append(dict(row))
        print(json.dumps(results, ensure_ascii=False, indent=2))

    conn.close()
except Exception as e:
    print(json.dumps({'error': str(e)}))
" 2>/dev/null
}

# 查询告警
smart_log_alerts() {
    local level="${1:-}"
    local limit="${2:-100}"

    local sql="SELECT id, type, host, object_name, description, start, level FROM alert"

    if [[ -n "$level" ]]; then
        sql="$sql WHERE level='$level'"
    fi

    sql="$sql ORDER BY start DESC LIMIT $limit"

    smart_log_sql "$sql"
}

# 查询操作日志
smart_log_logs() {
    local user="${1:-}"
    local limit="${2:-100}"

    local sql="SELECT id, type, host, user, description, start, end, status FROM log"

    if [[ -n "$user" ]]; then
        sql="$sql WHERE user='$user'"
    fi

    sql="$sql ORDER BY start DESC LIMIT $limit"

    smart_log_sql "$sql"
}

# 搜索告警/日志
smart_log_search() {
    local keyword="$1"
    local limit="${2:-50}"

    local db="$(_smart_db_path)"
    if [[ ! -f "$db" ]]; then
        echo "数据库不存在"
        return 1
    fi

    python3 -c "
import sqlite3
import json

try:
    conn = sqlite3.connect('$db')
    conn.row_factory = sqlite3.Row
    cursor = conn.cursor()

    # 搜索告警
    cursor.execute('''
        SELECT \"alert\" as src, id, object_name, description, start, level
        FROM alert
        WHERE description LIKE ? OR object_name LIKE ?
        LIMIT $limit
    ''', ('%$keyword%', '%$keyword%'))
    alerts = [dict(row) for row in cursor.fetchall()]

    # 搜索日志
    cursor.execute('''
        SELECT \"log\" as src, id, user, description, start, status
        FROM log
        WHERE description LIKE ? OR user LIKE ?
        LIMIT $limit
    ''', ('%$keyword%', '%$keyword%'))
    logs = [dict(row) for row in cursor.fetchall()]

    print(json.dumps({'alerts': alerts, 'logs': logs}, ensure_ascii=False, indent=2))
    conn.close()
except Exception as e:
    print(json.dumps({'error': str(e)}))
" 2>/dev/null
}

# 统计
smart_log_stats() {
    local db="$(_smart_db_path)"

    if [[ ! -f "$db" ]]; then
        echo "数据库不存在"
        return
    fi

    python3 -c "
import sqlite3
import json

try:
    conn = sqlite3.connect('$db')
    cursor = conn.cursor()

    cursor.execute('SELECT COUNT(*) FROM alert')
    alert_cnt = cursor.fetchone()[0]

    cursor.execute('SELECT COUNT(*) FROM log')
    log_cnt = cursor.fetchone()[0]

    cursor.execute('SELECT level, COUNT(*) as cnt FROM alert GROUP BY level ORDER BY cnt DESC')
    levels = [{'level': r[0], 'count': r[1]} for r in cursor.fetchall()]

    print(json.dumps({'alert_count': alert_cnt, 'log_count': log_cnt, 'levels': levels}, indent=2))
    conn.close()
except Exception as e:
    print(json.dumps({'error': str(e)}))
" 2>/dev/null
}

# ========== 便捷函数 ==========

# 状态信息
smart_log_info() {
    echo "========================================"
    echo "  智能日志适配器"
    echo "========================================"
    echo "  Q单号:   $_SMART_QNO"
    echo "  年月份:  $_SMART_YM"
    echo "  主机:    $_SMART_HOST"
    echo "  日志目录: $_SMART_LOG_DIR"
    echo "========================================"

    # 显示日志类型
    echo ""
    echo "已识别的日志类型:"

    [[ -d "$_SMART_LOG_DIR/blackbox" ]] && echo "  ✓ blackbox (黑盒子日志)"
    [[ -f "$_SMART_LOG_DIR/log_new.db" ]] && echo "  ✓ log_new.db (数据库日志)"
    [[ -d "$_SMART_LOG_DIR/checkitem" ]] && echo "  ✓ checkitem (监控指标)"

    # 显示子目录
    if [[ -d "$_SMART_LOG_DIR" ]]; then
        echo ""
        echo "可用子目录:"
        ls -1 "$_SMART_LOG_DIR" | grep -vE '^_' | grep -vE '\.(txt|json|ini)$' | head -10 | sed 's/^/  - /'
    fi

    # 显示数据库统计
    if [[ -f "$_SMART_LOG_DIR/log_new.db" ]]; then
        local stats
        stats=$(smart_log_stats)
        local alert_cnt=$(echo "$stats" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('alert_count',0))" 2>/dev/null || echo "0")
        local log_cnt=$(echo "$stats" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('log_count',0))" 2>/dev/null || echo "0")
        echo ""
        echo "数据库: $alert_cnt 条告警, $log_cnt 条操作日志"
    fi
}

# 一键初始化
smart_icare() {
    smart_log_init "$1"
}

# 帮助
smart_log_help() {
    cat <<EOF
智能日志适配器 - 简化版API

用法:
    source smart_log_api.sh              # 加载脚本
    smart_icare Q2026031700281           # 一键初始化
    smart_log_init Q2026031700281        # 初始化（同样效果）

查询函数:
    smart_log_dir                        # 获取日志目录
    smart_log_host                       # 获取主机名
    smart_log_ls [子目录]                # 列出文件
    smart_log_grep <关键字> [子目录]     # 搜索日志文件
    smart_log_cat <文件路径>             # 读取文件
    smart_log_tail <文件路径> [行数]     # 读取文件尾部

数据库函数:
    smart_log_sql <SQL语句>              # 执行SQL查询
    smart_log_alerts [级别] [数量]       # 查询告警
    smart_log_logs [用户] [数量]         # 查询操作日志
    smart_log_search <关键字>            # 搜索告警和日志
    smart_log_stats                      # 统计信息

便捷函数:
    smart_log_info                       # 显示状态
    smart_log_help                       # 显示帮助

示例:
    smart_icare Q2026031700281           # 初始化
    smart_log_info                       # 查看状态
    smart_log_alerts critical            # 查询严重告警
    smart_log_search disk                # 搜索关键字
    smart_log_cat blackbox/20260315/LOG_dmesg.txt  # 读取日志
EOF
}

# 自动初始化（如果传入参数）
if [[ -n "$1" ]]; then
    smart_log_init "$1"
fi
