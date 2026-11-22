#!/bin/sh
set -eu
if [ -d /run/systemd/system ]; then
  systemctl daemon-reload || true
fi
if [ "${1:-}" = purge ]; then
  rm -rf /var/lib/kinosail /var/cache/kinosail /var/log/kinosail
fi
