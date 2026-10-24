#!/bin/bash
set -euo pipefail
S="$(cd "$(dirname "$0")" && pwd)"
WORK="${1:?usage: up.sh <work directory>}"
mkdir -p "$WORK"
WORK="$(cd "$WORK" && pwd)"
JELLYFIN=docker.io/jellyfin/jellyfin@sha256:7536c1009c6ea50dadd2b244165efb357504ca0f2670abefbceb1c773cc7e13d
EMBY=docker.io/emby/embyserver@sha256:383bcdaf090eeb1fd66daccfeab59ad51f0181189a98420aaeb27e269d52ee4b

[ -d "$WORK/media/movies" ] || "$S/make-media.sh" "$WORK/media"
podman rm -f kino-src-jellyfin kino-src-emby >/dev/null 2>&1 || true
podman run -d --name kino-src-jellyfin -p 127.0.0.1:8096:8096 -v "$WORK/media:/media:ro" "$JELLYFIN" >/dev/null
podman run -d --name kino-src-emby -p 127.0.0.1:8097:8096 -v "$WORK/media:/media:ro" "$EMBY" >/dev/null
python3 "$S/setup_server.py" http://127.0.0.1:8096 jellyfin
python3 "$S/setup_server.py" http://127.0.0.1:8097 emby
echo "Jellyfin http://127.0.0.1:8096 and Emby http://127.0.0.1:8097 are ready. User kino, password kino-test-password."
