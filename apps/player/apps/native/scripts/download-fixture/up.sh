#!/usr/bin/env bash
set -euo pipefail

native="$(cd "$(dirname "$0")/../.." && pwd)"
player="$(cd "$native/../.." && pwd)"
root="${KINOSAIL_TEST_ROOT:-$player/.kinosail-test}"
proxy_port="${KINOSAIL_DOWNLOAD_PROXY_PORT:-38300}"
control_port="${KINOSAIL_DOWNLOAD_CONTROL_PORT:-38301}"
cd "$player"
./scripts/test-instance.sh up
base="$(./scripts/test-instance.sh url)"
movie="$root/media/Movies/Long Haul (2024).mp4"
ffmpeg -loglevel error -y -f lavfi -i 'testsrc2=size=1280x720:rate=24' -f lavfi -i 'sine=frequency=330:sample_rate=48000' -t 150 \
  -c:v libx264 -preset ultrafast -b:v 8M -maxrate 8M -bufsize 16M -pix_fmt yuv420p -c:a aac -b:a 128k -movflags +faststart "$movie"
printf '<movie><title>Long Haul</title><year>2024</year></movie>\n' >"${movie%.mp4}.nfo"
token="$(curl --silent --insecure --fail --header 'Content-Type: application/json' \
  --data "{\"name\":\"Owner\",\"password\":\"test-instance-password\",\"code\":\"$(./scripts/test-instance.sh totp)\",\"device\":\"Download fixture\"}" \
  "$base/api/v1/session" | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')"
printf %s "$token" >"$root/owner-token"
chmod 600 "$root/owner-token"
curl --silent --insecure --fail --output /dev/null --request POST --header "Authorization: Bearer $token" "$base/api/v1/tasks/scan"
for _ in {1..120}; do
  if curl --silent --insecure --fail --header "Authorization: Bearer $token" "$base/api/v1/library?limit=100" | grep -q '"title":"Long Haul"'; then break; fi
  sleep 1
done
port="${base##*:}"
nohup python3 "$native/scripts/download-fixture/fault_proxy.py" "$proxy_port" "$control_port" 127.0.0.1 "$port" >"$root/fault-proxy.log" 2>&1 &
printf %s "$!" >"$root/fault-proxy.pid"
for _ in {1..40}; do curl --silent --fail "http://127.0.0.1:$control_port/" >/dev/null && break; sleep 0.25; done
shasum -a 256 "$movie"
printf 'Download fixture: http://127.0.0.1:%s (control http://127.0.0.1:%s)\n' "$proxy_port" "$control_port"
