#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
    echo "错误: 请以 root 身份运行此脚本" >&2
    exit 1
fi

echo "开始卸载 bw-guardian..."

# Stop and disable systemd timer/service
for unit in bw-guardian.timer bw-guardian.service; do
    if systemctl is-active --quiet "$unit" 2>/dev/null; then
        systemctl stop "$unit"
    fi
    if systemctl is-enabled --quiet "$unit" 2>/dev/null; then
        systemctl disable "$unit"
    fi
done

# Remove systemd unit files
for f in /etc/systemd/system/bw-guardian.timer /etc/systemd/system/bw-guardian.service; do
    if [[ -f "$f" ]]; then
        rm -f "$f"
        echo "已删除: $f"
    fi
done
systemctl daemon-reload

# Remove binary
if [[ -f /usr/local/bin/bw-guardian ]]; then
    rm -f /usr/local/bin/bw-guardian
    echo "已删除二进制文件: /usr/local/bin/bw-guardian"
fi

# Remove lock file
rm -f /var/run/bw-guardian.lock

# Prompt for state dir
if [[ -d /var/lib/bw-guardian ]]; then
    read -r -p "是否删除状态目录 /var/lib/bw-guardian？[y/N] " ans
    if [[ "${ans,,}" == "y" ]]; then
        rm -rf /var/lib/bw-guardian
        echo "已删除状态目录: /var/lib/bw-guardian"
    else
        echo "保留状态目录: /var/lib/bw-guardian"
    fi
fi

# Prompt for config dir
if [[ -d /etc/bw-guardian ]]; then
    read -r -p "是否删除配置目录 /etc/bw-guardian？[y/N] " ans
    if [[ "${ans,,}" == "y" ]]; then
        rm -rf /etc/bw-guardian
        echo "已删除配置目录: /etc/bw-guardian"
    else
        echo "保留配置目录: /etc/bw-guardian"
    fi
fi

echo ""
echo "✓ bw-guardian 已卸载"
