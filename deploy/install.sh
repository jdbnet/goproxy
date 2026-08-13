#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
CONFIG_DIR="${CONFIG_DIR:-/etc/goproxy}"
DATA_DIR="${DATA_DIR:-/var/lib/goproxy}"
BASE_URL="${GOPROXY_DOWNLOAD_URL:-https://apps.jdbnet.co.uk}"
SERVICE_NAME="goproxy"

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  echo "Run as root or with sudo"
  exit 1
fi

case "$(uname -m)" in
  x86_64|amd64) asset="goproxy-amd64" ;;
  aarch64|arm64) asset="goproxy-arm64" ;;
  *)
    echo "Unsupported architecture: $(uname -m) (need amd64 or arm64)"
    exit 1
    ;;
esac

url="${BASE_URL}/${asset}"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

echo "Downloading ${url}"
if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$tmp" "$url"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$tmp" "$url"
else
  echo "Need curl or wget"
  exit 1
fi

install -m 755 "$tmp" "${INSTALL_DIR}/goproxy"
mkdir -p "$CONFIG_DIR" "$DATA_DIR"

if [[ ! -f "${CONFIG_DIR}/config.yaml" ]]; then
  cat > "${CONFIG_DIR}/config.yaml" <<EOF
listen: 127.0.0.1:8080
log_level: info
data_dir: ${DATA_DIR}
proxy_config: ${CONFIG_DIR}/proxy.yaml
acme_email: ""
acme_directory: https://acme-v02.api.letsencrypt.org/directory
git:
  enabled: false
  url: ""
  branch: main
  auth: ssh
  key_path: ${CONFIG_DIR}/deploy_key
  token: ""
backup:
  enabled: true
  schedule: "0 3 * * *"
  retention_days: 14
  dest: dir
  dir: ${DATA_DIR}/backups
tls:
  renew_check: 24h
  renew_before: 720h
  jitter: 1h
  retry_backoff: 15m
  retry_max: 8h
update:
  enabled: true
  url: ""
  allow_dev: false
EOF
  echo "Wrote ${CONFIG_DIR}/config.yaml"
fi

if [[ ! -f "${CONFIG_DIR}/proxy.yaml" ]]; then
  cat > "${CONFIG_DIR}/proxy.yaml" <<'EOF'
frontends: []
acls: []
backends: []
certificates: []
notifications:
  webhooks: []
EOF
  echo "Wrote ${CONFIG_DIR}/proxy.yaml"
fi

cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=GoProxy load balancer
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/goproxy ${CONFIG_DIR}/config.yaml
Restart=on-failure
RestartSec=5
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl restart "$SERVICE_NAME"

echo "GoProxy installed from ${url}"
echo "Web UI: http://127.0.0.1:8080 (edit ${CONFIG_DIR}/config.yaml)"
echo "Status: systemctl status ${SERVICE_NAME}"
