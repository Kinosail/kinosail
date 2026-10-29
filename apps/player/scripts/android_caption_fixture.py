"""Owned loopback Server and bounded media fault bridge for Android UI journeys."""
import base64
from contextlib import contextmanager
import hashlib
import hmac
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import http.client
import json
import os
from pathlib import Path
import re
import shutil
import struct
import subprocess
import threading
import time
from urllib.parse import parse_qsl, urlencode, urlsplit
from hls_timeline_http import PublicServer


def target(method, raw):
    if len(raw) > 4096 or any(ord(char) < 33 or ord(char) > 126 for char in raw):
        raise ValueError()
    url = urlsplit(raw)
    if url.scheme or url.netloc or url.fragment:
        raise ValueError()
    path = url.path
    static = {"/api/v1/me", "/api/v1/library", "/api/v1/cast/config", "/api/v1/remote-players"}
    if path in static and method == "GET":
        path = next(value for value in static if value == path)
    else:
        if not re.fullmatch(r"/(api/v1/items|art|backdrop|episode-art|media|subtitle)/([A-Za-z0-9_-]{1,128})(/(?:playback|playback-preferences|progress/sync|[0-9]{1,6}|embedded/[0-9]{1,6}))?", path):
            raise ValueError()
        if method == "PUT" and (not path.startswith("/api/v1/items/") or not path.endswith(("/progress/sync", "/playback-preferences"))):
            raise ValueError()
        if method not in ("GET", "PUT"):
            raise ValueError()
    query = parse_qsl(url.query, keep_blank_values=True, strict_parsing=True, max_num_fields=16)
    allowed = {"q", "view", "sort", "offset", "limit"} if path == "/api/v1/library" else (
        {"videoCodecs", "audioCodecs", "hdrFormats", "maxAudioChannels"} if path.endswith("/playback") else
        {"variant"} if path.startswith("/art/") else set())
    if len({key for key, _ in query}) != len(query) or any(key not in allowed or len(value) > 512 for key, value in query):
        raise ValueError()
    destination = path + ("?" + urlencode(query) if query else "")
    if not re.fullmatch(r"/[A-Za-z0-9_/-]+(?:\?[A-Za-z0-9_~.%+=&-]+)?", destination):
        raise ValueError()
    return destination


class Bridge(BaseHTTPRequestHandler):
    mode = "ready"
    ready = threading.Event()
    ready.set()
    lock = threading.Lock()

    def setup(self):
        super().setup()
        self.connection.settimeout(10)

    def do_GET(self):
        self.forward()

    def do_PUT(self):
        self.forward()

    def do_POST(self):
        if self.path != "/__fixture/media" or self.headers.get("X-Fixture-Control") != "video-overlay":
            self.send_error(400)
            return
        try:
            size = int(self.headers.get("Content-Length", "0"))
            if not 0 < size <= 128:
                raise ValueError()
            value = json.loads(self.rfile.read(size))
            if set(value) != {"mode"} or value["mode"] not in {"ready", "pending", "failed"}:
                raise ValueError()
        except (ValueError, TypeError, KeyError):
            self.send_error(400)
            return
        with Bridge.lock:
            Bridge.mode = value["mode"]
            Bridge.ready.clear() if Bridge.mode == "pending" else Bridge.ready.set()
        self.send_response(204)
        self.end_headers()

    def forward(self):
        try:
            destination = target(self.command, self.path)
        except ValueError:
            self.send_error(400)
            return
        if self.path.startswith(("/media/", "/subtitle/")):
            Bridge.ready.wait(30)
            with Bridge.lock:
                mode = Bridge.mode
            if mode != "ready":
                self.send_error(404 if mode == "failed" else 503)
                return
        try:
            size = int(self.headers.get("Content-Length", "0"))
            if not 0 <= size <= 65536:
                raise ValueError()
        except ValueError:
            self.send_error(400)
            return
        upstream = http.client.HTTPConnection("127.0.0.1", 39227, timeout=30)
        try:
            body = self.rfile.read(size) if size else None
            headers = {k: v for k, v in self.headers.items() if k.lower() in
                ("authorization", "accept", "range", "if-range", "content-type", "x-kinosail-viewer-profile")}
            upstream.request(self.command, destination, body=body, headers=headers)
            response = upstream.getresponse()
            data = response.read(8388609)
            if len(data) > 8388608:
                self.send_error(502)
                return
            forwarded = {}
            for name in ("Content-Type", "Content-Range", "Accept-Ranges"):
                value = response.getheader(name)
                if value:
                    if len(value) > 1024 or "\r" in value or "\n" in value or not value.isascii() or not value.isprintable():
                        self.send_error(502)
                        return
                    forwarded[name] = value
            self.send_response(response.status)
            for name, value in forwarded.items():
                self.send_header(name, value)
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
        except (OSError, http.client.HTTPException):
            self.close_connection = True
        finally:
            upstream.close()

    def log_message(self, *args):
        pass


class OwnedBridgeServer(ThreadingHTTPServer):
    daemon_threads = False


@contextmanager
def fixture(root, run, receipt):
    media = run / "media" / "Movies"
    media.mkdir(parents=True)
    source = root / "apps/player/scripts/fixtures/android-captions"
    receipt["fixtureSHA256"] = {}
    for path in sorted(source.iterdir()):
        if not path.is_file() or path.suffix not in (".mp4", ".srt", ".nfo"):
            raise RuntimeError("unexpected_fixture_input")
        shutil.copy2(path, media / path.name)
        receipt["fixtureSHA256"][path.name] = hashlib.sha256(path.read_bytes()).hexdigest()
    env = dict(os.environ, KINOSAIL_LISTEN="127.0.0.1:39227", KINOSAIL_AUTH_URL="http://127.0.0.1:39227",
        KINOSAIL_TLS_ENABLED="false", KINOSAIL_DATA_DIR=str(run / "data"), KINOSAIL_MEDIA_DIR=str(run / "media"),
        KINOSAIL_CACHE_DIR=str(run / "cache"), KINOSAIL_BACKUP_DIR=str(run / "backups"),
        KINOSAIL_BACKUP_KEY="synthetic-android-caption-key", KINOSAIL_SUBTITLES="on")
    bridge = thread = log = server = None
    receipt["fixtureCleanupFailures"] = []
    try:
        bridge = OwnedBridgeServer(("127.0.0.1", 39231), Bridge)
        thread = threading.Thread(target=bridge.serve_forever, daemon=True)
        log = (run / "server.private.log").open("wb")
        server = subprocess.Popen([str(run / "player")], env=env, stdout=log, stderr=log)
        thread.start()
        api = PublicServer("http://127.0.0.1:39227")
        api.authorize()
        api.call("/api/v1/profiles", "POST", {"name": "Native QA Viewer", "password": "synthetic-native-qa-password",
            "libraries": ["all"], "rating": "all", "transcode": True}, 201)
        session = api.call("/api/v1/session", "POST", {"name": "Native QA Viewer",
            "password": "synthetic-native-qa-password", "device": "Hosted caption emulator"}, 201)
        api.token = session["token"]
        secret = base64.b32decode(api.call("/api/v1/me/mfa/setup", "POST", {}, 201)["secret"])
        digest = hmac.new(secret, struct.pack(">Q", int(time.time() / 30)), hashlib.sha1).digest()
        offset = digest[-1] & 15
        code = f'{(struct.unpack(">I", digest[offset:offset+4])[0] & 0x7fffffff) % 1000000:06d}'
        api.call("/api/v1/me/mfa", "PUT", {"code": code})
        deadline = time.monotonic() + 45
        while time.monotonic() < deadline:
            if len(api.call("/api/v1/library?q=01a0f2ab&limit=24")["items"]) == 2:
                break
            time.sleep(.25)
        else:
            raise RuntimeError("fixture_catalog_not_ready")
        yield api.token
    finally:
        Bridge.ready.set()
        def clean(name, action):
            try:
                action()
            except Exception as error:
                receipt["fixtureCleanupFailures"].append({"operation": name, "failureClass": type(error).__name__})
        if thread is not None and thread.is_alive():
            clean("bridge_shutdown", bridge.shutdown)
        if bridge is not None:
            clean("bridge_close", bridge.server_close)
        if thread is not None and thread.ident is not None:
            clean("bridge_join", lambda: thread.join(timeout=5))
        if server is not None:
            if server.poll() is None:
                clean("server_terminate", server.terminate)
                clean("server_wait", lambda: server.wait(timeout=10))
            if server.poll() is None:
                clean("server_kill", server.kill)
                clean("server_reap", lambda: server.wait(timeout=5))
        if log is not None:
            clean("server_log_close", log.close)
        receipt["serverSettled"] = (server is None or server.poll() is not None) and not receipt["fixtureCleanupFailures"]
        receipt["bridgeSettled"] = (thread is None or not thread.is_alive()) and not receipt["fixtureCleanupFailures"]
