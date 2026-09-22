#!/usr/bin/env bash
# Separate targets avoid accumulating two extractor builds on one small runner.
set -euo pipefail
[[ $# == 1 ]] || { echo 'expected one Apple platform' >&2; exit 2; }
case "$1" in iOS|tvOS) ;; *) echo 'invalid Apple platform' >&2; exit 2 ;; esac
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
: "${RUNNER_TEMP:?runner temporary directory is required}"
# Xcode -jobs does not limit the legacy Swift driver, which uses CPU count.
# A clean whole-module build avoids parallel copies of the same module AST.
# Record enough evidence to distinguish compiler work from memory contention.
python3 -u -c '
import datetime, subprocess, time
while True:
    print(datetime.datetime.now(datetime.timezone.utc).isoformat())
    rows = subprocess.check_output(["ps", "-axo", "pid=,ppid=,pcpu=,rss=,comm="], text=True).splitlines()
    print("\n".join(sorted(rows, key=lambda row: int(row.split(None, 4)[3]), reverse=True)[:12]))
    subprocess.run(["sysctl", "vm.swapusage"], check=True)
    time.sleep(30)
' > "$RUNNER_TEMP/codeql-swift-resources.log" 2>&1 &
monitor=$!
trap 'kill "$monitor" 2>/dev/null || true' EXIT
xcodebuild -project "$repo/apps/player/apps/native/Kinosail.xcodeproj" \
  -scheme "Kinosail-$1" -configuration Debug -destination "generic/platform=$1 Simulator" \
  -derivedDataPath "$RUNNER_TEMP/codeql-swift-$1" -jobs 1 \
  ARCHS=arm64 ONLY_ACTIVE_ARCH=YES CODE_SIGNING_ALLOWED=NO CODE_SIGNING_REQUIRED=NO \
  COMPILATION_CACHE_ENABLE_CACHING=NO SWIFT_ENABLE_COMPILE_CACHE=NO \
  SWIFT_USE_INTEGRATED_DRIVER=NO SWIFT_COMPILATION_MODE=wholemodule \
  SWIFT_USE_PARALLEL_WHOLE_MODULE_OPTIMIZATION=NO SWIFT_USE_PARALLEL_WMO_TARGETS=NO \
  "KINOSAIL_SOURCE_REVISION=$GITHUB_SHA" build \
  2>&1 | tee "$RUNNER_TEMP/codeql-swift-build.log"
