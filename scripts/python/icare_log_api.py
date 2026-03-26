#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
ICare日志适配器 - Python API (简化版)
技术支持人员只需提供Q单号，其他自动处理
"""

import os
import re
import json
import subprocess
from typing import List, Dict, Optional


class IcareLogAdapter:
    """ICare日志适配器 - 简化版"""

    ROOT_PATH = "/sf/data/icare_log/logall"

    def __init__(self, root_path: str = None):
        """初始化适配器"""
        self.root_path = root_path or self.ROOT_PATH
        self.qno = ""
        self.yearmonth = ""  # 从Q单号自动解析：2603
        self.host = ""
        self.hosts = []
        self.extracted = ""

    def init(self, qno: str) -> bool:
        """
        初始化适配器（只需Q单号）

        Args:
            qno: Q单号，如 Q2026031700424

        Returns:
            bool: 初始化是否成功
        """
        if not qno:
            print("Error: Q单号不能为空")
            return False

        # 验证Q单号格式
        if not re.match(r'^Q\d{12}$', qno):
            print(f"Error: Q单号格式错误，应为Q+12位数字，如Q2026031700424")
            return False

        self.qno = qno

        # 从Q单号解析年月：Q2026031700424 -> 2603
        # Q单号格式：Q + 年(4位) + 月(2位) + 序号(6位)
        year = qno[1:5]   # 2026
        month = qno[5:7]  # 03
        self.yearmonth = f"{year[2:]}{month}"  # 2603

        # 自动查找主机
        self._find_hosts()

        # 自动解压（如需要）
        self._extract_if_needed()

        return True

    def _parse_yearmonth(self) -> str:
        """解析年月份"""
        if not self.qno:
            return ""
        year = self.qno[1:5]
        month = self.qno[5:7]
        return f"{year}-{month}"

    def _find_hosts(self):
        """自动查找主机列表"""
        qno_dir = os.path.join(self.root_path, self.yearmonth, self.qno)

        # 如果目录不存在，尝试在其他年月份中查找
        if not os.path.isdir(qno_dir):
            # 搜索所有可能的年月份目录
            try:
                for ym_dir in os.listdir(self.root_path):
                    test_dir = os.path.join(self.root_path, ym_dir, self.qno)
                    if os.path.isdir(test_dir):
                        qno_dir = test_dir
                        self.yearmonth = ym_dir
                        break
            except OSError:
                pass

        self.hosts = []

        if not os.path.isdir(qno_dir):
            return

        # 列出主机目录（排除特殊文件）
        excluded = {'_members', 'cfgmaster_ini', 'collect_record.txt', 'log_list.json'}
        for item in os.listdir(qno_dir):
            full_path = os.path.join(qno_dir, item)
            if not os.path.isdir(full_path):
                continue
            if item in excluded or item.endswith('.zip'):
                continue
            self.hosts.append(item)

        self.hosts.sort()

        # 设置第一个主机为当前主机
        if self.hosts:
            self.host = self.hosts[0]

    def _extract_if_needed(self):
        """解压ZIP文件（如存在）"""
        qno_dir = os.path.join(self.root_path, self.yearmonth, self.qno)

        if not os.path.isdir(qno_dir):
            return

        # 查找ZIP文件
        zip_file = None
        for f in os.listdir(qno_dir):
            if f.endswith('.zip'):
                zip_file = os.path.join(qno_dir, f)
                break

        if not zip_file:
            return

        # 检查是否已解压
        extract_dir = zip_file[:-4]  # 移除.zip
        if os.path.isdir(extract_dir):
            self.extracted = extract_dir
            return

        # 解压ZIP文件
        print(f"正在解压: {os.path.basename(zip_file)}")
        try:
            result = subprocess.run(
                ['unzip', '-o', zip_file, '-d', qno_dir],
                capture_output=True,
                timeout=60
            )
            if os.path.isdir(extract_dir):
                self.extracted = extract_dir
                print("解压完成")
        except Exception as e:
            print(f"Warning: 解压失败 - {e}")

    # ========== 获取信息 ==========

    def get_qno(self) -> str:
        """获取Q单号"""
        return self.qno

    def get_yearmonth(self) -> str:
        """获取年月份"""
        return self.yearmonth

    def get_yearmonth_formatted(self) -> str:
        """获取格式化的年月份"""
        return self._parse_yearmonth()

    def list_hosts(self) -> List[str]:
        """获取主机列表"""
        return self.hosts

    def set_host(self, host: str) -> bool:
        """设置当前主机"""
        if host in self.hosts:
            self.host = host
            return True
        print(f"Error: 主机不存在 - {host}")
        return False

    def get_log_path(self) -> str:
        """获取日志目录路径"""
        if not self.qno or not self.host:
            return ""
        return os.path.join(self.root_path, self.yearmonth, self.qno, self.host)

    def exists(self) -> bool:
        """检查日志目录是否存在"""
        return bool(self.get_log_path()) and os.path.isdir(self.get_log_path())

    # ========== 文件操作 ==========

    def list(self) -> List[Dict]:
        """获取日志文件列表"""
        log_path = self.get_log_path()
        if not os.path.isdir(log_path):
            return []

        files = []
        for root, dirs, filenames in os.walk(log_path):
            for filename in filenames:
                filepath = os.path.join(root, filename)
                relpath = os.path.relpath(filepath, log_path)
                try:
                    stat = os.stat(filepath)
                    files.append({
                        "path": relpath,
                        "name": filename,
                        "size": stat.st_size,
                        "mtime": int(stat.st_mtime)
                    })
                except OSError:
                    continue

        return files

    def count(self) -> int:
        """获取文件数量"""
        return len(self.list())

    def read(self, filepath: str) -> str:
        """读取日志文件内容"""
        fullpath = os.path.join(self.get_log_path(), filepath)

        if not os.path.isfile(fullpath):
            return json.dumps({"error": f"file not found: {filepath}"})

        try:
            with open(fullpath, 'r', encoding='utf-8', errors='ignore') as f:
                return f.read()
        except Exception as e:
            return json.dumps({"error": str(e)})

    def tail(self, filepath: str, lines: int = 100) -> str:
        """读取日志文件最后N行"""
        fullpath = os.path.join(self.get_log_path(), filepath)

        if not os.path.isfile(fullpath):
            return json.dumps({"error": f"file not found: {filepath}"})

        try:
            with open(fullpath, 'r', encoding='utf-8', errors='ignore') as f:
                all_lines = f.readlines()
                return ''.join(all_lines[-lines:])
        except Exception as e:
            return json.dumps({"error": str(e)})

    def search(self, keyword: str, max_results: int = 1000) -> List[Dict]:
        """搜索日志关键字"""
        log_path = self.get_log_path()
        if not os.path.isdir(log_path):
            return []

        results = []
        log_extensions = ('.log', '.txt', '.info')

        for root, dirs, filenames in os.walk(log_path):
            for filename in filenames:
                if not filename.endswith(log_extensions):
                    continue

                filepath = os.path.join(root, filename)
                relpath = os.path.relpath(filepath, log_path)

                try:
                    with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
                        for line_num, line in enumerate(f, 1):
                            if keyword in line:
                                results.append({
                                    "file": relpath,
                                    "line": line_num,
                                    "content": line.strip()
                                })
                                if len(results) >= max_results:
                                    return results
                except Exception:
                    continue

        return results

    def count_keyword(self, keyword: str) -> int:
        """统计关键字出现次数"""
        return len(self.search(keyword, max_results=10000))

    # ========== 状态和帮助 ==========

    def status(self) -> Dict:
        """获取适配器状态"""
        return {
            "qno": self.qno,
            "yearmonth": self.yearmonth,
            "yearmonth_formatted": self._parse_yearmonth(),
            "log_path": self.get_log_path(),
            "host_count": len(self.hosts),
            "hosts": self.hosts,
            "current_host": self.host,
            "extracted": self.extracted
        }

    def print_status(self):
        """打印状态信息"""
        print("=" * 40)
        print("  ICare日志适配器状态")
        print("=" * 40)
        print(f"  Q单号:     {self.qno}")
        print(f"  年月份:    {self.yearmonth} ({self._parse_yearmonth()})")
        print(f"  日志路径:  {self.get_log_path()}")
        print(f"  主机数量:  {len(self.hosts)}")
        print(f"  当前主机:  {self.host}")
        print("=" * 40)

        if len(self.hosts) > 1:
            print("可用主机:")
            for h in self.hosts:
                print(f"  - {h}")
            print("\n切换主机: adapter.set_host('<主机名>')")

    def help(self):
        """打印帮助信息"""
        print("""
ICare日志适配器 - 简化版API

使用方法：
    from icare_log import IcareLogAdapter

    adapter = IcareLogAdapter()
    adapter.init("Q2026031700424")          # 只需Q单号

    # 或指定主机
    adapter.init("Q2026031700424", "sp_10.250.0.7")

API函数：
    adapter.init(qno)               # 初始化（只需Q单号）
    adapter.set_host(host)          # 切换主机
    adapter.list_hosts()            # 列出所有主机
    adapter.list()                  # 获取日志文件列表
    adapter.read(path)              # 读取日志文件
    adapter.tail(path, n)           # 读取最后N行
    adapter.search(word)            # 搜索关键字
    adapter.count_keyword(word)     # 统计关键字次数
    adapter.status()                # 获取状态
    adapter.print_status()          # 打印状态

全局函数：
    icare(qno)                      # 一键初始化
    icare(qno, host)                # 指定主机初始化
""")


# ========== 全局便捷函数 ==========

# 全局适配器实例
_adapter = IcareLogAdapter()


def init(qno: str) -> bool:
    """初始化全局适配器（只需Q单号）"""
    return _adapter.init(qno)


def set_host(host: str) -> bool:
    """设置当前主机"""
    return _adapter.set_host(host)


def list_hosts() -> List[str]:
    """获取主机列表"""
    return _adapter.list_hosts()


def list() -> List[Dict]:
    """获取日志文件列表"""
    return _adapter.list()


def read(filepath: str) -> str:
    """读取日志文件内容"""
    return _adapter.read(filepath)


def tail(filepath: str, lines: int = 100) -> str:
    """读取日志文件最后N行"""
    return _adapter.tail(filepath, lines)


def search(keyword: str) -> List[Dict]:
    """搜索日志关键字"""
    return _adapter.search(keyword)


def count_keyword(keyword: str) -> int:
    """统计关键字出现次数"""
    return _adapter.count_keyword(keyword)


def status() -> Dict:
    """获取状态"""
    return _adapter.status()


def print_status():
    """打印状态"""
    _adapter.print_status()


def help():
    """打印帮助"""
    _adapter.help()


# 一键初始化函数
def icare(qno: str, host: str = None) -> IcareLogAdapter:
    """
    一键初始化适配器

    Args:
        qno: Q单号
        host: 主机名（可选）

    Returns:
        IcareLogAdapter: 适配器实例
    """
    _adapter.init(qno)
    if host:
        _adapter.set_host(host)
    _adapter.print_status()
    return _adapter


# ========== 主函数 ==========

if __name__ == "__main__":
    import sys

    if len(sys.argv) < 2:
        help()
        sys.exit(1)

    qno = sys.argv[1]
    host = sys.argv[2] if len(sys.argv) > 2 else None

    adapter = icare(qno, host)
    print("\n日志文件列表:")
    print(json.dumps(adapter.list()[:10], indent=2))  # 只显示前10个
