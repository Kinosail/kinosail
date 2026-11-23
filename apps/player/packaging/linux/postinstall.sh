#!/bin/sh
set -eu
if ! getent group kinosail >/dev/null; then
  groupadd --system kinosail
fi
if ! getent passwd kinosail >/dev/null; then
  useradd --system --gid kinosail --home-dir /var/lib/kinosail --no-create-home --shell /usr/sbin/nologin kinosail
fi
for group in video render; do
  if getent group "$group" >/dev/null; then
    usermod --append --groups "$group" kinosail
  fi
done
install -d -m 0755 /srv/media
install -d -m 0750 -o kinosail -g kinosail /var/lib/kinosail /var/cache/kinosail /var/lib/kinosail/backups
if [ -d /run/systemd/system ]; then
  systemctl daemon-reload
  systemctl enable kinosail.service >/dev/null 2>&1
  systemctl restart kinosail.service
fi
printf '\nKinosail is running. Open https://localhost:38127 on this computer to finish setup.\n'
printf 'The setup wizard helps you pick your movie folders and share Kinosail with your TV and phone.\n\n'
