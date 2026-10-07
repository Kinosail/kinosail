"""Disposable loopback fixture for Kinosail-iOS-Touch, using a generated MP4."""
import json
import math
import re
import sys
import time
import unicodedata
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlsplit

def read_movie(args):
    if len(args) != 1 or not args[0] or len(args[0]) > 4096:
        raise ValueError("One media path is required")
    path = Path(args[0]).resolve(strict=True)
    cap = 32 * 1024 * 1024
    if not path.is_file() or not 0 < path.stat().st_size <= cap:
        raise ValueError("Invalid fixture media")
    with path.open("rb") as stream:
        data = stream.read(cap + 1)
    if not 0 < len(data) <= cap:
        raise ValueError("Invalid fixture media")
    return data


try:
    MOVIE = read_movie(sys.argv[1:])
except (ValueError, OSError):
    raise SystemExit("Supply one regular, nonempty media file of at most 32 MiB.")
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


def text(value, cap, empty=False):
    if not isinstance(value, str) or len(value.encode("utf-8")) > cap or (not empty and not value.strip()) or any(unicodedata.category(c) in ("Cc", "Cf") for c in value):
        raise ValueError("Invalid text")
    return value


def fields(value, keys):
    if not isinstance(value, dict) or set(value) != set(keys):
        raise ValueError("Invalid fields")


def query_values(raw, allowed):
    if len(raw) > 2048 or re.search(r"%(?![0-9a-fA-F]{2})", raw):
        raise ValueError("Invalid query")
    query = parse_qs(raw, keep_blank_values=True, strict_parsing=True, max_num_fields=len(allowed), errors="strict")
    if not set(query) <= set(allowed) or any(len(values) != 1 for values in query.values()):
        raise ValueError("Invalid query fields")
    return {key: values[0] for key, values in query.items()}


def integer(value, low, high):
    if not isinstance(value, str) or not re.fullmatch(r"[0-9]{1,16}", value) or not low <= int(value) <= high:
        raise ValueError("Invalid integer")
    return int(value)


def progress(value, required):
    fields(value, ["seconds", "watched", "session", "revision"])
    seconds, revision = value["seconds"], value["revision"]
    if type(seconds) not in (int, float) or not 0 <= seconds <= 31536000 or not math.isfinite(seconds):
        raise ValueError("Invalid time")
    if type(value["watched"]) is not bool or type(revision) is not int or not int(required) <= revision <= 9007199254740991:
        raise ValueError("Invalid progress")
    text(value["session"], 128, empty=not required)


def media_range(value, size):
    match = re.fullmatch(r"bytes=([0-9]{0,20})-([0-9]{0,20})", value)
    if not match or not any(match.groups()):
        raise ValueError("Invalid range")
    first, last = match.groups()
    if not first:
        if int(last) == 0:
            raise ValueError("Invalid suffix")
        return max(0, size - int(last)), size - 1
    start, end = int(first), int(last) if last else size - 1
    if start >= size or end < start:
        raise ValueError("Unsatisfiable range")
    return start, min(end, size - 1)


def unique_object(pairs):
    value = dict(pairs)
    if len(value) != len(pairs):
        raise ValueError("Duplicate JSON field")
    return value


def invalid_constant(_):
    raise ValueError("Nonfinite JSON number")


class Handler(BaseHTTPRequestHandler):
    def read_json(self):
        lengths = self.headers.get_all("Content-Length", [])
        if len(lengths) != 1 or self.headers.get_all("Transfer-Encoding", []):
            raise ValueError("Invalid framing")
        length = integer(lengths[0], 1, 16384)
        self.connection.settimeout(5)
        raw = self.rfile.read(length)
        if len(raw) != length:
            raise ValueError("Truncated body")
        value = json.loads(raw.decode("utf-8"), object_pairs_hook=unique_object, parse_constant=invalid_constant)
        if not isinstance(value, dict):
            raise ValueError("Invalid JSON object")
        return value

    def authorized(self):
        return self.headers.get_all("Authorization", []) == ["Bearer " + TOKEN]

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
        if self.path not in ("/api/v1/quick-connect", "/api/v1/quick-connect/token"):
            return self.send(404, dict(error="Unsupported fixture operation"))
        try:
            body = self.read_json()
            if self.path == "/api/v1/quick-connect":
                fields(body, ["device"])
                text(body["device"], 80)
                return self.send(201, dict(code="123456", secret=TOKEN))
            fields(body, ["secret"])
            if body["secret"] != TOKEN:
                raise ValueError("Invalid fixture secret")
            return self.send(201, dict(token=TOKEN, expiresIn=3600))
        except (ValueError, OSError, RecursionError):
            self.send(400, dict(error="Invalid fixture request"))

    def do_GET(self):
        try:
            if len(self.path) > 4096 or not self.path.startswith("/"):
                raise ValueError("Invalid target")
            parsed = urlsplit(self.path)
            if parsed.fragment:
                raise ValueError("Unexpected fragment")
        except ValueError:
            return self.send(400, dict(error="Invalid fixture target"))
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
            if mode not in ("pending", "preparing", "failed", "loaded"):
                return self.send(400, dict(error="Unknown fixture mode"))
            STATE["mode"] = mode
            return self.send(200, STATE)
        if not self.authorized():
            return self.send(401, dict(error="Fixture token required"))
        try:
            allowed = {"/api/v1/library": ["q", "view", "sort", "offset", "limit"],
                       "/api/v1/items/loading-video/playback": ["videoCodecs", "audioCodecs", "hdrFormats", "maxAudioChannels"],
                       "/api/v1/items/loading-video/bookmarks": ["includeOffset"]}.get(path, [])
            query = query_values(parsed.query, allowed)
            if path == "/api/v1/library":
                text(query.get("q", ""), 256, empty=True)
                if query.get("view", "all") not in ("all", "movies", "shows", "unwatched", "list", "music", "audiobooks", "books", "photos", "history") or query.get("sort", "title") not in ("title", "added", "year"):
                    raise ValueError("Invalid library selection")
                offset = integer(query.get("offset", "0"), 0, 1000000)
                limit = integer(query.get("limit", "60"), 1, 200)
            if path == "/api/v1/items/loading-video/playback":
                fields(query, allowed)
                for key, choices in [("videoCodecs", {"h264", "hevc", "av1"}), ("audioCodecs", {"aac", "mp3", "ac3", "eac3"}), ("hdrFormats", {"sdr", "hdr10", "hlg"})]:
                    values = query[key].split(",")
                    if len(values) != len(set(values)) or not set(values) <= choices:
                        raise ValueError("Invalid capabilities")
                integer(query["maxAudioChannels"], 1, 8)
            if "includeOffset" in query and query["includeOffset"] not in ("true", "false"):
                raise ValueError("Invalid bookmark query")
        except (ValueError, UnicodeError):
            return self.send(400, dict(error="Invalid fixture query"))
        if path == "/api/v1/me":
            return self.send(200, VIEWER)
        if path == "/api/v1/library":
            view = query.get("view", "all")
            items = [ITEM] if view in ("all", "movies") else []
            return self.send(200, dict(items=items, total=len(items),
                                      offset=offset, limit=limit, letters=[]))
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
        if path == "/api/v1/items/loading-video/playback-preferences":
            return self.send(200, dict(playback=PLAY, overridden=False))
        if path == "/api/v1/items/loading-video/watch-progress":
            return self.send(200, dict(seconds=0, duration=60))
        if path == "/api/v1/items/loading-video/bookmarks":
            return self.send(200, dict(bookmarks=[]))
        if path == "/media/loading-video":
            ranges = self.headers.get_all("Range", [])
            try:
                if len(ranges) > 1:
                    raise ValueError("Ambiguous range")
                start, end = media_range(ranges[0], len(MOVIE)) if ranges else (0, len(MOVIE) - 1)
            except ValueError:
                return self.send(416, dict(error="Invalid fixture range"), headers={"Content-Range": f"bytes */{len(MOVIE)}"})
            if STATE["mode"] == "preparing":
                time.sleep(30)
            data, headers, status = MOVIE, {"Accept-Ranges": "bytes"}, 200
            if ranges:
                headers["Content-Range"] = f"bytes {start}-{end}/{len(data)}"
                data, status = data[start:end + 1], 206
            return self.send(status, data, "video/mp4", headers)
        self.send(404, dict(error="Unsupported fixture operation"))

    def do_PUT(self):
        if self.path != "/api/v1/items/loading-video/progress/sync":
            return self.send(404, dict(error="Unsupported fixture operation"))
        if not self.authorized():
            return self.send(401, dict(error="Fixture token required"))
        try:
            body = self.read_json()
            fields(body, ["progress", "expected", "playbackToken"])
            progress(body["progress"], required=True)
            progress(body["expected"], required=False)
            text(body["playbackToken"], 8192, empty=True)
            self.send(200, body["progress"])
        except (ValueError, OSError, RecursionError):
            self.send(400, dict(error="Invalid fixture request"))


print("Disposable iOS playback fixture on 127.0.0.1:4281", flush=True)
ThreadingHTTPServer(("127.0.0.1", 4281), Handler).serve_forever()
