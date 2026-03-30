# bw-guardian - VPS 带宽监控与自动降速工具

`bw-guardian` 部署在 KVM+VirtFusion 宿主机上，通过 systemd timer 每分钟检测各 VM 的实时流量，对持续超标的 VM 自动降速，流量恢复后自动解除，累计违规达上限后永久限速。

## 功能特性

- ✅ **自动降速**：连续超标达到阈值后，通过 `tc` 自动将 VM 限速至目标带宽
- ✅ **自动恢复**：流量持续正常后自动还原套餐带宽，无需人工干预
- ✅ **三振出局**：累计降速次数达上限后永久限速，VM 重启后依然生效
- ✅ **白名单**：支持跳过指定 VM 的所有检测
- ✅ **TCP/UDP 全覆盖**：基于 `tc` 实现，不依赖 `virsh domiftune`（domiftune 对 UDP 无效）
- ✅ **零依赖**：Go 编写，单一二进制，无需 Python 或其它运行时

## 快速开始

### 一键安装

使用 **curl**：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/polesnet/polesnet-bw-guardian/main/install.sh)
```

使用 **wget**：

```bash
bash <(wget -qO- https://raw.githubusercontent.com/polesnet/polesnet-bw-guardian/main/install.sh)
```

### 卸载

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/polesnet/polesnet-bw-guardian/main/uninstall.sh)
```

## 命令说明

| 命令 | 说明 |
|:---|:---|
| `bw-guardian status` | 查看所有 VM 的监控状态（速率、阈值、是否限速） |
| `bw-guardian unblock <uuid>` | 手动解除某台 VM 的限速，包括永久限速 |
| `bw-guardian run` | 手动触发一次检查（正常由 systemd timer 自动调用） |
| `bw-guardian version` | 显示版本信息 |

### status 输出示例

```
UUID                                    RATE(Mbps)  THRESH(Mbps)  THROTTLED  PERM  TIMES  PKG(KB/s)
----------------------------------------------------------------------------------------------------
3a5f1c2e-...                               45.23         80.00  yes        no        2      12500
8b2d9e7a-...                                2.10         80.00  no         no        0      12500
c1f04d3b-...                                0.00         10.00  yes        yes       3          0
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

**阈值计算公式：**

```
threshold = max(套餐带宽(Mbps) × OVERUSE_RATIO%, 10 Mbps)
```

白名单文件：`/etc/bw-guardian/whitelist`，每行一个 VM UUID。

## 查看日志

```bash
# 实时日志
journalctl -u bw-guardian.service -f

# 历史事件（降速/恢复/永久限速）
cat /var/log/bw-guardian.log
```

## 本地构建

```bash
go build -o bw-guardian ./cmd/bw-guardian
```

## 环境要求

- Debian 12 宿主机
- KVM + libvirt + VirtFusion
- `iproute2`（tc）、`virsh`

## 许可证

MIT License
