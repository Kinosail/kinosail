"""Private disposable HTTP fixture helpers; receipts never include credentials."""
import base64
import hashlib
import hmac
import json
import math
import re
import struct
import time
import urllib.error
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        return None


class PublicServer:
    def __init__(self, url):
        self.url, self.token = url, ""
        self.opener = urllib.request.build_opener(NoRedirect)

    def http(self, path, method="GET", body=None, authenticated=True):
        headers = {"Content-Type": "application/json"}
        if authenticated and self.token:
            headers["Authorization"] = "Bearer " + self.token
        request = urllib.request.Request(self.url + path, method=method, headers=headers,
            data=json.dumps(body).encode() if body is not None else None)
        try:
            response = self.opener.open(request, timeout=40)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            data = response.read(8 * 1024 * 1024 + 1)
            if len(data) > 8 * 1024 * 1024:
                raise RuntimeError("public_response_oversized")
            return response.getcode(), data, dict(response.headers)

    def call(self, path, method="GET", body=None, status=200):
        actual, data, _ = self.http(path, method, body)
        if actual != status:
            raise RuntimeError("public_http_status_" + str(actual))
        if len(data) > 2 * 1024 * 1024:
            raise RuntimeError("public_response_oversized")
        return json.loads(data) if data else None

    def authorize(self):
        limit = time.monotonic() + 30
        while time.monotonic() < limit:
            try:
                if self.http("/healthz", authenticated=False)[0] == 200:
                    break
            except OSError:
                pass
            time.sleep(0.1)
        owner = self.call("/api/v1/setup", "POST",
            {"name": "Owner", "password": "synthetic-timeline-password", "totp": True}, 201)
        self.token = owner["token"]
        secret = base64.b32decode(owner["totp"]["secret"])
        value = hmac.new(secret, struct.pack(">Q", int(time.time() / 30)), hashlib.sha1).digest()
        offset = value[-1] & 15
        code = f'{(struct.unpack(">I", value[offset:offset + 4])[0] & 0x7fffffff) % 1000000:06d}'
        if self.call("/api/v1/me/mfa", "PUT", {"code": code}) != {"enabled": True}:
            raise RuntimeError("mfa_confirmation")
        if self.http("/onboarding/finish")[0] != 303:
            raise RuntimeError("onboarding_completion")


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def source_state(path):
    info = path.stat()
    return {"sizeBytes": info.st_size, "mtimeNanoseconds": str(info.st_mtime_ns), "sha256": sha(path)}


def copied_playlist_evidence(cache):
    """Bounded private cache observation; never changes the playback verdict."""
    captured = []
    try:
        for number, playlist in enumerate(cache.rglob('index.m3u8')):
            if number >= 16:
                break
            if not re.fullmatch(r'\d{2,4}p', playlist.parent.name):
                continue
            with playlist.open('rb') as handle:
                data = handle.read(65537)
            if len(data) > 65536:
                captured.append({'state': 'oversized'})
                continue
            durations = [float(value) for value in re.findall(rb'^#EXTINF:(\d+(?:\.\d+)?),', data, re.M)]
            if len(durations) > 64 or any(not math.isfinite(value) or not 0 < value <= 60 for value in durations):
                captured.append({'state': 'invalid_durations'})
                continue
            captured.append({'state': 'captured', 'quality': playlist.parent.name,
                'sha256': hashlib.sha256(data).hexdigest(), 'ended': b'#EXT-X-ENDLIST' in data,
                'durations': durations, 'physicalSegments': [f'segment-{index:05d}.m4s' for index in range(64)
                    if (playlist.parent / f'segment-{index:05d}.m4s').is_file()]})
    except OSError:
        captured.append({'state': 'unavailable'})
    return captured
