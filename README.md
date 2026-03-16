# KB Runner

KB脚本执行框架 - 用于执行KB检查脚本并生成结果矩阵。

[English](./README_EN.md) | 中文

## 功能特性

### CLI工具
- 📋 CASE管理 - 查看、筛选、搜索CASE
- 🎬 场景管理 - 定义和执行场景
- ⚡ 脚本执行 - 支持Bash、Python脚本
- 📊 结果处理 - 加权、归一化、矩阵化
- 📝 日志规范 - 标准化日志输出
- 📤 结果导出 - JSON/YAML格式

### Web界面 (P1)
- 🌐 内嵌Web前端 - 单二进制交付
- 📈 执行历史 - 本地文件存储
- 🔄 自动清理 - 可配置最大记录数
- 🎨 Oatly风格 - 手写插画风格

## 快速开始

### 下载

从 [Releases](https://github.com/Wlinuxhv/kb-runner/releases) 下载对应平台的二进制文件。

### CLI模式

```bash
# 查看帮助
./kb-runner --help

# 列出所有CASE
./kb-runner list

# 执行脚本
./kb-runner run -s ./scripts/bash/security_check.sh -l bash

# 按CASE执行
./kb-runner run --case security_check

# 按场景执行
./kb-runner run --scenario daily_check

# 交互式选择
./kb-runner run --interactive

# 初始化CASE目录
./kb-runner init my_case
```

### Web界面

```bash
# 启动Web服务
./kb-runner serve

# 指定端口
./kb-runner serve --port 8080
```

启动后访问 http://localhost:8080

## 项目结构

```
kb-runner/
├── cmd/              # CLI入口
├── internal/         # 核心模块
│   ├── adapter/     # 脚本适配器
│   ├── api/         # Web API
│   ├── cases/      # CASE管理
│   ├── executor/    # 执行引擎
│   ├── processor/  # 结果处理
│   └── scenario/    # 场景管理
├── pkg/              # 公共库
├── scripts/          # 脚本API
│   ├── bash/        # Bash API
│   └── python/      # Python API
└── configs/         # 配置文件
```

## 脚本API

### Bash

```bash
#!/bin/bash
source ./scripts/bash/api.sh

kb_init

step_start "check_xxx"
# 检查逻辑
result "key" "value"
step_success "检查通过"

kb_save
```

### Python

```python
#!/usr/bin/env python3
from kb_api import kb_init, kb_save, step_start, step_success, result

kb_init()

step_start("check_xxx")
# 检查逻辑
result("key", "value")
step_success("检查通过")

kb_save()
```

## 配置

配置文件 `configs/config.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

execution:
  timeout: 300s
  max_parallel: 10
  work_dir: "./workspace"
  temp_dir: "./temp"

scripts:
  directory: "./scripts"
  allowed_languages:
    - bash
    - python

logging:
  level: "info"
  format: "json"

history:
  enabled: true
  max_records: 4294967296
  auto_cleanup: true
  cleanup_threshold: 0.9
```

## 开发

### 构建

```bash
# 构建Linux
make build-linux

# 构建Windows
make build-windows

# 构建所有平台
make release
```

### 测试

```bash
make test
```

## 许可证

MIT License
