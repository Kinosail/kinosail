"""Private disposable HTTP fixture helpers; receipts never include credentials."""
import base64
import hashlib
import hmac
import json
import math
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

    def http(self, path, method="GET", body=None, authenticated=True, *, timeout=40):
        if type(timeout) not in (int, float) or not math.isfinite(timeout) or not 0 < timeout <= 40:
            raise RuntimeError("public_timeout_invalid")
        headers = {"Content-Type": "application/json"}
        if authenticated and self.token:
            headers["Authorization"] = "Bearer " + self.token
        request = urllib.request.Request(self.url + path, method=method, headers=headers,
            data=json.dumps(body).encode() if body is not None else None)
        try:
            response = self.opener.open(request, timeout=timeout)
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
