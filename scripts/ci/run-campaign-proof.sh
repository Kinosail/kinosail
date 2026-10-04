#!/usr/bin/env bash
set -euo pipefail
[[ "$#" == 1 ]] || exit 2
case "$1" in
  R06) exec python3 apps/subtitles/scripts/campaign-r06-public.py ;;
  Q14) exec python3 apps/player/scripts/campaign-q14-public.py ;;
  Q09) exec python3 apps/player/scripts/campaign-q09-public.py ;;
  *) exit 2 ;;
esac
