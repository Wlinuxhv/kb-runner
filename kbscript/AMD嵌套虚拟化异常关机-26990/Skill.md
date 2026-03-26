# AMD嵌套虚拟化导致虚拟机异常关机

## 问题描述

在AMD平台和海光硬件平台上使用嵌套虚拟化可能会导致虚拟机内部关机。使用嵌套虚拟化的虚拟机可能会导致其他在主机上运行的虚拟机出现异常内部关机的现象。

## 前提条件

1. 与AMD和海光平台相关
2. 针对HCI 6.8.0及以上版本

## HCI前台行为

- HA恢复虚拟机，描述：虚拟机运行异常，HA尝试恢复运行此虚拟机
- 内部关闭虚拟机，状态：成功

## 排查步骤

### 步骤1：排除其他原因

1. 排除用户手动操作或虚拟机内部软件操作（如Windows自动更新）
2. 排除虚拟机内部操作系统宕机（Windows蓝屏或Linux Kernel Panic）

### 步骤2：检查qemu日志

检查异常内部关机虚拟机的qemu日志：
```
/sf/log/[日期]/sfvt_qemu_[vmid].log
```
搜索 `KVM_EXIT_SHUTDOWN`，如果找到则可能与当前案例相关。

### 步骤3：检查嵌套虚拟化配置

检查主机上是否有虚拟机使用嵌套虚拟化：
- Linux虚拟机中使用KVM、VirtualBox等
- Windows虚拟机中开启HyperV、WSL2等
- 开启360安全卫士的晶核保护等

后台执行：
```bash
cat /sys/module/kvm_amd/parameters/nested
```
如果为 `1` 或 `Y`，说明嵌套虚拟化已开启。

### 步骤4：检查kernel日志

检查kernel.log是否有 `KVM_EXIT_SHUTDOWN` 堆栈信息（仅6.10.0之后有此日志）。

### 步骤5：检查虚拟机嵌套配置

```bash
grep "nested_virtualization: 1" -rn /cfs/
```
有输出则说明存在开启嵌套虚拟化的虚拟机。

## 根因

VDI场景仅支持在Intel平台上使用嵌套虚拟化。HCI场景当前已发布版本（HCI 6.10.0.R2及之前）不支持虚拟机开嵌套。

所有C86和AMD的HCI和VDI二合一版本，操作后所有开机虚拟机将无法使用AMD嵌套虚拟化功能。

## 解决方案

### 临时规避方案

1. 修改配置 `/sf/modules/loadmod.conf`：
   ```
   kvm_amd nested=1
   ```
   改为：
   ```
   kvm_amd nested=0
   ```

2. HCI主机进入维护模式后重启主机生效

3. 验证是否关闭嵌套：
   ```bash
   cat /sys/module/kvm_amd/parameters/nested
   ```
   应该为 `0` 或 `N`

4. 对所有HCI主机执行1-3操作，直到全集群嵌套虚拟化关闭

验证全集群：
```bash
sfd_cluster_cmd.sh e "cat /sys/module/kvm_amd/parameters/nested"
```

### 彻底解决方案

升级HCI 6.10.0.R2版本，打上合集补丁：
- sp-HCI-6.10.0_R2-arm-col-20250528.pkg
- sp-HCI-6.10.0_R2-c86-col-20240528.pkg

采用滚动重启的方式让关闭嵌套功能生效。

**注意**：升级前巡检中如果有嵌套虚拟化提示，且客户不存在开启嵌套的虚拟机，可以先跳过完成升级和打补丁。如果存在开启嵌套的虚拟机，转研发确认。

## KB ID

26990