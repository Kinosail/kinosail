import json
import secrets
import sys
import urllib.parse
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PORT = int(sys.argv[1])
PLEX = sys.argv[2]
TOKEN = "kinosail-fixture-plex-token"
pins = {}

AUTH_PAGE = """<!doctype html><meta name=viewport content="width=device-width"><title>Plex fixture sign-in</title>
<body style="font:20px -apple-system;padding:40px;background:#111;color:#eee"><h1>Plex fixture</h1><p id=state>Signing in…</p>
<script>
const params = new URLSearchParams(location.hash.replace(/^#\\??/, ""));
fetch("/link?code=" + encodeURIComponent(params.get("code") || ""), {method: "POST"}).then(r => {
  document.getElementById("state").textContent = r.ok ? "Signed in. You can close this page." : "Code not found.";
  const forward = params.get("forwardUrl");
  if (r.ok && forward) setTimeout(() => { location.href = forward; }, 400);
});
</script>"""


def machine():
    request = urllib.request.Request(PLEX + "/identity", headers={"Accept": "application/json"})
    with urllib.request.urlopen(request, timeout=10) as response:
        return json.load(response)["MediaContainer"]["machineIdentifier"]


class Handler(BaseHTTPRequestHandler):
    def reply(self, status, body, kind="application/json"):
        data = body.encode() if isinstance(body, str) else json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", kind)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def authorized(self):
        return self.headers.get("X-Plex-Token") == TOKEN

    def do_POST(self):
        url = urllib.parse.urlparse(self.path)
        query = urllib.parse.parse_qs(url.query)
        if url.path == "/api/v2/pins":
            pin = {"id": len(pins) + 4100, "code": "".join(secrets.choice("ABCDEFGHJKLMNPQRSTUVWXYZ23456789") for _ in range(4)),
                   "authToken": None, "expiresIn": 900}
            pins[pin["id"]] = pin
            return self.reply(201, pin)
        if url.path == "/link":
            code = (query.get("code") or [""])[0].upper()
            for pin in pins.values():
                if pin["code"] == code:
                    pin["authToken"] = TOKEN
                    return self.reply(200, {"linked": True})
            return self.reply(404, {"error": "unknown code"})
        self.reply(404, {"error": "not found"})

    def do_GET(self):
        url = urllib.parse.urlparse(self.path)
        if url.path == "/auth":
            return self.reply(200, AUTH_PAGE, "text/html; charset=utf-8")
        if url.path.startswith("/api/v2/pins/"):
            pin = pins.get(int(url.path.rsplit("/", 1)[1]) if url.path.rsplit("/", 1)[1].isdigit() else -1)
            return self.reply(200, pin) if pin else self.reply(404, {"error": "not found"})
        if not self.authorized():
            return self.reply(401, {"error": "unauthorized"})
        if url.path == "/api/v2/user":
            return self.reply(200, {"id": 1001, "uuid": "fixture-user-1001", "username": "kino", "title": "kino"})
        if url.path == "/api/v2/resources":
            return self.reply(200, [
                {"name": "Kino Plex", "product": "Plex Media Server", "provides": "server", "clientIdentifier": machine(),
                 "owned": True, "accessToken": TOKEN, "connections": [
                     {"protocol": "https", "address": "203.0.113.9", "port": 32400, "uri": "https://203-0-113-9.fixture.plex.direct:32400",
                      "local": False, "relay": False},
                     {"protocol": "http", "address": "127.0.0.1", "port": 32400, "uri": PLEX, "local": True, "relay": False}]},
                {"name": "Living Room TV", "product": "Plex for Apple TV", "provides": "client,player", "clientIdentifier": "fixture-player",
                 "owned": True, "connections": []}])
        self.reply(404, {"error": "not found"})

    def log_message(self, format, *args):
        sys.stderr.write("plex.tv fixture %s %s\n" % (self.command, urllib.parse.urlparse(self.path).path))


ThreadingHTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
