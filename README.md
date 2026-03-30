# bw-guardian

部署在 KVM+VirtFusion 宿主机上的 VPS 带宽监控与自动降速工具。通过 systemd timer 每分钟检测一次各 VM 的实时流量，对持续超标的 VM 自动降速，流量恢复后自动解除，累计违规达上限后永久限速。

## 环境要求

- Debian 12（宿主机）
- KVM + libvirt + VirtFusion
- `tc`（iproute2）、`virsh`

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/polesnet/polesnet-bw-guardian/main/install.sh | bash
```

自动完成：

- 从 GitHub Releases 下载对应架构的二进制到 `/usr/local/bin/bw-guardian`
- 创建配置目录 `/etc/bw-guardian/`（包含默认配置和空白名单）
- 创建状态目录 `/var/lib/bw-guardian/`
- 注册并启动 systemd timer（每分钟触发一次）

## 卸载

```bash
sudo bash uninstall.sh
```

## 配置

配置文件：`/etc/bw-guardian/config`

```ini
# 超过套餐带宽的百分比算高占用（默认 80%）
OVERUSE_RATIO=80

# 降速目标 (Mbps)
LIMIT_MBIT=10

# 连续超标几分钟触发降速
MAX_COUNT=10

# 连续正常几分钟后自动恢复
NORMAL_COUNT_MAX=30

# 累计降速几次后永久限速
MAX_THROTTLE_TIMES=3
```

## 白名单

白名单文件：`/etc/bw-guardian/whitelist`，每行一个 VM UUID，白名单内的 VM 跳过所有检测。

```
xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy
```

## 用法

```bash
# 查看所有 VM 的监控状态
bw-guardian status

# 手动解除某台 VM 的限速（含永久限速）
bw-guardian unblock <uuid>

# 手动触发一次检查（正常由 systemd timer 自动调用）
bw-guardian run

# 查看版本
bw-guardian version
```

### status 输出示例

```
UUID                                    RATE(Mbps)  THRESH(Mbps)  THROTTLED  PERM  TIMES  PKG(KB/s)
----------------------------------------------------------------------------------------------------
3a5f1c2e-...                               45.23         80.00  yes        no        2      12500
8b2d9e7a-...                                2.10         80.00  no         no        0      12500
c1f04d3b-...                                0.00         10.00  yes        yes       3          0
```

| 列 | 说明 |
|---|---|
| RATE | 当前实时速率（rx+tx 汇总） |
| THRESH | 触发降速的阈值（套餐带宽 × OVERUSE_RATIO%，最低 10 Mbps） |
| THROTTLED | 当前是否处于降速状态 |
| PERM | 是否永久限速 |
| TIMES | 累计触发降速次数 |
| PKG | 套餐带宽（来自 virsh domiftune 缓存，0 表示未限速套餐） |

## 工作原理

### 流量计算

每分钟读取 `/sys/class/net/<iface>/statistics/{rx,tx}_bytes`，汇总 VM 所有网卡，与上一分钟的记录做差得到实时速率（Mbps）。支持 64 位计数器溢出处理。

### 阈值判定

套餐带宽通过 `virsh domiftune <uuid> <iface>` 的 `inbound.average`（KB/s）获取，首次读取后缓存到状态文件，不重复查询。

```
threshold = pkg_kbps × 8 / 1000 × OVERUSE_RATIO / 100
threshold = max(threshold, 10.0)   # 最低 10 Mbps
```

### 降速机制

**完全基于 `tc`**，不使用 `virsh domiftune` 限速（domiftune 对 UDP 流量无效）：

- **出向**（VM 下载）：`tc qdisc tbf`
- **入向**（VM 上传）：`tc qdisc ingress` + `tc filter police`，覆盖 IPv4 和 IPv6

### 状态机

```
连续 MAX_COUNT 分钟超标
  → 降速至 LIMIT_MBIT Mbps
  → times++
  → times ≥ MAX_THROTTLE_TIMES → 永久限速

降速后连续 NORMAL_COUNT_MAX 分钟正常
  → 用 tc 还原套餐带宽（不调 domiftune）
  → 恢复正常监控
```

### 永久限速

- 标记后每分钟仍重新应用 tc 规则，确保 VM 重启后依然受限
- 只能通过 `bw-guardian unblock <uuid>` 手动解除

## 日志

日志文件：`/var/log/bw-guardian.log`

```
2025-03-30 14:23:01 THROTTLE 3a5f1c2e-... iface=vnet3 rate=95.42 threshold=80.00 pkg=12500 times=1
2025-03-30 14:53:44 RECOVER  3a5f1c2e-... iface=vnet3 rate=3.21 pkg=12500
2025-03-30 15:10:05 THROTTLE 3a5f1c2e-... iface=vnet3 rate=102.17 threshold=80.00 pkg=12500 times=2
2025-03-30 15:12:05 PERMANENT 3a5f1c2e-... iface=vnet3 times=3
```

也可通过 journald 查看：

```bash
journalctl -u bw-guardian.service -f
```

## 状态文件

所有状态存储在 `/var/lib/bw-guardian/<uuid>.<type>`：

| 文件 | 内容 |
|---|---|
| `.state` | `total_bytes epoch_unix`，每分钟更新 |
| `.count` | 连续超标分钟数 |
| `.throttled` | `1` = 当前限速中 |
| `.normal` | 限速后连续正常分钟数 |
| `.times` | 累计触发降速次数（不自动重置） |
| `.permanent` | `1` = 永久限速，仅 unblock 清除 |
| `.pkg` | 套餐带宽缓存（KB/s） |

## CI / 发布

Push tag `v*` 触发 GitHub Actions，自动构建并发布：

- `bw-guardian-linux-amd64`
- `bw-guardian-linux-arm64`

## 项目结构

```
polesnet-bw-guardian/
├── cmd/bw-guardian/
│   ├── main.go          CLI 入口
│   ├── run.go           监控主循环与状态机
│   ├── status.go        状态表格展示
│   └── unblock.go       手动解除限速
├── internal/
│   ├── config/          配置加载
│   ├── state/           状态文件读写
│   ├── tc/              tc 命令封装
│   └── virsh/           virsh 命令封装
├── install.sh
└── uninstall.sh
```
