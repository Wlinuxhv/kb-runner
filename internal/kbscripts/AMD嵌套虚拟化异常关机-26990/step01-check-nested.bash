#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/bash/api.sh"

kb_init

step_start "检查CPU类型和嵌套虚拟化状态"

echo "=== CPU信息 ==="
CPU_INFO=$(cat /proc/cpuinfo 2>/dev/null | grep "model name" | head -1)
echo "$CPU_INFO"
result "CPU" "$CPU_INFO"

if echo "$CPU_INFO" | grep -qi "amd\|hygon\|海光"; then
    result "CPU类型" "AMD/海光平台"
    echo "检测到AMD或海光平台"
else
    result "CPU类型" "非AMD/海光平台"
    echo "非AMD/海光平台，此KB可能不适用"
fi

echo ""
echo "=== 嵌套虚拟化状态 ==="
if [ -f /sys/module/kvm_amd/parameters/nested ]; then
    NESTED=$(cat /sys/module/kvm_amd/parameters/nested 2>/dev/null)
    echo "kvm_amd nested: $NESTED"
    result "嵌套虚拟化" "$NESTED"
    
    if [ "$NESTED" = "Y" ] || [ "$NESTED" = "1" ]; then
        echo "警告：嵌套虚拟化已启用，可能导致虚拟机异常关机"
    fi
else
    echo "kvm_amd nested参数不存在"
    result "嵌套虚拟化" "参数不存在"
fi

echo ""
echo "=== 检查是否有虚拟机开启嵌套虚拟化 ==="
if [ -d /cfs ]; then
    NESTED_VMS=$(grep "nested_virtualization: 1" -rn /cfs/ 2>/dev/null)
    if [ -n "$NESTED_VMS" ]; then
        result "嵌套虚拟机" "存在"
        echo "以下虚拟机开启了嵌套虚拟化："
        echo "$NESTED_VMS"
    else
        result "嵌套虚拟机" "不存在"
        echo "未发现开启嵌套虚拟化的虚拟机"
    fi
else
    result "/cfs目录" "不存在"
fi

echo ""
echo "=== 检查loadmod.conf配置 ==="
if [ -f /sf/modules/loadmod.conf ]; then
    KVM_AMD_LINE=$(grep "kvm_amd" /sf/modules/loadmod.conf 2>/dev/null)
    echo "当前配置: $KVM_AMD_LINE"
    result "loadmod.conf" "$KVM_AMD_LINE"
fi

step_success "检查完成"

kb_save