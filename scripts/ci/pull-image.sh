#!/usr/bin/env bash
# Reject mutable or mismatched publication inputs before calling Docker.
set -euo pipefail
[[ $# == 2 ]] || { echo 'expected app and immutable image reference' >&2; exit 2; }
case "$1" in player|subtitles) ;; *) echo 'invalid app' >&2; exit 2 ;; esac
[[ "$2" =~ ^ghcr\.io/kinosail/kinosail-$1@sha256:[a-f0-9]{64}$ ]] || { echo 'invalid image digest reference' >&2; exit 2; }
exec docker pull "$2"
