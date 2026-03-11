#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
KB脚本执行框架 - Python API
提供日志输出和结果记录功能
"""

import json
import os
import sys
import time
from dataclasses import dataclass, field, asdict
from datetime import datetime
from enum import Enum
from typing import Any, Dict, List, Optional
import threading


class LogLevel(Enum):
    DEBUG = "DEBUG"
    INFO = "INFO"
    WARN = "WARN"
    ERROR = "ERROR"


class StepStatus(Enum):
    SUCCESS = "success"
    FAILURE = "failure"
    WARNING = "warning"
    SKIPPED = "skipped"


class ScriptStatus(Enum):
    RUNNING = "running"
    SUCCESS = "success"
    FAILURE = "failure"
    WARNING = "warning"


@dataclass
class StepResult:
    name: str
    status: str = "running"
    message: str = ""
    output: str = ""
    duration_ms: int = 0
    start_time: str = ""
    end_time: str = ""
    results: Dict[str, Any] = field(default_factory=dict)


@dataclass
class ScriptResult:
    script_name: str
    status: str = "running"
    steps: List[Dict] = field(default_factory=list)
    results: Dict[str, Any] = field(default_factory=dict)
    score: float = 0.0
    message: str = ""
    start_time: str = ""
    end_time: str = ""


class KBLogger:
    _instance = None
    _lock = threading.Lock()
    
    def __new__(cls):
        if cls._instance is None:
            with cls._lock:
                if cls._instance is None:
                    cls._instance = super().__new__(cls)
                    cls._instance._initialize()
        return cls._instance
    
    def _initialize(self):
        self.log_file = os.environ.get("KB_LOG_FILE", "/tmp/kb.log")
        self.log_level = LogLevel(os.environ.get("KB_LOG_LEVEL", "INFO"))
        self.script_name = os.environ.get("KB_SCRIPT_NAME", "unknown")
        self.current_step: Optional[str] = None
        
        self._level_order = {
            LogLevel.DEBUG: 0,
            LogLevel.INFO: 1,
            LogLevel.WARN: 2,
            LogLevel.ERROR: 3,
        }
    
    def _should_log(self, level: LogLevel) -> bool:
        return self._level_order[level] >= self._level_order[self.log_level]
    
    def _log(self, level: LogLevel, message: str) -> None:
        if not self._should_log(level):
            return
        
        timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        prefix = f"[{timestamp}] [{level.value}]"
        
        if self.current_step:
            prefix = f"{prefix} [{self.script_name}:{self.current_step}]"
        else:
            prefix = f"{prefix} [{self.script_name}]"
        
        log_line = f"{prefix} {message}\n"
        
        try:
            with open(self.log_file, "a", encoding="utf-8") as f:
                f.write(log_line)
        except Exception:
            pass
        
        if level == LogLevel.ERROR:
            print(log_line, end="", file=sys.stderr)
    
    def debug(self, message: str) -> None:
        self._log(LogLevel.DEBUG, message)
    
    def info(self, message: str) -> None:
        self._log(LogLevel.INFO, message)
    
    def warn(self, message: str) -> None:
        self._log(LogLevel.WARN, message)
    
    def error(self, message: str) -> None:
        self._log(LogLevel.ERROR, message)


class KBResult:
    _instance = None
    _lock = threading.Lock()
    
    def __new__(cls):
        if cls._instance is None:
            with cls._lock:
                if cls._instance is None:
                    cls._instance = super().__new__(cls)
                    cls._instance._initialize()
        return cls._instance
    
    def _initialize(self):
        self.result_file = os.environ.get("KB_RESULT_FILE", "/tmp/kb_result.json")
        self.script_name = os.environ.get("KB_SCRIPT_NAME", "unknown")
        self.current_step: Optional[StepResult] = None
        self.result = ScriptResult(
            script_name=self.script_name,
            start_time=datetime.now().isoformat()
        )
        self._save_result()
    
    def _save_result(self) -> None:
        try:
            with open(self.result_file, "w", encoding="utf-8") as f:
                json.dump(asdict(self.result), f, indent=2, ensure_ascii=False)
        except Exception:
            pass
    
    def step_start(self, name: str) -> None:
        if self.current_step is not None:
            self.step_warning("步骤未正确关闭")
        
        self.current_step = StepResult(
            name=name,
            start_time=datetime.now().isoformat()
        )
        self._save_result()
    
    def _end_step(self, status: StepStatus, message: str = "") -> None:
        if self.current_step is None:
            return
        
        self.current_step.status = status.value
        self.current_step.message = message
        self.current_step.end_time = datetime.now().isoformat()
        
        if self.current_step.start_time:
            try:
                start = datetime.fromisoformat(self.current_step.start_time)
                end = datetime.fromisoformat(self.current_step.end_time)
                self.current_step.duration_ms = int((end - start).total_seconds() * 1000)
            except Exception:
                pass
        
        self.result.steps.append(asdict(self.current_step))
        self.current_step = None
        self._save_result()
    
    def step_success(self, message: str = "执行成功") -> None:
        self._end_step(StepStatus.SUCCESS, message)
    
    def step_failure(self, message: str = "执行失败") -> None:
        self._end_step(StepStatus.FAILURE, message)
    
    def step_warning(self, message: str = "执行警告") -> None:
        self._end_step(StepStatus.WARNING, message)
    
    def step_skip(self, message: str = "跳过执行") -> None:
        self._end_step(StepStatus.SKIPPED, message)
    
    def step_output(self, output: str) -> None:
        if self.current_step:
            self.current_step.output = output
    
    def result_add(self, key: str, value: Any) -> None:
        self.result.results[key] = value
        self._save_result()
    
    def save(self) -> None:
        if self.current_step is not None:
            self.step_warning("步骤未正确关闭")
        
        self.result.end_time = datetime.now().isoformat()
        
        failure_count = sum(1 for s in self.result.steps if s["status"] == "failure")
        warning_count = sum(1 for s in self.result.steps if s["status"] == "warning")
        
        if failure_count > 0:
            self.result.status = "failure"
            self.result.score = 0.0
        elif warning_count > 0:
            self.result.status = "warning"
            self.result.score = max(0.0, 1.0 - warning_count * 0.1)
        else:
            self.result.status = "success"
            self.result.score = 1.0
        
        self._save_result()


class KBAPI:
    _instance = None
    _lock = threading.Lock()
    
    def __new__(cls):
        if cls._instance is None:
            with cls._lock:
                if cls._instance is None:
                    cls._instance = super().__new__(cls)
                    cls._instance._initialize()
        return cls._instance
    
    def _initialize(self):
        self.logger = KBLogger()
        self.result = KBResult()
    
    def log_debug(self, message: str) -> None:
        self.logger.debug(message)
    
    def log_info(self, message: str) -> None:
        self.logger.info(message)
    
    def log_warn(self, message: str) -> None:
        self.logger.warn(message)
    
    def log_error(self, message: str) -> None:
        self.logger.error(message)
    
    def step_start(self, name: str) -> None:
        self.logger.current_step = name
        self.logger.info(f"步骤开始: {name}")
        self.result.step_start(name)
    
    def step_success(self, message: str = "执行成功") -> None:
        self.result.step_success(message)
        self.logger.current_step = None
        self.logger.info(f"步骤成功: {message}")
    
    def step_failure(self, message: str = "执行失败") -> None:
        self.result.step_failure(message)
        self.logger.current_step = None
        self.logger.error(f"步骤失败: {message}")
    
    def step_warning(self, message: str = "执行警告") -> None:
        self.result.step_warning(message)
        self.logger.current_step = None
        self.logger.warn(f"步骤警告: {message}")
    
    def step_skip(self, message: str = "跳过执行") -> None:
        self.result.step_skip(message)
        self.logger.current_step = None
        self.logger.info(f"步骤跳过: {message}")
    
    def step_output(self, output: str) -> None:
        self.result.step_output(output)
    
    def result_add(self, key: str, value: Any) -> None:
        self.result.result_add(key, value)
    
    def save(self) -> None:
        self.result.save()
        self.logger.info("脚本执行完成")
    
    def get_param(self, key: str, default: str = "") -> str:
        var_name = f"KB_PARAM_{key.upper()}"
        return os.environ.get(var_name, default)


kb = KBAPI()
