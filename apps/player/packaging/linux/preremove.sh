#!/bin/sh
set -eu
case "${1:-}" in
  remove|purge|0)
    if [ -d /run/systemd/system ]; then
      systemctl disable --now kinosail.service >/dev/null 2>&1 || true
    fi
    ;;
esac
