#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
智能日志适配器 - 简化版 Python API
只需Q单号，其他自动处理
"""

import os
import re
import sqlite3
import subprocess
import glob
from typing import List, Dict, Optional


class SmartLog:
    """智能日志适配器 - 简化版"""

    ROOT_PATH = "/sf/data/icare_log/logall"

    def __init__(self, root_path: str = None):
        self.root_path = root_path or self.ROOT_PATH
        self.qno = ""
        self.yearmonth = ""
        self.host = ""
        self.log_dir = ""

    def init(self, qno: str) -> bool:
        """初始化（只需Q单号）"""
        if not qno or not re.match(r'^Q\d{13}$', qno):
            print(f"Error: Q单号格式错误，应为Q+13位数字")
            return False

        self.qno = qno
        # 解析年月份
        self.yearmonth = f"{qno[3:5]}{qno[5:7]}"

        # 查找Q单号目录
        qno_dir = os.path.join(self.root_path, self.yearmonth, qno)

        # 如果目录不存在，尝试在其他年月份中查找
        if not os.path.isdir(qno_dir):
            for ym_dir in os.listdir(self.root_path):
                test_dir = os.path.join(self.root_path, ym_dir, qno)
                if os.path.isdir(test_dir):
                    qno_dir = test_dir
                    self.yearmonth = ym_dir
                    break

        if not os.path.isdir(qno_dir):
            print(f"Error: Q单号目录不存在")
            return False

        # 检查是否有tgz文件需要解压
        self._extract_tgz(qno_dir)

        # 查找主机 - 可能直接在Q单号目录下，也可能在子目录中
        excluded = {'_members', 'cfgmaster_ini', 'log_list.json'}
        hosts = [d for d in os.listdir(qno_dir)
                if os.path.isdir(os.path.join(qno_dir, d)) and d not in excluded]
        hosts.sort()

        # 如果没有找到主机目录，尝试查找 sf 子目录（tgz解压后的结构）
        if not hosts:
            sf_dir = os.path.join(qno_dir, "sf")
            if os.path.isdir(sf_dir):
                # sf 目录下有 log 子目录
                self.host = "sf"
                self.log_dir = os.path.join(qno_dir, "sf", "log")
                # 自动解压
                self._extract()
                return True
            print(f"Error: 未找到主机目录")
            return False

        self.host = hosts[0]

        # 检查log目录位置：优先 {host}/log，其次 {host}/sf/log
        standard_log = os.path.join(qno_dir, self.host, "log")
        sf_log = os.path.join(qno_dir, self.host, "sf", "log")

        if os.path.isdir(standard_log):
            self.log_dir = standard_log
        elif os.path.isdir(sf_log):
            self.log_dir = sf_log
        else:
            # 如果都没有，使用 {host}/log 作为默认值（可能是空目录）
            self.log_dir = standard_log

        # 自动解压
        self._extract()

        return True

    def _extract_tgz(self, qno_dir: str):
        """解压Q单号目录下的tgz文件"""
        for tgz_file in glob.glob(f"{qno_dir}/*.tgz"):
            tgz_name = os.path.basename(tgz_file).replace(".tgz", "")
            extract_dir = os.path.join(qno_dir, tgz_name)

            if not os.path.isdir(extract_dir):
                print(f"[自动解压] {os.path.basename(tgz_file)}")
                try:
                    result = subprocess.run(["tar", "-xzf", tgz_file, "-C", qno_dir],
                                     capture_output=True, timeout=300)
                    if result.returncode != 0:
                        print(f"[错误] 解压失败: {os.path.basename(tgz_file)}")
                except Exception as e:
                    print(f"[错误] 解压失败: {e}")

    def _extract(self):
        """自动解压"""
        # 检查是否有 zstd 命令
        has_zstd = self._check_command("zstd")

        # 解压tar.gz (根目录)
        for tar_file in glob.glob(f"{self.log_dir}/*.tar.gz"):
            extract_dir = tar_file.replace(".tar.gz", "")
            if not os.path.isdir(extract_dir):
                print(f"[自动解压] {os.path.basename(tar_file)}")
                try:
                    subprocess.run(["tar", "-xzf", tar_file, "-C", os.path.dirname(tar_file)],
                                 capture_output=True, timeout=300)
                except:
                    pass

        # 解压tar.zst (根目录)
        if has_zstd:
            for tar_file in glob.glob(f"{self.log_dir}/*.tar.zst"):
                extract_dir = tar_file.replace(".tar.zst", "")
                if not os.path.isdir(extract_dir):
                    print(f"[自动解压] {os.path.basename(tar_file)}")
                    try:
                        # 先解压zstd，再用tar解压
                        subprocess.run(["tar", "-I", "zstd", "-xf", tar_file, "-C", os.path.dirname(tar_file)],
                                     capture_output=True, timeout=300)
                    except:
                        pass

        # 解压zip (根目录)
        for zip_file in glob.glob(f"{self.log_dir}/*.zip"):
            extract_dir = zip_file.replace(".zip", "")
            if not os.path.isdir(extract_dir):
                print(f"[自动解压] {os.path.basename(zip_file)}")
                try:
                    subprocess.run(["unzip", "-o", zip_file, "-d", os.path.dirname(zip_file)],
                                 capture_output=True, timeout=300)
                except:
                    pass

        # 递归解压子目录中的压缩文件 (如 blackbox/20260307/LOG_*.txt.zip)
        self._extract_subdirs()

    def _check_command(self, cmd: str) -> bool:
        """检查命令是否存在"""
        try:
            subprocess.run([cmd, "--version"], capture_output=True, timeout=5)
            return True
        except:
            return False

    def _extract_subdirs(self):
        """递归解压子目录中的压缩文件"""
        # 递归查找所有zip文件
        for root, dirs, files in os.walk(self.log_dir):
            for f in files:
                if f.endswith('.zip'):
                    zip_path = os.path.join(root, f)
                    extract_dir = zip_path.replace(".zip", "")
                    if not os.path.isdir(extract_dir):
                        print(f"[自动解压] {os.path.basename(zip_path)}")
                        try:
                            subprocess.run(["unzip", "-o", zip_path, "-d", os.path.dirname(zip_path)],
                                         capture_output=True, timeout=300)
                        except:
                            pass

        # 递归查找tar.gz文件
        for root, dirs, files in os.walk(self.log_dir):
            for f in files:
                if f.endswith('.tar.gz'):
                    tar_path = os.path.join(root, f)
                    extract_dir = tar_path.replace(".tar.gz", "")
                    if not os.path.isdir(extract_dir):
                        print(f"[自动解压] {os.path.basename(tar_path)}")
                        try:
                            subprocess.run(["tar", "-xzf", tar_path, "-C", os.path.dirname(tar_path)],
                                         capture_output=True, timeout=300)
                        except:
                            pass

    # ========== 核心属性 ==========

    @property
    def qno(self) -> str:
        return self._qno

    @qno.setter
    def qno(self, value):
        self._qno = value

    @property
    def yearmonth(self) -> str:
        return self._yearmonth

    @yearmonth.setter
    def yearmonth(self, value):
        self._yearmonth = value

    @property
    def host(self) -> str:
        return self._host

    @host.setter
    def host(self, value):
        self._host = value

    @property
    def log_dir(self) -> str:
        return self._log_dir

    @log_dir.setter
    def log_dir(self, value):
        self._log_dir = value

    # ========== 查询函数 ==========

    def dir(self, subdir: str = "") -> str:
        """获取日志目录"""
        if subdir:
            return os.path.join(self.log_dir, subdir)
        return self.log_dir

    def ls(self, subdir: str = "") -> List[str]:
        """列出文件"""
        d = self.dir(subdir)
        if os.path.isdir(d):
            return sorted(os.listdir(d))
        return []

    def grep(self, keyword: str, subdir: str = "blackbox") -> List[Dict]:
        """搜索日志文件"""
        results = []
        search_dir = self.dir(subdir)

        if not os.path.isdir(search_dir):
            return results

        for root, dirs, files in os.walk(search_dir):
            for f in files:
                if f.endswith(('.txt', '.log')):
                    filepath = os.path.join(root, f)
                    relpath = os.path.relpath(filepath, self.log_dir)
                    try:
                        with open(filepath, 'r', errors='ignore') as fp:
                            for line_num, line in enumerate(fp, 1):
                                if keyword in line:
                                    results.append({
                                        "file": relpath,
                                        "line": line_num,
                                        "content": line.strip()[:100]
                                    })
                    except:
                        continue
        return results[:50]

    def cat(self, filepath: str) -> str:
        """读取文件"""
        fullpath = os.path.join(self.log_dir, filepath)
        if os.path.isfile(fullpath):
            with open(fullpath, 'r', errors='ignore') as f:
                return f.read()
        return f"文件不存在: {filepath}"

    def tail(self, filepath: str, lines: int = 100) -> str:
        """读取文件尾部"""
        fullpath = os.path.join(self.log_dir, filepath)
        if os.path.isfile(fullpath):
            with open(fullpath, 'r', errors='ignore') as f:
                all_lines = f.readlines()
                return ''.join(all_lines[-lines:])
        return f"文件不存在: {filepath}"

    # ========== 数据库函数 ==========

    def _db_path(self) -> str:
        return os.path.join(self.log_dir, "log_new.db")

    def sql(self, query: str) -> List[Dict]:
        """执行SQL查询"""
        db_path = self._db_path()
        if not os.path.isfile(db_path):
            return [{"error": "数据库不存在"}]

        try:
            conn = sqlite3.connect(db_path)
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()
            cursor.execute(query)

            results = []
            for row in cursor.fetchall():
                results.append(dict(row))
            conn.close()
            return results
        except Exception as e:
            return [{"error": str(e)}]

    def alerts(self, level: str = "", limit: int = 100) -> List[Dict]:
        """查询告警"""
        sql = "SELECT id, type, host, object_name, description, start, level FROM alert"

        conditions = []
        if level:
            conditions.append(f"level='{level}'")

        if conditions:
            sql += " WHERE " + " AND ".join(conditions)

        sql += f" ORDER BY start DESC LIMIT {limit}"
        return self.sql(sql)

    def logs(self, user: str = "", limit: int = 100) -> List[Dict]:
        """查询操作日志"""
        sql = "SELECT id, type, host, user, description, start, end, status FROM log"

        conditions = []
        if user:
            conditions.append(f"user='{user}'")

        if conditions:
            sql += " WHERE " + " AND ".join(conditions)

        sql += f" ORDER BY start DESC LIMIT {limit}"
        return self.sql(sql)

    def search(self, keyword: str, limit: int = 50) -> Dict:
        """搜索告警和操作日志"""
        db_path = self._db_path()
        if not os.path.isfile(db_path):
            return {"error": "数据库不存在"}

        try:
            conn = sqlite3.connect(db_path)
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()

            # 搜索告警
            cursor.execute("""
                SELECT 'alert' as src, id, object_name, description, start, level
                FROM alert
                WHERE description LIKE ? OR object_name LIKE ?
                LIMIT ?
            """, (f'%{keyword}%', f'%{keyword}%', limit))
            alert_results = [dict(row) for row in cursor.fetchall()]

            # 搜索日志
            cursor.execute("""
                SELECT 'log' as src, id, user, description, start, status
                FROM log
                WHERE description LIKE ? OR user LIKE ?
                LIMIT ?
            """, (f'%{keyword}%', f'%{keyword}%', limit))
            log_results = [dict(row) for row in cursor.fetchall()]

            conn.close()
            return {"alerts": alert_results, "logs": log_results}

        except Exception as e:
            return {"error": str(e)}

    def stats(self) -> Dict:
        """统计信息"""
        db_path = self._db_path()
        if not os.path.isfile(db_path):
            return {"error": "数据库不存在"}

        try:
            conn = sqlite3.connect(db_path)
            cursor = conn.cursor()

            # 总数
            cursor.execute("SELECT COUNT(*) FROM alert")
            alert_count = cursor.fetchone()[0]

            cursor.execute("SELECT COUNT(*) FROM log")
            log_count = cursor.fetchone()[0]

            # 告警级别分布
            cursor.execute("SELECT level, COUNT(*) as cnt FROM alert GROUP BY level ORDER BY cnt DESC")
            level_dist = [{"level": row[0], "count": row[1]} for row in cursor.fetchall()]

            conn.close()

            return {
                "alert_count": alert_count,
                "log_count": log_count,
                "level_distribution": level_dist
            }

        except Exception as e:
            return {"error": str(e)}

    # ========== 便捷函数 ==========

    def info(self):
        """显示状态信息"""
        print("=" * 50)
        print("  智能日志适配器")
        print("=" * 50)
        print(f"  Q单号:    {self.qno}")
        print(f"  年月份:   {self.yearmonth}")
        print(f"  主机:     {self.host}")
        print(f"  日志目录: {self.log_dir}")
        print("=" * 50)

        # 显示日志类型
        print("\n已识别的日志类型:")
        if os.path.isdir(f"{self.log_dir}/blackbox"):
            print("  ✓ blackbox (黑盒子日志)")
        if os.path.isfile(f"{self.log_dir}/log_new.db"):
            print("  ✓ log_new.db (数据库日志)")
        if os.path.isdir(f"{self.log_dir}/checkitem"):
            print("  ✓ checkitem (监控指标)")

        # 显示数据库统计
        if os.path.isfile(f"{self.log_dir}/log_new.db"):
            stats = self.stats()
            if "alert_count" in stats:
                print(f"\n数据库: {stats['alert_count']} 条告警, {stats['log_count']} 条操作日志")

        # 显示子目录
        print("\n可用子目录:")
        for d in self.ls()[:10]:
            print(f"  - {d}")


# ========== 全局函数 ==========

_log = None

def init(qno: str) -> SmartLog:
    """初始化全局适配器"""
    global _log
    _log = SmartLog()
    _log.init(qno)
    return _log

def dir(subdir: str = "") -> str:
    return _log.dir(subdir) if _log else ""

def ls(subdir: str = "") -> List[str]:
    return _log.ls(subdir) if _log else []

def grep(keyword: str, subdir: str = "blackbox") -> List[Dict]:
    return _log.grep(keyword, subdir) if _log else []

def cat(filepath: str) -> str:
    return _log.cat(filepath) if _log else ""

def tail(filepath: str, lines: int = 100) -> str:
    return _log.tail(filepath, lines) if _log else ""

def sql(query: str) -> List[Dict]:
    return _log.sql(query) if _log else []

def alerts(level: str = "", limit: int = 100) -> List[Dict]:
    return _log.alerts(level, limit) if _log else []

def logs(user: str = "", limit: int = 100) -> List[Dict]:
    return _log.logs(user, limit) if _log else []

def search(keyword: str, limit: int = 50) -> Dict:
    return _log.search(keyword, limit) if _log else {}

def stats() -> Dict:
    return _log.stats() if _log else {}

def info():
    if _log:
        _log.info()


def smart_icare(qno: str) -> SmartLog:
    """一键初始化"""
    adapter = SmartLog()
    adapter.init(qno)
    adapter.info()
    return adapter


# ========== 主函数 ==========

if __name__ == "__main__":
    import sys

    if len(sys.argv) < 2:
        print("用法: python smart_log_api.py <Q单号>")
        print("示例: python smart_log_api.py Q2026031700281")
        sys.exit(1)

    smart_icare(sys.argv[1])
