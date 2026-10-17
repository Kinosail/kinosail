#!/usr/bin/env python3
"""Local E2E: Library loading state while a real first scan is delayed by strace."""
import base64
import hashlib
import hmac
import json
import os
import platform
import shutil
import signal
import socket
import subprocess
import sys
import time
import urllib.request
from pathlib import Path

root = Path(__file__).resolve().parents[2]
run = root / ".verification/library-loading" / time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())
run.mkdir(parents=True)
revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
diff_hash = hashlib.sha256(subprocess.check_output(["git", "diff", "HEAD"], cwd=root)).hexdigest()
for tool in ["strace", "ffmpeg", "node"]:
    if shutil.which(tool) is None:
        raise SystemExit(f"{tool} is required")
media = run / "media"
(media / "Slow").mkdir(parents=True)
generate = ["ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=640x360:r=24:d=4",
            "-c:v", "libx264", "-preset", "ultrafast", "-crf", "35", "-pix_fmt", "yuv420p", "-movflags", "+faststart",
            str(media / "Arrival (2016).mp4")]
subprocess.run(generate, check=True)
for name in ["Gamma (2019).mp4", "A long synthetic title that wraps on small screens (2020).mp4"]:
    os.link(media / "Arrival (2016).mp4", media / name)
os.link(media / "Arrival (2016).mp4", media / "Slow" / "Delayed (2021).mp4")
delays = {"loading": int(os.environ.get("KINOSAIL_LOADING_DELAY_SECONDS", "25")), "failed": int(os.environ.get("KINOSAIL_FAILED_DELAY_SECONDS", "15"))}
results, timings = {}, {}


def free_port():
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return listener.getsockname()[1]


def totp(secret):
    digest = hmac.new(base64.b32decode(secret), int(time.time() // 30).to_bytes(8, "big"), hashlib.sha1).digest()
    offset = digest[-1] & 15
    return str((int.from_bytes(digest[offset:offset + 4], "big") & 0x7fffffff) % 1000000).zfill(6)


def call(url, path, method="GET", body=None, token=""):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    request = urllib.request.Request(url + path, method=method, data=None if body is None else json.dumps(body).encode(), headers=headers)
    with urllib.request.urlopen(request, timeout=10) as response:
        return json.load(response) if response.headers.get("Content-Type", "").startswith("application/json") and response.length != 0 else None


class Server:
    def __init__(self, app_run, binary, env, phase, fault):
        self.log = (app_run / f"server-{phase}.log").open("w")
        command = [str(binary)]
        if fault:
            command = ["strace", "-f", "--seccomp-bpf", "-qq", "-o", str(app_run / f"strace-{phase}.log"), "-e", "trace=openat",
                       "-e", "inject=openat:" + fault[0], "-P", str(fault[1]), str(binary)]
        self.command = command
        self.started = time.monotonic()
        self.process = subprocess.Popen(command, env=env, stdout=self.log, stderr=self.log, start_new_session=True)

    def wait_healthy(self, url):
        while time.monotonic() - self.started < 60:
            if self.process.poll() is not None:
                raise RuntimeError("Server exited before readiness")
            try:
                with urllib.request.urlopen(url + "/healthz", timeout=1) as response:
                    if json.load(response) == {"status": "ok"}:
                        return round(time.monotonic() - self.started, 3)
            except (OSError, ValueError):
                time.sleep(0.1)
        raise RuntimeError("Server did not become healthy")

    def stop(self):
        try:
            os.killpg(self.process.pid, signal.SIGTERM)
            self.process.wait(timeout=15)
        except subprocess.TimeoutExpired:
            os.killpg(self.process.pid, signal.SIGKILL)
            self.process.wait()
        except ProcessLookupError:
            pass
        self.log.close()


def write_receipt():
    files = {}
    for path in sorted(run.rglob("*")):
        if path.is_file() and path.suffix in {".png", ".json", ".log"} and path.name != "receipt.json":
            files[str(path.relative_to(run))] = hashlib.sha256(path.read_bytes()).hexdigest()
    receipt = {"revision": revision, "diffSHA256": diff_hash, "command": "python3 scripts/testing/test-library-loading-local.py",
               "mediaCommand": generate, "delays": delays, "timings": timings, "results": results, "files": files,
               "browser": "chromium", "environment": f"{platform.system()} {platform.machine()}; native Go servers; loopback HTTP",
               "faults": "strace openat delay on Media/Slow (loading) and openat EIO after delay on the media root (failed)",
               "boundaries": "Synthetic media and account; no container, TLS, production deployment, physical device or native client"}
    (run / "receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")


try:
    for app in os.environ.get("KINOSAIL_LOADING_APPS", "player,subtitles").split(","):
        app_run = run / app
        app_run.mkdir()
        binary = app_run / "server"
        env = dict(os.environ, GOMAXPROCS="4")
        subprocess.run(["go", "-C", f"apps/{app}", "build", "-o", str(binary), "./cmd/kinosail"], cwd=root, env=env, check=True)
        port = free_port()
        url = f"http://localhost:{port}"
        env.update(KINOSAIL_LISTEN=f"127.0.0.1:{port}", KINOSAIL_AUTH_URL=url, KINOSAIL_TLS_ENABLED="false",
                   KINOSAIL_DATA_DIR=str(app_run / "config"), KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(app_run / "cache"),
                   KINOSAIL_BACKUP_DIR=str(app_run / "backups"), KINOSAIL_BACKUP_KEY="synthetic-loading-key")
        server = Server(app_run, binary, env, "setup", None)
        try:
            server.wait_healthy(url)
            created = call(url, "/api/v1/setup", "POST", {"name": "Owner", "password": "synthetic-loading-password", "device": "Library loading E2E", "totp": True})
            token = created["token"]
            call(url, "/api/v1/me/mfa", "PUT", {"code": totp(created["totp"]["secret"])}, token)
            call(url, "/api/v1/settings/onboarding", "PUT", {"enabled": False}, token)
        finally:
            server.stop()
        results[app] = {}
        for phase, fault in [("loading", (f"delay_enter={delays['loading'] * 1_000_000}", media / "Slow")),
                             ("failed", (f"error=EIO:delay_enter={delays['failed'] * 1_000_000}", media))]:
            server = Server(app_run, binary, env, phase, fault)
            try:
                timings[f"{app}-{phase}-health-seconds"] = server.wait_healthy(url)
                driver = dict(env, KINOSAIL_E2E_URL=url, KINOSAIL_LOADING_TOKEN=token, KINOSAIL_LOADING_APP=app,
                              KINOSAIL_LOADING_PHASE=phase, KINOSAIL_LOADING_RUN=str(app_run), KINOSAIL_TEST_REVISION=revision)
                with (app_run / f"browser-{phase}.log").open("w") as log:
                    results[app][phase] = subprocess.run(["node", "scripts/testing/library-loading-local.mjs"], cwd=root, env=driver,
                                                         stdout=log, stderr=subprocess.STDOUT).returncode
            finally:
                server.stop()
            print(f"{app} {phase}: exit {results[app][phase]}", flush=True)
        binary.unlink(missing_ok=True)
finally:
    write_receipt()
print(f"E2E artifact: {run}", flush=True)
sys.exit(0 if results and all(code == 0 for phases in results.values() for code in phases.values()) else 1)
