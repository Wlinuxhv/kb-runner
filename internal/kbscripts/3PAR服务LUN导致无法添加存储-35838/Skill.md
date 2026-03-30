# 3PAR服务LUN导致无法添加存储

**KB ID**: 35838
**来源**: 【HCI-中台】3PARdata服务端透传服务LUN 导致HCI 无法添加外置存储

## 问题概述

服务端透传服务 LUN 异常，没有透传数据 LUN，而是服务 LUN，导致前台添加存储界面为空。

## 告警匹配

- **告警来源**: 无直接告警
- **触发条件**: 存储端配置问题

## 排查步骤

### 步骤0：查询相关告警

**脚本**: `step00-check-alerts.bash`

### 步骤1：检查 iSCSI 认证状态

**脚本**: `step01-check-iscsi-auth.bash`

检查 iSCSI 服务器认证状态。

### 步骤2：检查内核日志 SES 设备

**脚本**: `step02-check-ses-device.bash`

检查内核日志中的 3PARdata SES 设备信息。

**关键字**: `3PARdata SES`、`LUN 0`、`LUN 254`

## 解决方案

**根因**: 服务端配置问题

**处理方式**: 服务端检查配置，尤其是 IQN 是否匹配

**注意**: LUN 0 或 LUN 254 可能是服务 LUN，不是数据 LUN

## 建议

1. 确认存储端映射的是数据 LUN
2. 检查 IQN 配置是否正确