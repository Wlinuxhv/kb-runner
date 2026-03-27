#!/bin/bash
# KB 验证工具 - 检查 AI 生成的 KB 脚本和 Skill.md 是否符合要求

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 统计
total_checks=0
passed_checks=0
failed_checks=0

# 检查函数
check_file_exists() {
    local file="$1"
    local desc="$2"
    total_checks=$((total_checks + 1))
    
    if [ -f "$file" ]; then
        echo -e "${GREEN}✓${NC} $desc: 存在"
        passed_checks=$((passed_checks + 1))
        return 0
    else
        echo -e "${RED}✗${NC} $desc: 不存在 - $file"
        failed_checks=$((failed_checks + 1))
        return 1
    fi
}

check_file_not_empty() {
    local file="$1"
    local desc="$2"
    total_checks=$((total_checks + 1))
    
    if [ -s "$file" ]; then
        echo -e "${GREEN}✓${NC} $desc: 不为空"
        passed_checks=$((passed_checks + 1))
        return 0
    else
        echo -e "${RED}✗${NC} $desc: 文件为空 - $file"
        failed_checks=$((failed_checks + 1))
        return 1
    fi
}

check_content_contains() {
    local file="$1"
    local pattern="$2"
    local desc="$3"
    total_checks=$((total_checks + 1))
    
    if grep -q "$pattern" "$file" 2>/dev/null; then
        echo -e "${GREEN}✓${NC} $desc: 包含 '$pattern'"
        passed_checks=$((passed_checks + 1))
        return 0
    else
        echo -e "${RED}✗${NC} $desc: 缺少 '$pattern'"
        failed_checks=$((failed_checks + 1))
        return 1
    fi
}

check_script_has_function() {
    local file="$1"
    local func="$2"
    local desc="$3"
    total_checks=$((total_checks + 1))
    
    if grep -q "$func" "$file" 2>/dev/null; then
        echo -e "${GREEN}✓${NC} $desc: 调用 $func"
        passed_checks=$((passed_checks + 1))
        return 0
    else
        echo -e "${RED}✗${NC} $desc: 缺少 $func 调用"
        failed_checks=$((failed_checks + 1))
        return 1
    fi
}

# 主检查函数
check_kb_directory() {
    local kb_dir="$1"
    local kb_name
    kb_name=$(basename "$kb_dir")
    
    echo ""
    echo "=========================================="
    echo "检查 KB: $kb_name"
    echo "=========================================="
    
    local has_error=false
    
    # 1. 检查必需文件
    echo ""
    echo "=== 文件检查 ==="
    check_file_exists "$kb_dir/Skill.md" "Skill.md" || has_error=true
    check_file_exists "$kb_dir/run.sh" "run.sh" || has_error=true
    check_file_exists "$kb_dir/case.yaml" "case.yaml" || has_error=true
    
    if [ "$has_error" = true ]; then
        echo -e "${RED}✗ 缺少必需文件，跳过后续检查${NC}"
        return 1
    fi
    
    # 2. 检查 Skill.md 内容
    echo ""
    echo "=== Skill.md 内容检查 ==="
    check_file_not_empty "$kb_dir/Skill.md" "Skill.md" || return 1
    
    # 检查 Skill.md 必需部分
    check_content_contains "$kb_dir/Skill.md" "KB ID" "Skill.md" || has_error=true
    check_content_contains "$kb_dir/Skill.md" -i "问题\|问题描述\|概述" "Skill.md (问题描述)" || has_error=true
    check_content_contains "$kb_dir/Skill.md" -i "步骤\|排查" "Skill.md (排查步骤)" || has_error=true
    check_content_contains "$kb_dir/Skill.md" -i "解决\|方案\|处理" "Skill.md (解决方案)" || has_error=true
    check_content_contains "$kb_dir/Skill.md" -i "根因\|原因" "Skill.md (根因分析)" || has_error=true
    
    # 3. 检查 run.sh 内容
    echo ""
    echo "=== run.sh 脚本检查 ==="
    check_file_not_empty "$kb_dir/run.sh" "run.sh" || return 1
    
    # 检查必需的 API 调用
    check_script_has_function "$kb_dir/run.sh" "kb_init" "run.sh" || has_error=true
    check_script_has_function "$kb_dir/run.sh" "kb_save" "run.sh" || has_error=true
    check_script_has_function "$kb_dir/run.sh" "step_start" "run.sh" || has_error=true
    check_script_has_function "$kb_dir/run.sh" "step_success\|step_failure\|step_warning\|step_info" "run.sh (步骤结束函数)" || has_error=true
    
    # 检查是否 source api.sh
    check_content_contains "$kb_dir/run.sh" "source.*api.sh" "run.sh" || has_error=true
    
    # 4. 检查 offline 模式处理
    echo ""
    echo "=== Offline 模式检查 ==="
    if grep -q "KB_RUN_MODE.*offline" "$kb_dir/run.sh"; then
        echo -e "${YELLOW}⚠${NC} 检测到 offline 模式处理"
        
        # 检查是否有提前 exit
        if grep -A 5 "KB_RUN_MODE.*offline" "$kb_dir/run.sh" | grep -q "exit 0"; then
            echo -e "${RED}✗${NC} Offline 模式下有提前 exit 0，这会导致步骤不完整"
            has_error=true
        else
            echo -e "${GREEN}✓${NC} Offline 模式处理正确（无提前退出）"
            passed_checks=$((passed_checks + 1))
        fi
        total_checks=$((total_checks + 1))
    else
        echo -e "${GREEN}✓${NC} 未特殊处理 offline 模式（在线模式脚本）"
        passed_checks=$((passed_checks + 1))
        total_checks=$((total_checks + 1))
    fi
    
    # 5. 检查步骤数量
    echo ""
    echo "=== 步骤数量检查 ==="
    local step_count
    step_count=$(grep -c "step_start" "$kb_dir/run.sh" 2>/dev/null || echo "0")
    
    if [ "$step_count" -ge 3 ]; then
        echo -e "${GREEN}✓${NC} 步骤数量充足：$step_count 个步骤"
        passed_checks=$((passed_checks + 1))
    else
        echo -e "${YELLOW}⚠${NC} 步骤数量较少：$step_count 个步骤（建议至少 3 个）"
        passed_checks=$((passed_checks + 1))  # 不视为错误，只是警告
    fi
    total_checks=$((total_checks + 1))
    
    # 6. 检查 case.yaml
    echo ""
    echo "=== case.yaml 检查 ==="
    check_content_contains "$kb_dir/case.yaml" "name:" "case.yaml" || has_error=true
    check_content_contains "$kb_dir/case.yaml" "scoring:" "case.yaml (得分配置)" || has_error=true
    check_content_contains "$kb_dir/case.yaml" "steps:" "case.yaml (步骤配置)" || has_error=true
    
    echo ""
    echo "=========================================="
    if [ "$has_error" = true ]; then
        echo -e "${RED}✗ KB: $kb_name 检查失败${NC}"
        return 1
    else
        echo -e "${GREEN}✓ KB: $kb_name 检查通过${NC}"
        return 0
    fi
}

# 主程序
main() {
    local target="${1:-all}"
    
    echo "=========================================="
    echo "KB 验证工具"
    echo "=========================================="
    echo ""
    
    local total_kbs=0
    local passed_kbs=0
    local failed_kbs=0
    
    if [ "$target" = "all" ]; then
        # 检查所有 KB
        for kb_dir in kbscript/*/; do
            [ -d "$kb_dir" ] || continue
            
            # 跳过空目录
            if [ ! -f "$kb_dir/run.sh" ]; then
                echo "跳过空目录：$kb_dir"
                continue
            fi
            
            total_kbs=$((total_kbs + 1))
            
            if check_kb_directory "$kb_dir"; then
                passed_kbs=$((passed_kbs + 1))
            else
                failed_kbs=$((failed_kbs + 1))
            fi
        done
    else
        # 检查指定 KB
        if [ -d "kbscript/$target" ]; then
            total_kbs=1
            if check_kb_directory "kbscript/$target"; then
                passed_kbs=1
            else
                failed_kbs=1
            fi
        else
            echo -e "${RED}错误：找不到 KB 目录：kbscript/$target${NC}"
            exit 1
        fi
    fi
    
    # 总结
    echo ""
    echo "=========================================="
    echo "检查总结"
    echo "=========================================="
    echo "KB 总数：$total_kbs"
    echo -e "通过：${GREEN}$passed_kbs${NC}"
    echo -e "失败：${RED}$failed_kbs${NC}"
    echo ""
    echo "检查项总数：$total_checks"
    echo -e "通过：${GREEN}$passed_checks${NC}"
    echo -e "失败：${RED}$failed_checks${NC}"
    echo "通过率：$(echo "scale=2; $passed_checks * 100 / $total_checks" | bc)%"
    echo "=========================================="
    
    if [ "$failed_kbs" -gt 0 ] || [ "$failed_checks" -gt 0 ]; then
        exit 1
    fi
    
    exit 0
}

# 显示帮助
if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    echo "用法：$0 [KB 名称|all]"
    echo ""
    echo "检查 AI 生成的 KB 脚本和 Skill.md 是否符合要求"
    echo ""
    echo "示例:"
    echo "  $0                    # 检查所有 KB"
    echo "  $0 all                # 检查所有 KB"
    echo "  $0 KB 名称 -ID          # 检查指定 KB"
    exit 0
fi

main "$@"
