#!/usr/bin/env python3
"""Repeatable local Player E2E, without containers, real media or TLS bypass."""
import hashlib
import json
import os
from pathlib import Path
import socket
import subprocess
import time
import urllib.request

root = Path(__file__).resolve().parents[3]
revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
run = root / ".verification/apple-mobile" / time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())
run.mkdir(parents=True)
binary = run / "kinosail-player"
media = run / "media"
media.mkdir()
command = ["go", "-C", "apps/player", "build", "-p=1", "-o", str(binary), "./cmd/kinosail"]
environment = dict(os.environ, GOCACHE="/tmp/kinosail-apple-go-cache")
subprocess.run(command, cwd=root, env=environment, check=True)
generate = ["ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i",
            "color=c=blue:s=640x360:d=12", "-c:v", "ffv1", "-threads", "1", str(media / "Arrival.mkv")]
subprocess.run(generate, check=True)
for name in ["Beta.mkv", "Gamma.mkv"]:
    os.link(media / "Arrival.mkv", media / name)
generate_direct = ["ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i",
                   "testsrc2=s=640x360:r=24:d=12", "-f", "lavfi", "-i", "sine=frequency=440:duration=12",
                   "-c:v", "libx264", "-threads", "1", "-preset", "veryfast", "-crf", "32", "-pix_fmt",
                   "yuv420p", "-c:a", "aac", "-movflags", "+faststart", str(media / "Direct Retry Control.mp4")]
subprocess.run(generate_direct, check=True)
(media / "Direct Retry Control.en.srt").write_text("1\n00:00:00,000 --> 00:00:11,500\nSynthetic English caption.\n")
with socket.socket() as listener:
    listener.bind(("127.0.0.1", 0))
    port = listener.getsockname()[1]
url = f"http://localhost:{port}"
environment.update(KINOSAIL_LISTEN=f"127.0.0.1:{port}", KINOSAIL_AUTH_URL=url, KINOSAIL_TLS_ENABLED="false",
                   KINOSAIL_DATA_DIR=str(run / "config"), KINOSAIL_MEDIA_DIR=str(media),
                   KINOSAIL_CACHE_DIR=str(run / "cache"), KINOSAIL_BACKUP_DIR=str(run / "backups"),
                   KINOSAIL_BACKUP_KEY="synthetic-local-e2e-key", KINOSAIL_APPLE_LOCAL_E2E="1",
                   KINOSAIL_E2E_URL=url, KINOSAIL_TEST_REVISION=revision,
                   KINOSAIL_E2E_OUTPUT_DIR=str(run / "results"), KINOSAIL_E2E_REPORT=str(run / "results.json"))
test_command = ["node", "node_modules/@playwright/test/cli.js", "test", "test-instance-apple-launch.spec.ts",
                "--config=apple-playback.config.ts", "--workers=1"]
result = None
try:
    with (run / "server.log").open("w") as log:
        server = subprocess.Popen([str(binary)], env=environment, stdout=log, stderr=log)
        try:
            for _ in range(120):
                if server.poll() is not None:
                    raise RuntimeError("Disposable Server exited before readiness")
                try:
                    with urllib.request.urlopen(url + "/healthz", timeout=1) as response:
                        if json.load(response) == {"status": "ok"}:
                            break
                except (OSError, ValueError):
                    time.sleep(0.25)
            else:
                raise RuntimeError("Disposable Server did not become healthy")
            with (run / "browser.log").open("w") as browser_log:
                result = subprocess.run(test_command, cwd=root / "apps/player/e2e", env=environment,
                                        stdout=browser_log, stderr=subprocess.STDOUT).returncode
        finally:
            server.terminate()
            try:
                server.wait(timeout=10)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait()
finally:
    checksum = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
    (run / "receipt.json").write_text(json.dumps({"revision": revision, "result": result,
        "workingDiffSHA256": hashlib.sha256(subprocess.check_output(["git", "diff", "HEAD"], cwd=root)).hexdigest(),
        "command": "python3 apps/player/scripts/test-apple-playback-local.py", "build": command,
        "browserCommand": test_command, "mediaCommands": [generate, generate_direct],
        "binarySHA256": checksum(binary), "environment": "macOS ARM64, Chrome, loopback HTTP native Go Server",
        "boundaries": "Synthetic media and account; simulated Apple API; no TLS, container, production or physical iPhone proof",
        "mediaSHA256": {p.name: checksum(p) for p in media.iterdir()}}, indent=2) + "\n")
    (run / "SHA256SUMS").write_text("".join(f"{checksum(p)}  {p.relative_to(run)}\n"
        for p in sorted(run.rglob("*")) if p.is_file() and p.name != "SHA256SUMS" and
        not any(part in {"config", "cache", "backups"} for part in p.relative_to(run).parts)))
    print(f"E2E artifact: {run}", flush=True)
raise SystemExit(result if result is not None else 1)
