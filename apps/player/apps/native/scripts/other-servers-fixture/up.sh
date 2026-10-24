#!/bin/bash
set -euo pipefail
S="$(cd "$(dirname "$0")" && pwd)"
WORK="${1:?usage: up.sh <work directory>}"
mkdir -p "$WORK"
WORK="$(cd "$WORK" && pwd)"
JELLYFIN=docker.io/jellyfin/jellyfin@sha256:7536c1009c6ea50dadd2b244165efb357504ca0f2670abefbceb1c773cc7e13d
EMBY=docker.io/emby/embyserver@sha256:383bcdaf090eeb1fd66daccfeab59ad51f0181189a98420aaeb27e269d52ee4b
PLEX=docker.io/plexinc/pms-docker@sha256:01bdf0ad6d53198da4331540c1e73ab65db7a72e961fff97ecb0847d46055c5f

[ -d "$WORK/media/movies" ] || "$S/make-media.sh" "$WORK/media"
podman rm -f kino-src-jellyfin kino-src-emby kino-src-plex >/dev/null 2>&1 || true
podman run -d --name kino-src-jellyfin -p 127.0.0.1:8096:8096 -v "$WORK/media:/media:ro" "$JELLYFIN" >/dev/null
podman run -d --name kino-src-emby -p 127.0.0.1:8097:8096 -v "$WORK/media:/media:ro" "$EMBY" >/dev/null
podman run -d --name kino-src-plex -p 127.0.0.1:32400:32400 -e TZ=UTC \
  -e ALLOWED_NETWORKS=10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,127.0.0.0/8 -v "$WORK/media:/data:ro" "$PLEX" >/dev/null
python3 "$S/setup_server.py" http://127.0.0.1:8096 jellyfin
python3 "$S/setup_server.py" http://127.0.0.1:8097 emby
python3 "$S/setup_plex.py" http://127.0.0.1:32400
pkill -f "$S/plex_tv.py" >/dev/null 2>&1 || true
nohup python3 "$S/plex_tv.py" 8099 http://127.0.0.1:32400 >"$WORK/plex-tv.log" 2>&1 &
echo "Jellyfin http://127.0.0.1:8096 and Emby http://127.0.0.1:8097 are ready. User kino, password kino-test-password."
echo "Plex http://127.0.0.1:32400 is ready. The plex.tv stand-in runs at http://127.0.0.1:8099."
