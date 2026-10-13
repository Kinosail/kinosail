"""Real HTTP boundary controls for the isolated caption fixture bridge.

The populated native journey cannot make its real Go Server emit malformed headers.
These controls own that gap and prove rejected request targets do not reach a peer.
Failure modes: path escape, absolute URL, encoded traversal, unbounded target/header,
header folding, rejection after response headers, or an unjoined fixture peer.
"""
import http.client
from http.server import BaseHTTPRequestHandler
import threading
from android_caption_fixture import Bridge, OwnedBridgeServer


class Peer(BaseHTTPRequestHandler):
    calls = 0
    malformed = False

    def do_GET(self):
        Peer.calls += 1
        self.send_response(200)
        self.send_header("Content-Type", "text/plain\r\n X-Injected: yes" if Peer.malformed else "text/plain")
        self.send_header("Content-Length", "2")
        self.end_headers()
        self.wfile.write(b"ok")

    def do_PUT(self):
        self.do_GET()

    def log_message(self, *args):
        pass


def verify_bridge(receipt):
    owned = []
    receipt.update(boundary="Isolated real HTTP peer; native populated E2E cannot emit malformed upstream headers",
                   cases=[], settled=False)
    try:
        for address, handler in ((("127.0.0.1", 39227), Peer), (("127.0.0.1", 39231), Bridge)):
            server = OwnedBridgeServer(address, handler)
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            owned.append((server, thread))
            thread.start()
        def request(method, path):
            client = http.client.HTTPConnection("127.0.0.1", 39231, timeout=10)
            try:
                client.request(method, path)
                response = client.getresponse()
                return response.status, dict(response.getheaders()), response.read(4096)
            finally:
                client.close()
        receipt["activeCase"] = "valid_request"
        status, _, body = request("GET", "/api/v1/me")
        assert status == 200 and body == b"ok"
        receipt["cases"].append({"name": "valid_request", "status": status})
        for name, method, path in (
            ("dot_parent", "GET", "/api/v1/../settings"),
            ("encoded_parent", "GET", "/api/v1/%2e%2e/settings"),
            ("encoded_slash", "GET", "/api/v1/items/abc%2fdef"),
            ("absolute_url", "GET", "http://127.0.0.1:39227/api/v1/me"),
            ("foreign_endpoint", "GET", "/api/v1/settings"),
            ("wrong_method", "PUT", "/api/v1/me"),
            ("unknown_query", "GET", "/api/v1/library?redirect=http%3A%2F%2Fexample.invalid"),
            ("oversized_target", "GET", "/api/v1/library?q=" + "a" * 4096)):
            receipt["activeCase"] = name
            before = Peer.calls
            status, _, _ = request(method, path)
            receipt["cases"].append({"name": name, "status": status, "upstreamCalls": Peer.calls - before})
            assert status == 400 and Peer.calls == before, name
        receipt["activeCase"] = "folded_upstream_header"
        Peer.malformed = True
        status, headers, _ = request("GET", "/api/v1/me")
        assert status == 502 and not any("injected" in k.lower() or "injected" in v.lower() for k,v in headers.items())
        receipt["cases"].append({"name": "folded_upstream_header", "status": status, "injectedHeader": False})
    finally:
        Peer.malformed = False
        for server, thread in reversed(owned):
            if thread.is_alive():
                server.shutdown()
            server.server_close()
            if thread.ident is not None:
                thread.join(timeout=5)
        receipt["settled"] = all(not thread.is_alive() for _,thread in owned)
    assert receipt["settled"]
    receipt["activeCase"] = None
    return receipt
