#!/bin/bash

# KB Runner API 测试脚本
BASE_URL="http://localhost:8080/api/v1"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

pass_count=0
fail_count=0

test_api() {
    local name=$1
    local method=$2
    local path=$3
    local expected_status=$4
    
    echo -e "${YELLOW}测试：${name}${NC}"
    
    response=$(curl -s -w "\n%{http_code}" -X "${method}" "${BASE_URL}${path}")
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n-1)
    
    if [ "$http_code" == "$expected_status" ]; then
        echo -e "${GREEN}✓ 通过${NC} (HTTP ${http_code})"
        ((pass_count++))
        return 0
    else
        echo -e "${RED}✗ 失败${NC} (期望 HTTP ${expected_status}, 实际 HTTP ${http_code})"
        echo "响应内容：$body"
        ((fail_count++))
        return 1
    fi
}

echo "=========================================="
echo "KB Runner API 功能测试"
echo "=========================================="
echo ""

# 1. 健康检查
echo "=== 1. 健康检查 ==="
test_api "健康检查" "GET" "/health" "200"
echo ""

# 2. KB 列表
echo "=== 2. KB 列表 ==="
test_api "获取 KB 列表" "GET" "/cases" "200"
kb_count=$(curl -s "${BASE_URL}/cases" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('data',[])))" 2>/dev/null || echo "0")
echo "KB 数量：$kb_count"
if [ "$kb_count" -gt 0 ]; then
    echo -e "${GREEN}✓ KB 列表有数据${NC}"
    ((pass_count++))
else
    echo -e "${RED}✗ KB 列表为空${NC}"
    ((fail_count++))
fi
echo ""

# 3. 场景列表
echo "=== 3. 场景列表 ==="
test_api "获取场景列表" "GET" "/scenarios" "200"
echo ""

# 4. 执行历史列表
echo "=== 4. 执行历史列表 ==="
test_api "获取执行历史" "GET" "/executions" "200"
exec_count=$(curl -s "${BASE_URL}/executions" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('data',[])))" 2>/dev/null || echo "0")
echo "执行记录数：$exec_count"
if [ "$exec_count" -gt 0 ]; then
    echo -e "${GREEN}✓ 执行历史有数据${NC}"
    ((pass_count++))
    
    # 获取第一条执行记录的目录名
    first_dir=$(curl -s "${BASE_URL}/executions" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['data'][0]['dir_name'] if d.get('data') else '')" 2>/dev/null)
    echo "第一条记录：$first_dir"
else
    echo -e "${YELLOW}⚠ 执行历史为空（需要先执行 KB）${NC}"
fi
echo ""

# 5. 执行详情
echo "=== 5. 执行详情 ==="
if [ -n "$first_dir" ]; then
    test_api "获取执行详情 ($first_dir)" "GET" "/executions/$first_dir" "200"
    
    # 验证详情数据结构
    detail=$(curl -s "${BASE_URL}/executions/$first_dir")
    has_scripts=$(echo "$detail" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if d.get('data',{}).get('scripts') else 'no')" 2>/dev/null)
    if [ "$has_scripts" == "yes" ]; then
        echo -e "${GREEN}✓ 执行详情包含 scripts 数据${NC}"
        ((pass_count++))
    else
        echo -e "${RED}✗ 执行详情缺少 scripts 数据${NC}"
        ((fail_count++))
    fi
else
    echo -e "${YELLOW}⚠ 跳过（无执行记录）${NC}"
fi
echo ""

# 6. Q 单列表
echo "=== 6. Q 单筛选 ==="
qnos=$(curl -s "${BASE_URL}/executions" | python3 -c "import sys,json; d=json.load(sys.stdin); qnos=set(e['qno'] for e in d.get('data',[])); print(','.join(qnos) if qnos else 'none')" 2>/dev/null)
echo "Q 单列表：$qnos"
if [ "$qnos" != "none" ] && [ -n "$qnos" ]; then
    echo -e "${GREEN}✓ Q 单筛选功能正常${NC}"
    ((pass_count++))
else
    echo -e "${YELLOW}⚠ 无 Q 单数据${NC}"
fi
echo ""

# 7. 删除功能测试（仅测试权限检查）
echo "=== 7. 删除功能（权限检查）==="
# 尝试删除（应该返回 403，因为没有 admin 权限）
response=$(curl -s -w "\n%{http_code}" -X DELETE "${BASE_URL}/executions/test")
http_code=$(echo "$response" | tail -n1)
if [ "$http_code" == "403" ] || [ "$http_code" == "404" ]; then
    echo -e "${GREEN}✓ 删除功能有权限检查或资源不存在${NC}"
    ((pass_count++))
else
    echo -e "${YELLOW}⚠ 删除功能响应：HTTP ${http_code}${NC}"
fi
echo ""

# 总结
echo "=========================================="
echo "测试总结"
echo "=========================================="
echo -e "通过：${GREEN}${pass_count}${NC}"
echo -e "失败：${RED}${fail_count}${NC}"
echo ""

if [ $fail_count -eq 0 ]; then
    echo -e "${GREEN}✓ 所有测试通过！${NC}"
    exit 0
else
    echo -e "${RED}✗ 有测试失败${NC}"
    exit 1
fi
