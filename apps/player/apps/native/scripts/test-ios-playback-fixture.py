"""Native UI journeys cannot inject hostile fixture inputs. Isolated checks cover
CLI bind-before-validation, unbounded reads, ambiguous JSON/query/ranges, and
invalid progress/approval acceptance. Every rejection must preserve state.
"""
from email.message import Message
import io
import importlib.util
import json
from pathlib import Path
import runpy
import sys
import tempfile
import unittest
from unittest.mock import Mock, patch


class FixtureValidationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        script = Path(__file__).with_name("ios-playback-fixture.py")
        spec = importlib.util.spec_from_file_location("fixture", script)
        cls.fixture = importlib.util.module_from_spec(spec)
        with tempfile.NamedTemporaryFile() as video:
            video.write(b"fixture")
            video.flush()
            with patch.object(sys, "argv", [str(script), video.name]), patch("http.server.ThreadingHTTPServer"):
                spec.loader.exec_module(cls.fixture)

    def request(self, query):
        handler = object.__new__(self.fixture.Handler)
        handler.path = "/qa/state" + query
        handler.headers = {}
        responses = []
        handler.send = lambda status, body: responses.append((status, body))
        handler.do_GET()
        return responses[0]

    def test_invalid_state_requests_have_no_side_effects(self):
        for query in ["", "?mode=", "?mode=unknown", "?mode=failed&mode=loaded",
                      "?mode=failed&extra=1", "?mode=failed&broken", "?mode=%GG",
                      "?mode=failed&" + "x" * 4096]:
            with self.subTest(query=query[:80]):
                self.fixture.STATE["mode"] = "loaded"
                self.assertEqual(self.request(query)[0], 400)
                self.assertEqual(self.fixture.STATE, {"mode": "loaded"})

    def test_supported_modes_are_accepted(self):
        for mode in ["pending", "preparing", "failed", "loaded"]:
            self.assertEqual(self.request("?mode=" + mode)[0], 200)
            self.assertEqual(self.fixture.STATE, {"mode": mode})

    def send_request(self, method, path, body=b"", headers=None):
        handler = object.__new__(self.fixture.Handler)
        handler.path, handler.connection = path, Mock()
        handler.headers = Message()
        for key, value in (headers if headers is not None else
                           [("Content-Length", str(len(body)))]) + [("Authorization", "Bearer " + self.fixture.TOKEN)]:
            handler.headers[key] = value
        handler.rfile = io.BytesIO(body)
        responses = []
        handler.send = lambda status, value=None, *args, **kwargs: responses.append((status, value))
        before = dict(self.fixture.STATE)
        with patch.object(self.fixture.time, "sleep") as delay:
            getattr(handler, "do_" + method)()
            if responses[0][0] >= 400:
                delay.assert_not_called()
                self.assertEqual(self.fixture.STATE, before)
        return responses[0], handler

    def test_invalid_cli_never_binds(self):
        script = Path(__file__).with_name("ios-playback-fixture.py")
        with tempfile.TemporaryDirectory() as directory:
            small, empty, huge = [Path(directory) / name for name in ["small.mp4", "empty.mp4", "huge.mp4"]]
            small.write_bytes(b"movie")
            empty.touch()
            with huge.open("wb") as stream:
                stream.truncate(32 * 1024 * 1024 + 1)
            for args in [[], [""], [str(small), "extra"], [directory], [str(empty)], [str(huge)],
                         [str(Path(directory) / "missing.mp4")], ["x" * 4097], ["bad\0path"]]:
                with self.subTest(args=[value[:80] for value in args]), \
                     patch.object(sys, "argv", [str(script)] + args), patch("http.server.ThreadingHTTPServer") as server:
                    with self.assertRaises(SystemExit):
                        runpy.run_path(str(script), run_name="__main__")
                    server.assert_not_called()

    def test_bad_framing_is_rejected_before_reading(self):
        for headers in [[], [("Content-Length", "-1")], [("Content-Length", "+2")],
                        [("Content-Length", "x")], [("Content-Length", "١")],
                        [("Content-Length", "16385")], [("Content-Length", "9" * 100)],
                        [("Content-Length", "2"), ("Content-Length", "2")],
                        [("Content-Length", "2"), ("Transfer-Encoding", "chunked")]]:
            with self.subTest(headers=headers):
                (status, _), handler = self.send_request("POST", "/api/v1/quick-connect", b"{}", headers)
                self.assertEqual(status, 400)
                self.assertEqual(handler.rfile.tell(), 0)

    def test_invalid_json_and_approval_fields_are_rejected(self):
        for body in [b"", b"{", b"[]", b"null", b"\xff", b'{"device":"QA","device":"other"}',
                     b'{"device":NaN}', b'{"device":Infinity}', b"{}", b'{"device":""}',
                     b'{"device":false}', json.dumps({"device": "x" * 81}).encode(),
                     b'{"device":"QA","extra":1}']:
            with self.subTest(body=body[:80]):
                self.assertEqual(self.send_request("POST", "/api/v1/quick-connect", body)[0][0], 400)
        self.assertEqual(self.send_request("POST", "/api/v1/quick-connect", b'{"device":"QA"}',
                                          [("Content-Length", "100")])[0][0], 400)
        for secret in [None, "", "unknown", False]:
            body = json.dumps({"secret": secret}).encode()
            self.assertEqual(self.send_request("POST", "/api/v1/quick-connect/token", body)[0][0], 400)

    def test_library_rejects_ambiguous_unknown_and_out_of_range_inputs(self):
        for query in ["offset=-1", "offset=1.0", "offset=1000001", "limit=0", "limit=201",
                      "limit=x", "limit=1&limit=2", "view=unknown", "view=", "sort=unknown",
                      "extra=1", "q=%FF", "q=%GG", "q=%00", "q=" + "x" * 257,
                      "q=" + "x" * 4096, "limit=2&broken"]:
            with self.subTest(query=query[:80]):
                self.assertEqual(self.send_request("GET", "/api/v1/library?" + query)[0][0], 400)
        self.assertEqual(self.send_request("GET", "/api/v1/library?q=&view=history&sort=title&offset=0&limit=200")[0][0], 200)

    def test_range_rejection_never_delays_or_returns_media(self):
        self.fixture.STATE["mode"] = "preparing"
        for value in ["0-1", "bytes=-", "bytes=-0", "bytes=-2-4", "bytes=4-2", "bytes=0-1,2-3",
                      "bytes=+1-2", "bytes=0-x", "bytes=" + "9" * 200 + "-"]:
            with self.subTest(value=value[:80]):
                self.assertEqual(self.send_request("GET", "/media/loading-video", headers=[("Range", value)])[0][0], 416)
        size = len(self.fixture.MOVIE)
        self.assertEqual(self.send_request("GET", "/media/loading-video", headers=[("Range", f"bytes={size}-")])[0][0], 416)
        for value, data in [("bytes=0-1", self.fixture.MOVIE[:2]),
                            ("bytes=1-", self.fixture.MOVIE[1:]),
                            ("bytes=-2", self.fixture.MOVIE[-2:]),
                            ("bytes=0-999", self.fixture.MOVIE)]:
            (status, actual), _ = self.send_request("GET", "/media/loading-video", headers=[("Range", value)])
            self.assertEqual((status, actual), (206, data))

    def test_progress_schema_and_routes_reject_invalid_values(self):
        progress = dict(seconds=1, watched=False, session="qa-session", revision=1)
        baseline = dict(seconds=0, watched=False, session="", revision=0)
        valid = dict(progress=progress, expected=baseline, playbackToken="")
        path = "/api/v1/items/loading-video/progress/sync"
        variants = [{}, [], {**valid, "unknown": True}, {**valid, "playbackToken": "x" * 8193}]
        for key, value in [("seconds", -1), ("seconds", 31536001), ("seconds", True),
                           ("seconds", float("inf")), ("watched", 1), ("session", ""),
                           ("session", "x" * 129), ("revision", 0), ("revision", True),
                           ("revision", 1.5), ("revision", 9007199254740992)]:
            variants.append({**valid, "progress": {**progress, key: value}})
        variants.append({**valid, "progress": {**progress, "unknown": 1}})
        for body in variants:
            with self.subTest(body=str(body)[:80]):
                self.assertEqual(self.send_request("PUT", path, json.dumps(body).encode())[0][0], 400)
        self.assertEqual(self.send_request("PUT", "/api/v1/items/other/progress/sync", json.dumps(valid).encode())[0][0], 404)
        self.assertEqual(self.send_request("GET", "/api/v1/items/other/bookmarks")[0][0], 404)
        self.assertEqual(self.send_request("PUT", path, json.dumps(valid).encode())[0], (200, progress))

    def test_playback_capabilities_reject_unknown_duplicate_and_invalid_values(self):
        query = "videoCodecs=h264&audioCodecs=aac,mp3,ac3,eac3&hdrFormats=sdr&maxAudioChannels=2"
        path = "/api/v1/items/loading-video/playback?"
        self.fixture.STATE["mode"] = "loaded"
        for value in [query + "&unknown=1", query + "&maxAudioChannels=3",
                      query.replace("h264", "unknown"), query.replace("h264", "h264,h264"),
                      query.replace("maxAudioChannels=2", "maxAudioChannels=0"),
                      query.replace("maxAudioChannels=2", "maxAudioChannels=9"),
                      query.replace("sdr", ""), "", "videoCodecs=" + "x" * 4096]:
            with self.subTest(query=value[:80]):
                self.assertEqual(self.send_request("GET", path + value)[0][0], 400)
        self.assertEqual(self.send_request("GET", path + query)[0][0], 200)

    def test_unknown_playback_routes_cannot_use_fixture_metadata(self):
        self.assertEqual(self.send_request("GET", "/api/v1/items/other/playback")[0][0], 404)


if __name__ == "__main__":
    unittest.main()
