#!/bin/bash
# ============================================
# KB脚本执行框架 - 标准化测试脚本
# ============================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_DIR="$PROJECT_ROOT/bin"
KB_RUNNER="$BIN_DIR/kb-runner.exe"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 测试统计
TOTAL=0
PASSED=0
FAILED=0
SKIPPED=0

echo_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

echo_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((PASSED++))
    ((TOTAL++))
}

echo_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((FAILED++))
    ((TOTAL++))
}

echo_skip() {
    echo -e "${YELLOW}[SKIP]${NC} $1"
    ((SKIPPED++))
    ((TOTAL++))
}

echo_section() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE} $1${NC}"
    echo -e "${BLUE}========================================${NC}"
}

# 检查KB_RUNNER是否存在
check_binary() {
    if [ ! -f "$KB_RUNNER" ]; then
        echo_info "编译 kb-runner..."
        cd "$PROJECT_ROOT"
        go build -o "$KB_RUNNER" ./cmd/kb-runner
    fi
}

# ============================================
# 测试: 基础命令测试
# ============================================
test_help() {
    echo_section "基础命令测试"
    
    $KB_RUNNER --help > /dev/null 2>&1 && echo_pass "帮助信息" || echo_fail "帮助信息"
    $KB_RUNNER version > /dev/null 2>&1 && echo_pass "版本信息" || echo_fail "版本信息"
    $KB_RUNNER list --help > /dev/null 2>&1 && echo_pass "list帮助" || echo_fail "list帮助"
    $KB_RUNNER run --help > /dev/null 2>&1 && echo_pass "run帮助" || echo_fail "run帮助"
    $KB_RUNNER init --help > /dev/null 2>&1 && echo_pass "init帮助" || echo_fail "init帮助"
    $KB_RUNNER scenario --help > /dev/null 2>&1 && echo_pass "scenario帮助" || echo_fail "scenario帮助"
}

# ============================================
# 测试: CASE管理功能
# ============================================
test_case_list() {
    echo_section "CASE管理功能测试"
    
    # 列出所有CASE
    $KB_RUNNER list > /dev/null 2>&1 && echo_pass "CASE列表" || echo_fail "CASE列表"
    
    # 按分类筛选
    $KB_RUNNER list --category security > /dev/null 2>&1 && echo_pass "分类筛选" || echo_fail "分类筛选"
    
    # 按标签筛选
    $KB_RUNNER list --tags critical > /dev/null 2>&1 && echo_pass "标签筛选" || echo_fail "标签筛选"
    
    # 搜索
    $KB_RUNNER list --search "check" > /dev/null 2>&1 && echo_pass "CASE搜索" || echo_fail "CASE搜索"
    
    # 查看详情
    $KB_RUNNER show security_check > /dev/null 2>&1 && echo_pass "CASE详情" || echo_fail "CASE详情"
}

# ============================================
# 测试: 场景管理功能
# ============================================
test_scenario() {
    echo_section "场景管理功能测试"
    
    # 场景列表
    $KB_RUNNER scenario list > /dev/null 2>&1 && echo_pass "场景列表" || echo_fail "场景列表"
    
    # 场景详情
    $KB_RUNNER scenario show daily_check > /dev/null 2>&1 && echo_pass "场景详情" || echo_fail "场景详情"
}

# ============================================
# 测试: CASE初始化功能
# ============================================
test_init() {
    echo_section "CASE初始化功能测试"
    
    TEST_DIR="$PROJECT_ROOT/test_output/init_test"
    rm -rf "$TEST_DIR"
    mkdir -p "$TEST_DIR"
    
    # 创建Bash CASE
    $KB_RUNNER init test_bash -o "$TEST_DIR" > /dev/null 2>&1
    if [ -f "$TEST_DIR/test_bash/case.yaml" ] && [ -f "$TEST_DIR/test_bash/run.sh" ]; then
        echo_pass "创建Bash CASE"
    else
        echo_fail "创建Bash CASE"
    fi
    
    # 创建Python CASE
    $KB_RUNNER init test_python -l python -o "$TEST_DIR" > /dev/null 2>&1
    if [ -f "$TEST_DIR/test_python/case.yaml" ] && [ -f "$TEST_DIR/test_python/run.py" ]; then
        echo_pass "创建Python CASE"
    else
        echo_fail "创建Python CASE"
    fi
    
    # 重复创建应失败
    $KB_RUNNER init test_bash -o "$TEST_DIR" > /dev/null 2>&1 && echo_fail "重复创建应失败" || echo_pass "重复创建应失败"
}

# ============================================
# 测试: 脚本执行功能
# ============================================
test_execution() {
    echo_section "脚本执行功能测试"
    
    # 注意: Windows环境下Bash脚本无法执行，这是平台限制
    # 检查是否有bash可用
    
    if command -v bash &> /dev/null; then
        TEST_DIR="$PROJECT_ROOT/test_output/exec_test"
        mkdir -p "$TEST_DIR"
        
        # 创建简单测试脚本
        cat > "$TEST_DIR/simple.sh" << 'EOF'
#!/bin/bash
echo "test output"
exit 0
EOF
        chmod +x "$TEST_DIR/simple.sh"
        
        # 执行测试脚本
        $KB_RUNNER run -s "$TEST_DIR/simple.sh" > /dev/null 2>&1
        if [ $? -eq 0 ]; then
            echo_pass "脚本执行"
        else
            echo_fail "脚本执行"
        fi
        
        # 测试不同输出格式
        $KB_RUNNER run -s "$TEST_DIR/simple.sh" -f json > /dev/null 2>&1 && echo_pass "JSON输出格式" || echo_fail "JSON输出格式"
        $KB_RUNNER run -s "$TEST_DIR/simple.sh" -f table > /dev/null 2>&1 && echo_pass "Table输出格式" || echo_fail "Table输出格式"
    else
        echo_skip "脚本执行 (需要Bash环境)"
        echo_skip "JSON输出格式 (需要Bash环境)"
        echo_skip "Table输出格式 (需要Bash环境)"
    fi
}

# ============================================
# 测试结果汇总
# ============================================
show_summary() {
    echo_section "测试结果汇总"
    echo -e "${GREEN}通过: $PASSED${NC}"
    echo -e "${RED}失败: $FAILED${NC}"
    echo -e "${YELLOW}跳过: $SKIPPED${NC}"
    echo -e "总计: $TOTAL"
    echo ""
    
    if [ $FAILED -gt 0 ]; then
        echo -e "${RED}测试存在失败项${NC}"
        exit 1
    else
        echo -e "${GREEN}所有测试通过${NC}"
        exit 0
    fi
}

# ============================================
# 主函数
# ============================================
main() {
    echo ""
    echo_info "KB脚本执行框架 - 自动化测试"
    echo_info "项目目录: $PROJECT_ROOT"
    echo_info "测试工具: $KB_RUNNER"
    echo ""
    
    check_binary
    
    test_help
    test_case_list
    test_scenario
    test_init
    test_execution
    
    show_summary
}

main "$@"
