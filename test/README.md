# KB Runner 测试脚本

## 目录结构

```
test/
├── README.md              # 本文档
├── TEST_REPORT.md         # 测试报告
├── scripts/               # 测试脚本目录
│   └── ...                # 其他测试脚本
├── test_all.sh            # 完整测试脚本
├── test_api.sh            # API 接口测试
├── test_frontend.sh       # 前端资源测试
└── test_e2e.sh            # 端到端测试
```

## 测试脚本说明

### 1. test_all.sh - 完整测试脚本
运行所有测试（API + 前端 + 端到端）

```bash
./test_all.sh
```

### 2. test_api.sh - API 接口测试
测试所有 API 端点

```bash
./test_api.sh
```

**测试项**:
- 健康检查
- KB 列表
- 场景列表
- 执行历史列表
- 执行详情
- Q 单筛选
- 删除功能权限检查

### 3. test_frontend.sh - 前端资源测试
测试前端资源加载

```bash
./test_frontend.sh
```

**测试项**:
- 首页加载
- JS 文件加载
- CSS 文件加载
- 缓存控制头
- API 数据完整性

### 4. test_e2e.sh - 端到端测试
测试完整功能流程

```bash
./test_e2e.sh
```

**测试项**:
- KB 数据来源（从 kbscript 加载）
- 执行历史数据
- 柱状图数据（normalized_score）
- Q 单目录结构
- 文件完整性

## 使用方法

### 运行完整测试
```bash
cd /home/wlinuxhv/workspace/kb-runner
./test/test_all.sh
```

### 运行单个测试
```bash
./test/test_api.sh
./test/test_frontend.sh
./test/test_e2e.sh
```

## 测试前提

1. 服务已启动: `./bin/kb-runner serve --port 8080`
2. 服务地址: http://localhost:8080
3. 测试数据已生成（执行过 KB）

## 测试结果

测试通过后会显示:
```
✓✓✓ 所有测试通过！前后端完全打通！
```

## 测试报告

测试完成后会生成 `TEST_REPORT.md`，包含:
- 测试范围
- 测试结果
- 功能清单
- 技术实现
- API 端点
- 访问地址
