#!/bin/sh
set -e

case "$1" in
configure)
    mkdir -p /var/lib/goproxy /etc/goproxy
    systemctl daemon-reload
    if systemctl is-enabled --quiet goproxy 2>/dev/null; then
        systemctl restart goproxy || true
    else
        systemctl enable goproxy
        systemctl start goproxy || true
    fi
    ;;
esac
