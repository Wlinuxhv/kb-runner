#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
系统性能检查脚本
"""

import os
import sys

try:
    sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
except NameError:
    pass

from kb_api import kb_init, kb_save, step_start, step_success, step_warning, step_failure, step_skip, result

def main():
    kb_init()
    
    step_start("check_cpu")
    try:
        if os.path.exists("/proc/loadavg"):
            with open("/proc/loadavg", "r") as f:
                loadavg = f.read().split()[:3]
            result("load_1m", loadavg[0])
            result("load_5m", loadavg[1])
            result("load_15m", loadavg[2])
            
            load_1m = float(loadavg[0])
            if load_1m < 2.0:
                step_success(f"CPU负载正常: 1分钟负载={load_1m}")
            elif load_1m < 4.0:
                step_warning(f"CPU负载较高: 1分钟负载={load_1m}")
            else:
                step_failure(f"CPU负载过高: 1分钟负载={load_1m}")
        else:
            step_skip("无法获取CPU负载信息")
    except Exception as e:
        step_warning(f"CPU检查异常: {str(e)}")
    
    step_start("check_memory")
    try:
        if os.path.exists("/proc/meminfo"):
            meminfo = {}
            with open("/proc/meminfo", "r") as f:
                for line in f:
                    parts = line.split()
                    if len(parts) >= 2:
                        key = parts[0].rstrip(":")
                        value = int(parts[1])
                        meminfo[key] = value
            
            total = meminfo.get("MemTotal", 0)
            available = meminfo.get("MemAvailable", meminfo.get("MemFree", 0))
            
            if total > 0:
                usage_percent = (1 - available / total) * 100
                result("memory_total_kb", total)
                result("memory_available_kb", available)
                result("memory_usage_percent", f"{usage_percent:.1f}")
                
                if usage_percent < 70:
                    step_success(f"内存使用率正常: {usage_percent:.1f}%")
                elif usage_percent < 90:
                    step_warning(f"内存使用率较高: {usage_percent:.1f}%")
                else:
                    step_failure(f"内存使用率过高: {usage_percent:.1f}%")
            else:
                step_skip("无法计算内存使用率")
        else:
            step_skip("无法获取内存信息")
    except Exception as e:
        step_warning(f"内存检查异常: {str(e)}")
    
    step_start("check_disk")
    try:
        import shutil
        total, used, free = shutil.disk_usage("/")
        
        usage_percent = (used / total) * 100
        result("disk_total_gb", f"{total / (1024**3):.1f}")
        result("disk_used_gb", f"{used / (1024**3):.1f}")
        result("disk_usage_percent", f"{usage_percent:.1f}")
        
        if usage_percent < 80:
            step_success(f"磁盘使用率正常: {usage_percent:.1f}%")
        elif usage_percent < 90:
            step_warning(f"磁盘使用率较高: {usage_percent:.1f}%")
        else:
            step_failure(f"磁盘使用率过高: {usage_percent:.1f}%")
    except Exception as e:
        step_warning(f"磁盘检查异常: {str(e)}")
    
    kb_save()

if __name__ == "__main__":
    main()
else:
    main()