#!/bin/bash
# bw-guardian 安装脚本
# https://github.com/polesnet/polesnet-bw-guardian
#
# 用法:
#   curl -fsSL https://raw.githubusercontent.com/polesnet/polesnet-bw-guardian/main/install.sh | bash

set -e

REPO="polesnet/polesnet-bw-guardian"
BIN_PATH="/usr/local/bin/bw-guardian"
CONFIG_DIR="/etc/bw-guardian"
STATE_DIR="/var/lib/bw-guardian"
SYSTEMD_DIR="/etc/systemd/system"
LOG_FILE="/var/log/bw-guardian.log"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# --- Root check ---
if [ "$EUID" -ne 0 ]; then
    log_error "请使用 root 权限运行"
    exit 1
fi

# --- Architecture ---
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  BIN_ARCH="amd64" ;;
    aarch64) BIN_ARCH="arm64" ;;
    *)
        log_error "不支持的系统架构: $ARCH"
        exit 1
        ;;
esac

echo "=== bw-guardian 安装 ==="
echo "架构: $ARCH"
echo ""

# --- Download binary ---
BIN_URL="https://github.com/${REPO}/releases/latest/download/bw-guardian-linux-${BIN_ARCH}"
log_info "下载 bw-guardian ($BIN_ARCH)..."
if ! curl -fsSL "$BIN_URL" -o "$BIN_PATH"; then
    log_error "下载失败: $BIN_URL"
    exit 1
fi
chmod +x "$BIN_PATH"
log_info "已安装到 $BIN_PATH"

# --- Directories ---
mkdir -p "$CONFIG_DIR" "$STATE_DIR"

# --- Default config ---
if [ ! -f "${CONFIG_DIR}/config" ]; then
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
    log_info "已创建默认配置: ${CONFIG_DIR}/config"
else
    log_warn "配置文件已存在，跳过: ${CONFIG_DIR}/config"
fi

# --- Whitelist ---
if [ ! -f "${CONFIG_DIR}/whitelist" ]; then
    touch "${CONFIG_DIR}/whitelist"
    log_info "已创建空白白名单: ${CONFIG_DIR}/whitelist"
fi

# --- systemd service ---
cat > "${SYSTEMD_DIR}/bw-guardian.service" <<EOF
[Unit]
Description=bw-guardian bandwidth check (single run)
After=network.target libvirtd.service
Wants=libvirtd.service

[Service]
Type=oneshot
ExecStart=${BIN_PATH} run
StandardOutput=append:${LOG_FILE}
StandardError=append:${LOG_FILE}
EOF

# --- systemd timer ---
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
log_info "已启用 systemd timer"

echo ""
log_info "bw-guardian 安装完成"
echo ""
echo "  配置文件: ${CONFIG_DIR}/config"
echo "  白名单:   ${CONFIG_DIR}/whitelist"
echo "  日志文件: ${LOG_FILE}"
echo ""
echo "  查看状态:        bw-guardian status"
echo "  解除限速:        bw-guardian unblock <uuid>"
echo "  查看日志:        journalctl -u bw-guardian.service -f"
echo "  手动触发一次:    systemctl start bw-guardian.service"
