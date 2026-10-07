"""Disposable loopback fixture for Kinosail-iOS-Touch, using a generated MP4."""
import json
import sys
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlsplit

MOVIE = Path(sys.argv[1]).read_bytes()
STATE = {"mode": "pending"}
TOKEN = "ios-playback-fixture-token"
PROGRESS = dict(seconds=0, watched=False, session="", revision=0)
ITEM = dict(id="loading-video", kind="video", title="Playback regression", year="2026",
            stream="/media/loading-video", container="mp4", progress=PROGRESS)
PLAY = dict(rate=1, audioLanguage="auto", subtitleLanguage="auto", audioTrack="",
            subtitleTrack="", nightMode=False, dialogueBoost=False, volumeBoost=1)
VIEWER = dict(server="Disposable playback fixture", serverId="ios-playback-fixture",
              viewer=dict(id="qa", name="QA Viewer", owner=True, downloads=True,
                          transcode=True, remote=False))


class Handler(BaseHTTPRequestHandler):
    def send(self, status, value=None, kind="application/json", headers=None):
        data = json.dumps(value).encode() if kind == "application/json" else value
        self.send_response(status)
        self.send_header("Content-Type", kind)
        self.send_header("Content-Length", str(len(data)))
        self.send_header("Cache-Control", "no-store")
        for key, value in (headers or {}).items():
            self.send_header(key, value)
        self.end_headers()
        try:
            self.wfile.write(data)
        except (BrokenPipeError, ConnectionResetError):
            pass

    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length", "0")))
        if self.path == "/api/v1/quick-connect":
            return self.send(201, dict(code="123456", secret=TOKEN))
        if self.path == "/api/v1/quick-connect/token":
            return self.send(201, dict(token=TOKEN, expiresIn=3600))
        self.send(404, dict(error="Unsupported fixture operation"))

    def do_GET(self):
        parsed = urlsplit(self.path)
        path = parsed.path
        if path == "/qa/state":
            if len(parsed.query) > 128:
                return self.send(400, dict(error="Fixture state query is too large"))
            try:
                query = parse_qs(parsed.query, keep_blank_values=True, strict_parsing=True, max_num_fields=1)
            except ValueError:
                return self.send(400, dict(error="Invalid fixture state query"))
            if set(query) != {"mode"} or len(query["mode"]) != 1:
                return self.send(400, dict(error="One fixture mode is required"))
            mode = query["mode"][0]
            if mode not in ("pending", "failed", "loaded"):
                return self.send(400, dict(error="Unknown fixture mode"))
            STATE["mode"] = mode
            return self.send(200, STATE)
        query = parse_qs(parsed.query)
        if self.headers.get("Authorization") != "Bearer " + TOKEN:
            return self.send(401, dict(error="Fixture token required"))
        if path == "/api/v1/me":
            return self.send(200, VIEWER)
        if path == "/api/v1/library":
            view = query.get("view", ["all"])[0]
            items = [ITEM] if view in ("all", "movies") else []
            return self.send(200, dict(items=items, total=len(items),
                                      offset=int(query.get("offset", ["0"])[0]),
                                      limit=int(query.get("limit", ["60"])[0]), letters=[]))
        if path == "/api/v1/me/media-preferences":
            return self.send(200, dict(playback=PLAY, autoDownloadNext=0, removeWatched=False,
                                      downloadLimitGiB=20, wifiOnly=True, readerFontSize=20, readerTheme="auto"))
        if path == "/api/v1/items/loading-video":
            return self.send(200, dict(item=ITEM, listed=False, profileId="qa"))
        if path == "/api/v1/items/loading-video/playback":
            mode = STATE["mode"]
            if mode == "pending":
                time.sleep(30)
            if mode == "failed":
                return self.send(500, dict(error="Simulated playback failure"))
            return self.send(200, dict(media=dict(kind="video", duration=60),
                                      plan=dict(allowed=True, mode="direct", reason="compatible"),
                                      directAllowed=True, direct="/media/loading-video", directType="video/mp4",
                                      duration=60, start=0, chapters=[], subtitles=[]))
        if path.endswith("/playback-preferences"):
            return self.send(200, dict(playback=PLAY, overridden=False))
        if path.endswith("/watch-progress"):
            return self.send(200, dict(seconds=0, duration=60))
        if path.endswith("/bookmarks"):
            return self.send(200, dict(bookmarks=[]))
        if path == "/media/loading-video":
            data, headers, status = MOVIE, {"Accept-Ranges": "bytes"}, 200
            if self.headers.get("Range"):
                start, end = self.headers["Range"].removeprefix("bytes=").split("-")
                start, end = int(start), min(int(end) if end else len(data) - 1, len(data) - 1)
                headers["Content-Range"] = f"bytes {start}-{end}/{len(data)}"
                data, status = data[start:end + 1], 206
            return self.send(status, data, "video/mp4", headers)
        self.send(404, dict(error="Unsupported fixture operation"))

    def do_PUT(self):
        body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", "0"))) or "{}")
        self.send(200, body.get("progress", {}))


print("Disposable iOS playback fixture on 127.0.0.1:4281", flush=True)
ThreadingHTTPServer(("127.0.0.1", 4281), Handler).serve_forever()
