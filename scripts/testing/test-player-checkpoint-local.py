#!/usr/bin/env python3
"""Bounded real Player/browser progress proof with preserved disposable state."""
import argparse
import base64
import hashlib
import hmac
import json
import os
from pathlib import Path
import socket
import subprocess
import time
import urllib.request

parser = argparse.ArgumentParser()
parser.add_argument("--phase", choices=["deployed", "current", "candidate"], required=True)
args = parser.parse_args()
root = Path(__file__).resolve().parents[2]
run = root / ".verification/paused-seek-checkpoint" / time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())
run.mkdir(parents=True)
media = run / "media"
media.mkdir()
revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
diff = subprocess.check_output(["git", "diff", "HEAD"], cwd=root)
sources = ["packages/webassets/static/player-progress.js", "packages/playerweb/player_template.go",
           "packages/playerweb/progress_notice.go", "apps/player/internal/server/static/player-streaming-recovery.js",
           "apps/player/e2e/test-instance-checkpoint.spec.ts", "packages/webassets/static/player-presentation.js",
           "packages/webassets/static/player-core.js", "packages/webassets/static/player-status.js",
           "apps/player/e2e/player-preparation-progress.spec.ts", "apps/player/e2e/test-instance-progress.spec.ts",
           "scripts/testing/test-player-checkpoint-local.py"]
receipt = {"revision": revision, "workingDiffSHA256": hashlib.sha256(diff).hexdigest(),
           "sourceSHA256": {path: hashlib.sha256((root / path).read_bytes()).hexdigest() for path in sources},
           "phase": args.phase, "command": "GOMAXPROCS=2 python3 scripts/testing/test-player-checkpoint-local.py --phase " + args.phase,
           "environment": "Native Go Kinosail Server; loopback HTTP; one Chromium worker",
           "data": "Disposable synthetic Owner, TOTP, and generated 30-second video. State preserved.",
           "boundaries": "Browser baseline replays the base progress asset against the same real Server. No production, container, device, physical TV, or TLS deployment proof.",
           "result": "failed", "runs": []}
try:
    subprocess.run(["ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=320x180:r=24:d=30",
                    "-c:v", "libx264", "-threads", "1", "-preset", "ultrafast", "-crf", "35", "-pix_fmt", "yuv420p",
                    "-movflags", "+faststart", str(media / "Checkpoint Example.mp4")], check=True)
    binary = run / "kinosail-player"
    subprocess.run(["go", "build", "-p", "1", "-o", str(binary), "./cmd/kinosail"],
                   cwd=root / "apps/player", env={**os.environ, "GOMAXPROCS": "2"}, check=True)
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]
    url = f"http://localhost:{port}"
    env = {**os.environ, "GOMAXPROCS": "2", "KINOSAIL_LISTEN": f"127.0.0.1:{port}",
           "KINOSAIL_AUTH_URL": url, "KINOSAIL_TLS_ENABLED": "false", "KINOSAIL_DATA_DIR": str(run / "config"),
           "KINOSAIL_MEDIA_DIR": str(media), "KINOSAIL_CACHE_DIR": str(run / "cache"),
           "KINOSAIL_BACKUP_DIR": str(run / "backups"), "KINOSAIL_BACKUP_KEY": os.urandom(32).hex()}
    with (run / "server-private.log").open("w") as server_log:
        server = subprocess.Popen([str(binary)], cwd=root, env=env, stdout=server_log, stderr=subprocess.STDOUT)
        try:
            for attempt in range(100):
                try:
                    with urllib.request.urlopen(url + "/healthz") as response:
                        if response.status == 200:
                            break
                except OSError:
                    time.sleep(.1)
            else:
                raise RuntimeError("Disposable Server readiness failed")
            def request(path, body, token="", method="POST"):
                headers = {"Content-Type": "application/json"}
                if token:
                    headers["Authorization"] = "Bearer " + token
                value = urllib.request.Request(url + path, method=method, data=json.dumps(body).encode(), headers=headers)
                with urllib.request.urlopen(value) as response:
                    return json.load(response) if response.status != 204 else None
            created = request("/api/v1/setup", {"name": "Owner", "password": "synthetic-progress-password", "device": "Disposable checkpoint proof", "totp": True})
            token, secret = created["token"], created["totp"]["secret"]
            digest = hmac.new(base64.b32decode(secret), int(time.time() // 30).to_bytes(8, "big"), hashlib.sha1).digest()
            offset = digest[-1] & 15
            code = str((int.from_bytes(digest[offset:offset + 4], "big") & 0x7fffffff) % 1000000).zfill(6)
            request("/api/v1/me/mfa", {"code": code}, token, "PUT")
            request("/api/v1/settings/onboarding", {"enabled": False}, token, "PUT")
            browser_env = {**env, "KINOSAIL_TEST_INSTANCE": "1", "KINOSAIL_TEST_TOTP_SECRET": secret,
                           "KINOSAIL_E2E_OWNER_PASSWORD": "synthetic-progress-password", "KINOSAIL_E2E_URL": url,
                           "KINOSAIL_E2E_VIDEO": "off", "KINOSAIL_BROWSER_WORKERS": "1"}
            for phase in [args.phase]:
                selected = ["test-instance-checkpoint.spec.ts"] + (["test-instance-progress.spec.ts"] if phase == "candidate" else [])
                command = ["node", "node_modules/@playwright/test/cli.js", "test", *selected,
                           "--project=chromium", "--workers=1", "--repeat-each=2"]
                browser_env["KINOSAIL_CHECKPOINT_SOURCE"] = phase
                browser_env["KINOSAIL_E2E_ARTIFACT_DIR"] = str(run / (phase + "-report"))
                browser_env["KINOSAIL_E2E_OUTPUT_DIR"] = str(run / (phase + "-artifacts"))
                with (run / (phase + "-browser.log")).open("w") as browser_log:
                    result = subprocess.run(command, cwd=root / "apps/player/e2e", env=browser_env, stdout=browser_log, stderr=subprocess.STDOUT)
                receipt["runs"].append({"phase": phase, "command": " ".join(command), "exitCode": result.returncode})
                if result.returncode:
                    raise RuntimeError("Browser progress proof failed")
            receipt["result"] = "passed"
        finally:
            server.terminate()
            try:
                server.wait(timeout=5)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait()
finally:
    checksums = {str(path.relative_to(run)): hashlib.sha256(path.read_bytes()).hexdigest()
                 for path in run.rglob("*") if path.is_file() and path.name != "kinosail-player"}
    receipt["checksums"] = checksums
    (run / "receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")
    print(f"Checkpoint native Server proof: {receipt['result']}; receipt: {run / 'receipt.json'}")
