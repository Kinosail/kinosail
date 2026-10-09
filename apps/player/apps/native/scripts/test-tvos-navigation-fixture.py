"""Isolated checks for fixture safety: UI journeys cannot supply hostile CLI,
state, framing or Range inputs. Rejections must preserve the disposable state.
"""
from email.message import Message
import importlib.util
import io
from pathlib import Path
import runpy
import sys
import tempfile
import unittest
from unittest.mock import patch


class NavigationFixtureTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.script = Path(__file__).with_name("tvos-navigation-fixture.py")
        spec = importlib.util.spec_from_file_location("navigation_fixture", cls.script)
        cls.fixture = importlib.util.module_from_spec(spec)
        with tempfile.NamedTemporaryFile() as movie:
            movie.write(b"generated-fixture")
            movie.flush()
            with patch.object(sys, "argv", [str(cls.script), movie.name]), \
                 patch("http.server.ThreadingHTTPServer"):
                spec.loader.exec_module(cls.fixture)

    def request(self, method, path, body=b"", authorized=False):
        handler = object.__new__(self.fixture.Handler)
        handler.path = path
        handler.headers = Message()
        handler.headers["Content-Length"] = str(len(body))
        if authorized:
            handler.headers["Authorization"] = "Bearer tv-polish-fixture-token"
        handler.rfile = io.BytesIO(body)
        responses = []
        handler.send = lambda status, value=None, *args, **kwargs: responses.append((status, value))
        getattr(handler, "do_" + method)()
        return responses[0], handler

    def test_invalid_cli_never_binds_or_reads_unbounded_media(self):
        with tempfile.TemporaryDirectory() as folder:
            empty = Path(folder) / "empty.mp4"
            empty.touch()
            huge = Path(folder) / "huge.mp4"
            with huge.open("wb") as movie:
                movie.truncate(32 * 1024 * 1024 + 1)
            for args in [[], [""], [folder], [str(empty)], [str(huge)], ["missing"],
                         [str(empty), "extra"]]:
                with self.subTest(args=args), patch.object(sys, "argv", [str(self.script)] + args), \
                     patch("http.server.ThreadingHTTPServer") as server:
                    with self.assertRaises(SystemExit):
                        runpy.run_path(str(self.script), run_name="__main__")
                    server.assert_not_called()

    def test_invalid_state_preserves_mode_and_viewer(self):
        for query in ["mode=unknown", "mode=failed&mode=loaded", "delay=-1", "delay=inf",
                      "delay=31", "profile=production", "target=unknown", "extra=1", "mode=%GG"]:
            before = dict(self.fixture.STATE), dict(self.fixture.VIEWER["viewer"])
            with self.subTest(query=query):
                self.assertEqual(self.request("GET", "/qa/state?" + query)[0][0], 400)
                self.assertEqual((self.fixture.STATE, self.fixture.VIEWER["viewer"]), before)

    def test_unauthorized_and_unknown_writes_are_rejected_before_reading(self):
        path = "/api/v1/items/movie-000/progress/sync"
        (status, _), handler = self.request("PUT", path, b"{}")
        self.assertEqual(status, 401)
        self.assertEqual(handler.rfile.tell(), 0)
        (status, _), handler = self.request("PUT", "/unsupported", b"{}", authorized=True)
        self.assertEqual(status, 404)
        self.assertEqual(handler.rfile.tell(), 0)

    def test_invalid_ranges_never_return_media(self):
        for value in ["bytes=99999999999999999999-", "bytes=5-2", "bytes=--", "bytes=-0"]:
            handler = object.__new__(self.fixture.Handler)
            handler.path = "/media/movie-000"
            handler.headers = {"Authorization": "Bearer tv-polish-fixture-token", "Range": value}
            statuses = []
            handler.send = lambda status, *args, **kwargs: statuses.append(status)
            handler.do_GET()
            self.assertEqual(statuses, [416])


if __name__ == "__main__":
    unittest.main()
