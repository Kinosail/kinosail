#!/usr/bin/env bash
set -euo pipefail
[[ "$#" == 1 ]] || exit 2
r06_suite="${CAMPAIGN_R06_SUITE-protocol}"
case "$r06_suite" in
  protocol|save-controls|save-headers|save-body|source-format) ;;
  *) exit 2 ;;
esac
if [[ "$1" != R06 && "$r06_suite" != protocol ]]; then exit 2; fi
q47_suite="${CAMPAIGN_Q47_SUITE-source-format}"
case "$q47_suite" in
  source-format|primary) ;;
  *) exit 2 ;;
esac
if [[ "$1" != Q47 && "$q47_suite" != source-format ]]; then exit 2; fi
case "$1" in
  R06)
    if [[ "$r06_suite" == protocol ]]; then
      exec python3 apps/subtitles/scripts/campaign-r06-public.py
    elif [[ "$r06_suite" == source-format ]]; then
      exec python3 apps/subtitles/scripts/campaign_r06_format.py
    else
      exec python3 apps/subtitles/scripts/campaign-r06-browser.py
    fi ;;
  Q14) exec python3 apps/player/scripts/campaign-q14-public.py ;;
  Q09) exec python3 apps/player/scripts/campaign-q09-public.py ;;
  Q47)
    if [[ "$q47_suite" == source-format ]]; then
      exec python3 apps/player/scripts/campaign_q47_format.py
    else
      exec python3 apps/player/scripts/campaign-q47-public.py
    fi ;;
  *) exit 2 ;;
esac
