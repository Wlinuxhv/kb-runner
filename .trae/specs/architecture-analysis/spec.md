# 项目架构分析规范

## Why
项目需要一个清晰的架构文档，用于理解 KB Runner 的系统设计、模块关系和扩展点。

## What Changes
- 创建完整的项目架构分析文档
- 识别核心模块和依赖关系
- 绘制系统架构图
- 识别关键设计模式和扩展点

## Impact
- Affected specs: 项目架构文档
- Affected code: 全项目

## ADDED Requirements
### Requirement: 项目架构分析文档
KB Runner 是一个用于执行 KB 检查脚本的框架，提供命令行和 Web 界面。

#### Scenario: 系统架构概览
- **WHEN** 分析项目架构时
- **THEN** 识别以下核心组件：
  - **cmd/kb-runner**: CLI 入口，使用 Cobra 框架
  - **internal/adapter**: 脚本适配器，支持 Bash 和 Python
  - **internal/executor**: 任务执行引擎，支持并行执行
  - **internal/cases**: CASE 管理（检查项配置）
  - **internal/scenario**: 场景管理（多个 CASE 的组合）
  - **internal/processor**: 结果处理器，生成结果矩阵
  - **internal/api**: Web 服务器，提供 REST API 和 Web UI
  - **pkg/config**: 配置管理
  - **pkg/logger**: 日志系统
  - **pkg/result**: 结果数据模型
  - **中台/**: 技能库（100+ 故障排查脚本）

#### Scenario: 脚本执行流程
- **WHEN** 用户执行 KB 脚本时
- **THEN** 系统按以下流程执行：
  1. 解析命令行参数或 API 请求
  2. 加载 CASE/场景配置
  3. 选择对应的脚本适配器 (Bash/Python)
  4. 执行脚本并捕获输出
  5. 解析脚本返回的 JSON 结果
  6. 计算加权得分并生成结果矩阵

#### Scenario: Web 服务架构
- **WHEN** 启动 Web 服务时
- **THEN** 提供以下功能：
  - RESTful API (API v1)
  - 前端 Web UI
  - 认证中间件 (Token)
  - 执行历史管理
  - 技能库管理

#### Scenario: 技能库架构
- **WHEN** 查看技能库时
- **THEN** 技能按问题类型组织：
  - 存储问题 (FC/iSCSI/NFS)
  - 网络问题 (网卡驱动)
  - 虚拟机问题
  - 硬件问题 (CPU/内存/显卡)
  - 系统问题 (内核/驱动)
  - 每个技能包含 Skill.md 和多个排查步骤脚本

## MODIFIED Requirements
### Requirement: 设计模式
项目使用了多种经典设计模式：

#### 1. 适配器模式 (Adapter Pattern)
- **位置**: internal/adapter
- **描述**: 脚本适配器统一接口，BashAdapter 和 PythonAdapter 实现相同的 Adapter 接口
- **好处**: 解耦执行引擎与具体脚本语言，支持扩展新语言

#### 2. 注册表模式 (Registry Pattern)
- **位置**: internal/adapter.Registry
- **描述**: 集中管理所有适配器，支持动态注册和获取
- **好处**: 避免硬编码，支持运行时添加新适配器

#### 3. 命令模式 (Command Pattern)
- **位置**: internal/adapter.Task
- **描述**: Task 封装了脚本执行的所有参数（路径、语言、超时等）
- **好处**: 解耦请求与执行，支持任务队列和撤销

#### 4. 策略模式 (Strategy Pattern)
- **位置**: internal/executor
- **描述**: 不同脚本语言使用不同执行策略
- **好处**: 算法可替换，易于添加新执行策略

#### 5. 模板方法模式 (Template Method)
- **位置**: internal/executor.Execute
- **描述**: 执行流程固定（验证→准备→执行→清理），子类可重写特定步骤
- **好处**: 代码复用，易于扩展

#### 6. 工厂模式 (Factory Pattern)
- **位置**: cases.Manager, scenario.Manager
- **描述**: 从配置文件加载并创建 Case/Scenario 对象
- **好处**: 对象创建与使用分离

#### 7. 单例模式 (Singleton)
- **位置**: pkg/logger.Logger (隐式)
- **描述**: 日志系统全局单例
- **好处**: 全局唯一，避免重复初始化

#### 8. 中间件模式 (Middleware)
- **位置**: internal/api/auth.go
- **描述**: HTTP 中间件链式处理认证、日志等
- **好处**: 关注点分离，易于组合

### Requirement: 扩展点
项目设计的扩展点：

#### 1. 新脚本语言支持
- **方式**: 实现 adapter.Adapter 接口
- **示例**: 添加 PowerShellAdapter、AnsibleAdapter
- **注册**: engine.RegisterAdapter()

#### 2. 新结果格式支持
- **方式**: 实现 result.Serializer 接口
- **示例**: 添加 XML、CSV 导出格式

#### 3. 新认证方式
- **位置**: internal/api/auth.go
- **方式**: 实现 http.Handler 接口
- **示例**: OAuth2、JWT、LDAP

#### 4. 新执行策略
- **位置**: internal/executor
- **方式**: 实现自定义调度算法
- **示例**: 优先级调度、依赖调度

#### 5. 远程执行支持
- **位置**: internal/adapter
- **方式**: 实现 SSH 适配器或容器化执行
- **示例**: 集群环境远程执行

### Requirement: 最佳实践
项目中体现的最佳实践：

#### 1. 依赖注入
- Config、Logger 通过构造函数注入
- 便于单元测试和 Mock

#### 2. 接口编程
- 核心功能使用接口定义
- Adapter、Processor 等都有清晰接口

#### 3. 配置外部化
- 所有配置在 config.yaml
- 支持命令行覆盖

#### 4. 错误处理
- 使用 errors.Is/As 判断错误类型
- 错误信息包含上下文

#### 5. 并发安全
- 使用 sync.RWMutex 保护共享资源
- 使用 sync.WaitGroup 协调并发任务

#### 6. 资源清理
- defer 确保文件句柄关闭
- 临时文件及时清理
