#!/usr/bin/env python3
"""Serial native-server layout audit with synthetic data and no TLS bypass."""
import atexit
import hashlib
import base64
import hmac
import json
import os
import platform
from pathlib import Path
import socket
import subprocess
import time
import urllib.request

root = Path(__file__).resolve().parents[2]
run = root / ".verification/layout" / time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())
run.mkdir(parents=True)
revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
media = run / "media"
media.mkdir()
generate = ["ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=640x360:r=24:d=8",
            "-c:v", "libx264", "-threads", "1", "-preset", "ultrafast", "-crf", "35",
            "-pix_fmt", "yuv420p", "-movflags", "+faststart", str(media / "Layout Example.mp4")]
subprocess.run(generate, check=True)
for name in ["Arrival", "A long synthetic title that wraps on small screens", "Gamma"]:
    os.link(media / "Layout Example.mp4", media / (name + ".mp4"))
(media / "Layout Example.en.srt").write_text("1\n00:00:00,000 --> 00:00:07,500\nSynthetic caption.\n")
os.link(media / "Layout Example.en.srt", media / "Layout Example.fr.srt")
results = {}
initial_diff_hash = hashlib.sha256(subprocess.check_output(["git", "diff", "HEAD"], cwd=root)).hexdigest()
initial_scripts = {name: hashlib.sha256((root / "scripts/testing" / name).read_bytes()).hexdigest()
                   for name in ["test-layout-stability-local.py", "layout-stability-local.mjs", "layout-stability-flows.mjs"]}
settings = {key: value for key, value in os.environ.items() if key.startswith("KINOSAIL_LAYOUT_")}
def write_receipt():
    final_revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
    receipt = {"revision": revision, "command": "python3 scripts/testing/test-layout-stability-local.py",
        "diffSHA256": hashlib.sha256(subprocess.check_output(["git", "diff", "HEAD"], cwd=root)).hexdigest(),
        "initialDiffSHA256": initial_diff_hash, "scripts": initial_scripts, "settings": settings,
        "finalRevision": final_revision,
        "sourceDrift": final_revision != revision or initial_diff_hash != hashlib.sha256(subprocess.check_output(["git", "diff", "HEAD"], cwd=root)).hexdigest(),
        "results": results, "mediaCommand": generate,
        "browser": os.environ.get("KINOSAIL_LAYOUT_BROWSER", "chromium"),
        "environment": f"{platform.system()} {platform.machine()}; native Go servers; supported loopback HTTP",
        "boundaries": "Synthetic media/account; delayed real responses; no production, container, TLS or physical devices"}
    (run / "receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")
atexit.register(write_receipt)
for app in os.environ.get("KINOSAIL_LAYOUT_APPS", "player,subtitles").split(","):
    app_run = run / app
    app_run.mkdir()
    binary = app_run / "server"
    results[app] = "not_completed"
    env = dict(os.environ, GOCACHE="/tmp/kinosail-apple-go-cache", GOMAXPROCS="4")
    build = ["go", "-C", f"apps/{app}", "build", "-p=1", "-o", str(binary), "./cmd/kinosail"]
    subprocess.run(build, cwd=root, env=env, check=True)
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]
    url = f"http://localhost:{port}"
    env.update(KINOSAIL_LISTEN=f"127.0.0.1:{port}", KINOSAIL_AUTH_URL=url, KINOSAIL_TLS_ENABLED="false",
               KINOSAIL_DATA_DIR=str(app_run / "config"), KINOSAIL_MEDIA_DIR=str(media),
               KINOSAIL_CACHE_DIR=str(app_run / "cache"), KINOSAIL_BACKUP_DIR=str(app_run / "backups"),
               KINOSAIL_BACKUP_KEY="synthetic-layout-key", KINOSAIL_E2E_URL=url,
               KINOSAIL_LAYOUT_APP=app, KINOSAIL_LAYOUT_RUN=str(app_run), KINOSAIL_TEST_REVISION=revision)
    browser = ["node", "scripts/testing/layout-stability-local.mjs"]
    result = None
    with (app_run / "server.log").open("w") as log:
        server = subprocess.Popen([str(binary)], env=env, stdout=log, stderr=log)
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
            setup = urllib.request.Request(url + "/api/v1/setup", data=json.dumps({"name": "Owner",
                "password": "synthetic-layout-password", "device": "Disposable layout audit", "totp": True}).encode(),
                headers={"Content-Type": "application/json"})
            with urllib.request.urlopen(setup) as response:
                created = json.load(response)
                token, secret = created["token"], created["totp"]["secret"]
            digest = hmac.new(base64.b32decode(secret), int(time.time() // 30).to_bytes(8, "big"), hashlib.sha1).digest()
            offset = digest[-1] & 15
            code = str((int.from_bytes(digest[offset:offset + 4], "big") & 0x7fffffff) % 1000000).zfill(6)
            confirm = urllib.request.Request(url + "/api/v1/me/mfa", method="PUT",
                data=json.dumps({"code": code}).encode(), headers={"Content-Type": "application/json", "Authorization": "Bearer " + token})
            with urllib.request.urlopen(confirm):
                pass
            env["KINOSAIL_LAYOUT_TOTP"] = secret
            disable = urllib.request.Request(url + "/api/v1/settings/onboarding", method="PUT",
                data=b'{"enabled":false}', headers={"Content-Type": "application/json", "Authorization": "Bearer " + token})
            with urllib.request.urlopen(disable):
                pass
            with (app_run / "browser.log").open("w") as browser_log:
                result = subprocess.run(browser, cwd=root, env=env, stdout=browser_log, stderr=subprocess.STDOUT).returncode
        finally:
            server.terminate()
            try:
                server.wait(timeout=10)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait()
            binary.unlink(missing_ok=True)
    results[app] = result
    binary.unlink(missing_ok=True)
    print(f"{app}: exit {result}, artifacts {app_run}", flush=True)
print(f"E2E artifact: {run}", flush=True)
raise SystemExit(0 if all(value == 0 for value in results.values()) else 1)
