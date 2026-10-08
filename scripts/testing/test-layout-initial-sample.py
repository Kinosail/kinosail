#!/usr/bin/env python3
"""Exercise the actual layout CLI with stale samples and a real late movement."""
import argparse
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import subprocess
import threading
import zipfile
from urllib.parse import urlsplit

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--fixtures", type=Path, required=True,
                    help="TestWriteUIStateFixturesSubtitleInspector output")
parser.add_argument("--output", type=Path, required=True)
parser.add_argument("--browser", choices=["chromium", "firefox", "webkit"], default="webkit")
args = parser.parse_args()
root = Path(__file__).resolve().parents[2]
assets = {name: (args.fixtures / name).read_bytes() for name in ["subtitle-inspector.html",
          "subtitle-inspector.json", "app.css", "subtitle-inspector.css", "subtitle-inspector.js",
          "manrope.woff2", "icon.svg", "cinema-backdrop.jpg"]}
dock = (root / "apps/subtitles/internal/server/static/subtitle-dock-initial.js").read_text()
args.output.mkdir(parents=True, exist_ok=False)
revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
receipt = {"revision": revision, "command": ["python3", str(Path(__file__).relative_to(root)),
    "--fixtures", str(args.fixtures), "--output", str(args.output), "--browser", args.browser],
    "environment": "Native browser; frozen Go-rendered inspector, delivered CSS and dock initializer; disposable HTTP peer",
    "data": "Two installed Arrival cues; 390px at 200 percent text; delayed audit callback after dock initialization",
    "boundaries": "Login and unrelated bundle are fixture placeholders; no live authentication, media or provider proof",
    "sources": {name: hashlib.sha256((root / name).read_bytes()).hexdigest() for name in [
        "scripts/testing/layout-stability-local.mjs", "scripts/testing/test-layout-initial-sample.py",
        "apps/subtitles/internal/server/static/subtitle-dock-initial.js"]},
    "assets": {name: hashlib.sha256(content).hexdigest() for name, content in assets.items()}, "results": []}

class Peer(BaseHTTPRequestHandler):
    late = False
    missing_login_name = False

    def log_message(self, *_args):
        pass

    def respond(self, content, content_type="text/html", status=200):
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(content)))
        self.end_headers()
        self.wfile.write(content)

    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length", "0")))
        self.send_response(303)
        self.send_header("Location", "/")
        self.end_headers()

    def do_GET(self):
        path = urlsplit(self.path).path
        if path == "/login":
            label = "Missing name control" if self.missing_login_name else "Name"
            self.respond((f'<form method="post"><label>{label}<input name="name"></label>'
                          '<label>Password<input name="password" type="password"></label>'
                          '<button>Sign in</button></form>').encode())
        elif path in ["/api/v1/library", "/api/v1/subtitle-library"]:
            self.respond(b'{"items":[{"id":"fixture","title":"Layout Example"}]}', "application/json")
        elif path.endswith("/inspect"):
            self.respond(assets["subtitle-inspector.json"], "application/json")
        elif path.endswith("/draft"):
            self.respond(b'{"state":"idle","words":[]}', "application/json")
        elif path == "/static/theme.js":
            # Delay only the audit's next sample. Initialize the production dock
            # after the first recorded frame; leave real geometry and time intact.
            script = """const nativeFrame = requestAnimationFrame;
let initialized = false, delayed = false;
window.requestAnimationFrame = callback => {
  if (callback.name !== 'sample') return nativeFrame(callback);
  return nativeFrame(time => {
    if (initialized && !delayed) {
      delayed = true; setTimeout(() => nativeFrame(callback), 450); return;
    }
    callback(time);
    if (!initialized && window.layoutAudit?.frames.some(frame => frame.boxes.length)) {
      initialized = true;
      DOCK_INITIALIZER
    }
  });
};
if (matchMedia('(max-width:900px)').matches) document.documentElement.style.fontSize = '200%';
""".replace("DOCK_INITIALIZER", dock)
            if self.late:
                script += "setTimeout(() => { document.querySelector('main').style.paddingTop = '64px'; }, 1800);"
            self.respond(script.encode(), "text/javascript")
        elif path.startswith("/static/"):
            name = path.rsplit("/", 1)[-1]
            types = {"css": "text/css", "js": "text/javascript", "woff2": "font/woff2",
                     "svg": "image/svg+xml", "jpg": "image/jpeg"}
            self.respond(assets.get(name, b""), types.get(name.rsplit(".", 1)[-1], "text/plain"))
        elif path.startswith("/media/"):
            self.respond(b"", "text/plain", 404)
        else:
            self.respond(assets["subtitle-inspector.html"])

try:
    for mode, expected in [("missing-login-name", 1), ("stale-sample", 0), ("late-movement", 1)]:
        Peer.late = mode == "late-movement"
        Peer.missing_login_name = mode == "missing-login-name"
        server = ThreadingHTTPServer(("127.0.0.1", 0), Peer)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        run = args.output / mode
        run.mkdir()
        env = dict(os.environ, KINOSAIL_E2E_URL=f"http://127.0.0.1:{server.server_port}",
            KINOSAIL_LAYOUT_APP="subtitles", KINOSAIL_LAYOUT_RUN=str(run),
            KINOSAIL_LAYOUT_BROWSER=args.browser, KINOSAIL_LAYOUT_TOTP="JBSWY3DPEHPK3PXP",
            KINOSAIL_LAYOUT_PATHS="/subtitles/inspect/fixture?language=en", KINOSAIL_LAYOUT_QUICK="1",
            KINOSAIL_LAYOUT_ENFORCE="1", KINOSAIL_TEST_REVISION=revision)
        for key in ["KINOSAIL_LAYOUT_VARIANTS", "KINOSAIL_LAYOUT_FLOWS", "KINOSAIL_LAYOUT_APPLE_SHIM"]:
            env.pop(key, None)
        try:
            with (run / "browser.log").open("w") as log:
                code = subprocess.run(["node", "scripts/testing/layout-stability-local.mjs"],
                    cwd=root, env=env, stdout=log, stderr=subprocess.STDOUT, timeout=90).returncode
            if mode == "missing-login-name":
                failure = json.loads((run / "failure.json").read_text())
                trace = run / "failure-trace.zip"
                receipt["results"].append({"mode": mode, "exitCode": code, "expectedExitCode": expected,
                    "stage": failure["stage"], "traceBytes": trace.stat().st_size if trace.exists() else 0})
                assert code == expected and failure["stage"] == "login-name", "Missing login control must fail"
                assert failure["completedCases"] == 0 and failure["loginResponses"] == [], "No authentication or measurement may occur"
                assert len(failure["loginNavigation"]) == 1 and failure["loginNavigation"][0]["status"] == 200
                assert failure["loginDocument"]["urlMatchesExpected"] and failure["loginDocument"]["state"] == "complete"
                assert failure["loginDocument"]["nameEditable"] and failure["loginDocument"]["passwordEditable"]
                assert failure["loginDocument"]["nameLabelMatches"] is False, "The real incorrect label must remain observable"
                with zipfile.ZipFile(trace) as archive:
                    assert archive.testzip() is None and any(name.endswith(".trace") for name in archive.namelist())
                continue
            reports = json.loads((run / "measurements.json").read_text())["reports"]
            moved = sum(len(report["moved"]) for report in reports)
            receipt["results"].append({"mode": mode, "exitCode": code, "expectedExitCode": expected,
                "cases": len(reports), "moved": moved})
            assert len(reports) == 2, f"{mode}: incomplete audit"
            assert code == expected, f"{mode}: exit {code}, expected {expected}"
            assert (moved > 0) == (mode == "late-movement"), f"{mode}: movement control failed"
        finally:
            server.shutdown()
            server.server_close()
            thread.join()
finally:
    (args.output / "receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")
