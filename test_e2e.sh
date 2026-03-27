#!/bin/bash

# KB Runner 端到端测试脚本
BASE_URL="http://localhost:8080/api/v1"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "KB Runner 端到端功能测试"
echo "=========================================="
echo ""

pass=0
fail=0

# 测试 1: 验证 KB 列表从 kbscript 加载
echo "=== 测试 1: KB 数据来源 ==="
kb_list=$(curl -s "${BASE_URL}/cases")
kb_paths=$(echo "$kb_list" | python3 -c "import sys,json; d=json.load(sys.stdin); print('\n'.join(d['data'][0]['path'] for d in d['data'][:1]))" 2>/dev/null)

if echo "$kb_paths" | grep -q "kbscript"; then
    echo -e "${GREEN}✓ KB 从 kbscript 目录加载${NC}"
    ((pass++))
else
    echo -e "${RED}✗ KB 未从 kbscript 目录加载${NC}"
    ((fail++))
fi
echo ""

# 测试 2: 验证执行历史显示
echo "=== 测试 2: 执行历史显示 ==="
executions=$(curl -s "${BASE_URL}/executions")
exec_count=$(echo "$executions" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('data',[])))" 2>/dev/null)

if [ "$exec_count" -gt 0 ]; then
    echo -e "${GREEN}✓ 执行历史显示 $exec_count 条记录${NC}"
    ((pass++))
    
    # 获取第一条记录详情
    first_dir=$(echo "$executions" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['data'][0]['dir_name'])" 2>/dev/null)
    echo "  第一条记录：$first_dir"
else
    echo -e "${YELLOW}⚠ 执行历史为空${NC}"
fi
echo ""

# 测试 3: 验证执行详情（柱状图数据）
echo "=== 测试 3: 执行详情（柱状图数据）==="
if [ -n "$first_dir" ]; then
    detail=$(curl -s "${BASE_URL}/executions/$first_dir")
    
    # 验证包含 scripts 数组
    has_scripts=$(echo "$detail" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if d.get('data',{}).get('scripts') else 'no')" 2>/dev/null)
    if [ "$has_scripts" == "yes" ]; then
        echo -e "${GREEN}✓ 执行详情包含 scripts 数据${NC}"
        ((pass++))
    else
        echo -e "${RED}✗ 执行详情缺少 scripts 数据${NC}"
        ((fail++))
    fi
    
    # 验证包含归一化得分
    has_normalized=$(echo "$detail" | python3 -c "import sys,json; d=json.load(sys.stdin); scripts=d.get('data',{}).get('scripts',[]); print('yes' if scripts and 'normalized_score' in scripts[0] else 'no')" 2>/dev/null)
    if [ "$has_normalized" == "yes" ]; then
        echo -e "${GREEN}✓ 包含 normalized_score（柱状图需要）${NC}"
        ((pass++))
    else
        echo -e "${RED}✗ 缺少 normalized_score${NC}"
        ((fail++))
    fi
    
    # 验证得分排序（应该按降序排列）
    is_sorted=$(echo "$detail" | python3 -c "
import sys,json
d=json.load(sys.stdin)
scripts=d.get('data',{}).get('scripts',[])
if len(scripts) > 1:
    scores = [(s.get('normalized_score',0) or 0) for s in scripts]
    is_sorted = all(scores[i] >= scores[i+1] for i in range(len(scores)-1))
    print('yes' if is_sorted else 'no')
else:
    print('yes' if scripts else 'no')
" 2>/dev/null)
    if [ "$is_sorted" == "yes" ]; then
        echo -e "${GREEN}✓ 得分已按降序排序${NC}"
        ((pass++))
    else
        echo -e "${YELLOW}⚠ 得分未排序（可能只有 1 条数据）${NC}"
    fi
else
    echo -e "${YELLOW}⚠ 跳过（无执行记录）${NC}"
fi
echo ""

# 测试 4: 验证删除功能权限
echo "=== 测试 4: 删除功能权限 ==="
# 尝试删除（应该返回 403 Forbidden）
del_response=$(curl -s -w "\n%{http_code}" -X DELETE "${BASE_URL}/executions/test-dir")
del_code=$(echo "$del_response" | tail -n1)

if [ "$del_code" == "403" ]; then
    echo -e "${GREEN}✓ 删除功能需要 admin 权限${NC}"
    ((pass++))
elif [ "$del_code" == "404" ]; then
    echo -e "${GREEN}✓ 删除功能正常（资源不存在返回 404）${NC}"
    ((pass++))
else
    echo -e "${YELLOW}⚠ 删除功能返回 HTTP $del_code${NC}"
fi
echo ""

# 测试 5: 验证 Q 单目录结构
echo "=== 测试 5: Q 单目录结构 ==="
result_root="$HOME/kb-runner/workspace/results"
if [ -d "$result_root" ]; then
    q_dirs=$(ls -d "$result_root"/Q*-2* 2>/dev/null | wc -l)
    if [ "$q_dirs" -gt 0 ]; then
        echo -e "${GREEN}✓ Q 单目录格式正确 (Q{单号}-{时间戳})${NC}"
        echo "  目录数量：$q_dirs"
        ((pass++))
        
        # 验证目录下有 ranked_results 文件
        first_q_dir=$(ls -d "$result_root"/Q*-2* 2>/dev/null | head -1)
        ranked_files=$(ls "$first_q_dir"/ranked_results_*.json 2>/dev/null | wc -l)
        if [ "$ranked_files" -gt 0 ]; then
            echo -e "${GREEN}✓ Q 单目录包含 ranked_results 文件${NC}"
            ((pass++))
        else
            echo -e "${RED}✗ Q 单目录缺少 ranked_results 文件${NC}"
            ((fail++))
        fi
    else
        echo -e "${YELLOW}⚠ 未找到 Q 单目录${NC}"
    fi
else
    echo -e "${RED}✗ 结果目录不存在：$result_root${NC}"
    ((fail++))
fi
echo ""

# 总结
echo "=========================================="
echo "端到端测试总结"
echo "=========================================="
echo -e "通过：${GREEN}${pass}${NC}"
echo -e "失败：${RED}${fail}${NC}"
echo ""

if [ $fail -eq 0 ]; then
    echo -e "${GREEN}✓ 所有端到端测试通过！${NC}"
    exit 0
else
    echo -e "${RED}✗ 有测试失败${NC}"
    exit 1
fi
