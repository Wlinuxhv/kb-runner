#!/bin/bash

# KB Runner 前端功能测试脚本
BASE_URL="http://localhost:8080"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "KB Runner 前端功能测试"
echo "=========================================="
echo ""

# 1. 测试首页加载
echo "=== 1. 首页加载 ==="
response=$(curl -s -w "\n%{http_code}" "${BASE_URL}/")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)

if [ "$http_code" == "200" ]; then
    echo -e "${GREEN}✓ 首页加载成功${NC}"
    
    # 检查是否包含 KB 列表
    if echo "$body" | grep -q "KB 列表"; then
        echo -e "${GREEN}✓ 页面包含'KB 列表'导航${NC}"
    else
        echo -e "${RED}✗ 页面缺少'KB 列表'导航${NC}"
    fi
    
    # 检查是否包含执行历史
    if echo "$body" | grep -q "执行历史"; then
        echo -e "${GREEN}✓ 页面包含'执行历史'导航${NC}"
    else
        echo -e "${RED}✗ 页面缺少'执行历史'导航${NC}"
    fi
    
    # 检查是否加载了 JS 文件
    if echo "$body" | grep -q "app.js"; then
        echo -e "${GREEN}✓ 页面引用了 app.js${NC}"
    else
        echo -e "${RED}✗ 页面缺少 app.js 引用${NC}"
    fi
    
    # 检查是否加载了 CSS 文件
    if echo "$body" | grep -q "style.css"; then
        echo -e "${GREEN}✓ 页面引用了 style.css${NC}"
    else
        echo -e "${RED}✗ 页面缺少 style.css 引用${NC}"
    fi
else
    echo -e "${RED}✗ 首页加载失败 (HTTP ${http_code})${NC}"
fi
echo ""

# 2. 测试 JS 文件加载
echo "=== 2. JS 文件加载 ==="
response=$(curl -s -w "\n%{http_code}" "${BASE_URL}/js/app.js")
http_code=$(echo "$response" | tail -n1)
js_size=$(echo "$response" | head -n-1 | wc -c)

if [ "$http_code" == "200" ] && [ "$js_size" -gt 1000 ]; then
    echo -e "${GREEN}✓ JS 文件加载成功 (${js_size} bytes)${NC}"
    
    # 检查是否包含关键函数
    js_content=$(curl -s "${BASE_URL}/js/app.js")
    if echo "$js_content" | grep -q "loadKBs"; then
        echo -e "${GREEN}✓ JS 包含 loadKBs 函数${NC}"
    else
        echo -e "${RED}✗ JS 缺少 loadKBs 函数${NC}"
    fi
    
    if echo "$js_content" | grep -q "showExecutionDetail"; then
        echo -e "${GREEN}✓ JS 包含 showExecutionDetail 函数${NC}"
    else
        echo -e "${RED}✗ JS 缺少 showExecutionDetail 函数${NC}"
    fi
    
    if echo "$js_content" | grep -q "renderExecutions"; then
        echo -e "${GREEN}✓ JS 包含 renderExecutions 函数${NC}"
    else
        echo -e "${RED}✗ JS 缺少 renderExecutions 函数${NC}"
    fi
else
    echo -e "${RED}✗ JS 文件加载失败或太小${NC}"
fi
echo ""

# 3. 测试 CSS 文件加载
echo "=== 3. CSS 文件加载 ==="
response=$(curl -s -w "\n%{http_code}" "${BASE_URL}/css/style.css")
http_code=$(echo "$response" | tail -n1)
css_size=$(echo "$response" | head -n-1 | wc -c)

if [ "$http_code" == "200" ] && [ "$css_size" -gt 1000 ]; then
    echo -e "${GREEN}✓ CSS 文件加载成功 (${css_size} bytes)${NC}"
else
    echo -e "${RED}✗ CSS 文件加载失败或太小${NC}"
fi
echo ""

# 4. 测试缓存控制头
echo "=== 4. 缓存控制头 ==="
headers=$(curl -s -I "${BASE_URL}/")
if echo "$headers" | grep -qi "Cache-Control:.*no-cache"; then
    echo -e "${GREEN}✓ Cache-Control 头正确${NC}"
else
    echo -e "${RED}✗ Cache-Control 头缺失或不正确${NC}"
fi

if echo "$headers" | grep -qi "Pragma:.*no-cache"; then
    echo -e "${GREEN}✓ Pragma 头正确${NC}"
else
    echo -e "${RED}✗ Pragma 头缺失${NC}"
fi

if echo "$headers" | grep -qi "Expires:.*0"; then
    echo -e "${GREEN}✓ Expires 头正确${NC}"
else
    echo -e "${RED}✗ Expires 头缺失${NC}"
fi
echo ""

# 5. 测试 API 数据完整性
echo "=== 5. API 数据完整性 ==="
executions=$(curl -s "${BASE_URL}/api/v1/executions")

# 检查数据结构
if echo "$executions" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'success' in d and 'data' in d" 2>/dev/null; then
    echo -e "${GREEN}✓ 执行历史 API 返回正确结构${NC}"
else
    echo -e "${RED}✗ 执行历史 API 返回结构错误${NC}"
fi

# 检查字段完整性
if echo "$executions" | python3 -c "
import sys,json
d=json.load(sys.stdin)
if d.get('data'):
    e = d['data'][0]
    assert 'dir_name' in e
    assert 'qno' in e
    assert 'exec_id' in e
    assert 'script_count' in e
    assert 'summary' in e
    print('ok')
" 2>/dev/null; then
    echo -e "${GREEN}✓ 执行记录字段完整${NC}"
else
    echo -e "${RED}✗ 执行记录字段缺失${NC}"
fi
echo ""

echo "=========================================="
echo "前端测试完成"
echo "=========================================="
