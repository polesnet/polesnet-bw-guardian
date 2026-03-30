#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/bw-guardian"
STATE_DIR="/var/lib/bw-guardian"
SYSTEMD_DIR="/etc/systemd/system"
LOG_FILE="/var/log/bw-guardian.log"

# --- Root check ---
if [[ $EUID -ne 0 ]]; then
    echo "错误: 请以 root 身份运行此脚本" >&2
    exit 1
fi

# --- Locate binary ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  BIN="${SCRIPT_DIR}/bw-guardian-linux-amd64" ;;
    aarch64) BIN="${SCRIPT_DIR}/bw-guardian-linux-arm64" ;;
    *)       BIN="" ;;
esac

if [[ -n "$BIN" && -f "$BIN" ]]; then
    echo "安装二进制文件: $BIN → ${INSTALL_DIR}/bw-guardian"
    install -m 755 "$BIN" "${INSTALL_DIR}/bw-guardian"
elif command -v go &>/dev/null; then
    echo "未找到预编译二进制，使用 go build 编译..."
    cd "$SCRIPT_DIR"
    go build -o "${INSTALL_DIR}/bw-guardian" ./cmd/bw-guardian
    chmod 755 "${INSTALL_DIR}/bw-guardian"
else
    echo "错误: 未找到可用二进制文件，且系统未安装 Go" >&2
    exit 1
fi

# --- Create directories ---
mkdir -p "$CONFIG_DIR" "$STATE_DIR"
echo "已创建目录: $CONFIG_DIR, $STATE_DIR"

# --- Default config file ---
if [[ ! -f "${CONFIG_DIR}/config" ]]; then
    cat > "${CONFIG_DIR}/config" <<'EOF'
# bw-guardian 配置文件
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
EOF
    echo "已创建默认配置: ${CONFIG_DIR}/config"
else
    echo "配置文件已存在，跳过: ${CONFIG_DIR}/config"
fi

# --- Whitelist ---
if [[ ! -f "${CONFIG_DIR}/whitelist" ]]; then
    touch "${CONFIG_DIR}/whitelist"
    echo "已创建空白白名单: ${CONFIG_DIR}/whitelist"
else
    echo "白名单已存在，跳过: ${CONFIG_DIR}/whitelist"
fi

# --- systemd service ---
cat > "${SYSTEMD_DIR}/bw-guardian.service" <<EOF
[Unit]
Description=bw-guardian bandwidth check (single run)
After=network.target libvirtd.service
Wants=libvirtd.service

[Service]
Type=oneshot
ExecStart=${INSTALL_DIR}/bw-guardian run
StandardOutput=append:${LOG_FILE}
StandardError=append:${LOG_FILE}
EOF

# --- systemd timer (every minute) ---
cat > "${SYSTEMD_DIR}/bw-guardian.timer" <<'EOF'
[Unit]
Description=Run bw-guardian every minute

[Timer]
OnBootSec=60
OnUnitActiveSec=60
AccuracySec=10s

[Install]
WantedBy=timers.target
EOF

systemctl daemon-reload
systemctl enable --now bw-guardian.timer
echo "已启用 systemd timer: bw-guardian.timer"

echo ""
echo "✓ bw-guardian 安装完成"
echo "  配置文件: ${CONFIG_DIR}/config"
echo "  白名单:   ${CONFIG_DIR}/whitelist"
echo "  状态目录: ${STATE_DIR}"
echo "  日志文件: ${LOG_FILE}"
echo "  Timer:    ${SYSTEMD_DIR}/bw-guardian.timer"
echo ""
echo "  查看 timer 状态: systemctl status bw-guardian.timer"
echo "  手动运行一次:    systemctl start bw-guardian.service"
echo "  查看日志:        journalctl -u bw-guardian.service -f"
echo "  查看状态:        bw-guardian status"
echo "  解除限速:        bw-guardian unblock <uuid>"
