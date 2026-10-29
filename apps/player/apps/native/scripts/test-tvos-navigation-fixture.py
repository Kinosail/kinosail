"""Isolated checks for fixture safety: UI journeys cannot supply hostile CLI,
state, framing or Range inputs. Rejections must preserve the disposable state.
"""
from email.message import Message
import importlib.util
import io
import json
from pathlib import Path
import runpy
import sys
import tempfile
import unittest
from unittest.mock import Mock, patch


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
                      "delay=31", "profile=production", "target=unknown", "extra=1", "mode=%GG",
                      "target=connection&mode=empty", "target=connection&delay=1",
                      "target=artwork&delay=45", "target=artwork&mode=failed",
                      "target=artwork&mode=failed&delay=91", "target=artwork&mode=failed&delay=nan"]:
            before = dict(self.fixture.STATE), dict(self.fixture.VIEWER["viewer"])
            with self.subTest(query=query):
                self.assertEqual(self.request("GET", "/qa/state?" + query)[0][0], 400)
                self.assertEqual((self.fixture.STATE, self.fixture.VIEWER["viewer"]), before)

    def test_empty_details_remove_only_response_backdrop(self):
        original_state = dict(self.fixture.STATE)
        original_items = [dict(item) for item in self.fixture.ITEMS]
        try:
            self.assertEqual(self.request("GET", "/qa/state?mode=empty&target=details")[0][0], 200)
            status, detail = self.request("GET", "/api/v1/items/movie-004", authorized=True)[0]
            self.assertEqual(status, 200)
            expected = dict(original_items[4], backdrop="")
            self.assertEqual(detail, dict(item=expected, listed=False, profileId="qa"))
            self.assertEqual(self.fixture.ITEMS, original_items)
            status, library = self.request("GET", "/api/v1/library", authorized=True)[0]
            self.assertEqual(status, 200)
            movie = next(item for item in library["items"] if item["id"] == "movie-004")
            self.assertEqual(movie["backdrop"], original_items[4]["backdrop"])
            self.assertEqual(self.request("GET", "/qa/state?mode=loaded&target=details")[0][0], 200)
            status, restored = self.request("GET", "/api/v1/items/movie-004", authorized=True)[0]
            self.assertEqual(status, 200)
            self.assertEqual(restored["item"], original_items[4])
            self.assertEqual(self.fixture.ITEMS, original_items)
        finally:
            self.fixture.STATE.clear()
            self.fixture.STATE.update(original_state)

    def test_artwork_outage_fails_only_posters_until_it_ends(self):
        original_state = dict(self.fixture.STATE)
        try:
            with patch.object(self.fixture.time, "time", return_value=1000.0):
                self.assertEqual(self.request("GET", "/qa/state?mode=failed&target=artwork&delay=45")[0][0], 200)
            with patch.object(self.fixture.time, "sleep"), patch.object(sys, "stdout", io.StringIO()) as output:
                with patch.object(self.fixture.time, "time", return_value=1044.0):
                    self.assertEqual(self.request("GET", "/art/movie-004", authorized=True)[0][0], 404)
                    self.assertEqual(self.request("GET", "/backdrop/movie-004", authorized=True)[0][0], 200)
                    self.assertEqual(self.request("GET", "/art/movie-004")[0][0], 401)
                with patch.object(self.fixture.time, "time", return_value=1045.0):
                    self.assertEqual(self.request("GET", "/art/movie-004", authorized=True)[0][0], 200)
            receipts = [json.loads(line.split(" ", 1)[1]) for line in output.getvalue().splitlines()
                        if line.startswith("ARTWORK_OUTAGE_RESPONSE ")]
            self.assertEqual(receipts, [dict(path="/art/movie-004", status=404, unixTime=1044.0)])
            self.assertEqual(self.request("GET", "/qa/state?mode=loaded&target=library")[0][0], 200)
            self.assertEqual(self.fixture.STATE["artworkUntil"], 0)
        finally:
            self.fixture.STATE.clear()
            self.fixture.STATE.update(original_state)

    def test_unauthorized_and_unknown_writes_are_rejected_before_reading(self):
        path = "/api/v1/items/movie-000/progress/sync"
        (status, _), handler = self.request("PUT", path, b"{}")
        self.assertEqual(status, 401)
        self.assertEqual(handler.rfile.tell(), 0)
        (status, _), handler = self.request("PUT", "/unsupported", b"{}", authorized=True)
        self.assertEqual(status, 404)
        self.assertEqual(handler.rfile.tell(), 0)

    def test_invalid_title_preferences_never_change_saved_state(self):
        path = "/api/v1/items/movie-004/playback-preferences"
        valid = dict(self.fixture.PLAY, rate=1.5)
        (status, _), handler = self.request("PUT", path, json.dumps(valid).encode())
        self.assertEqual((status, handler.rfile.tell()), (401, 0))
        (status, _), handler = self.request("PUT", "/api/v1/items/../playback-preferences", b"{}", authorized=True)
        self.assertEqual((status, handler.rfile.tell()), (404, 0))
        for body in [b"[]", b"{", json.dumps(dict(valid, rate=9)).encode(), json.dumps(dict(valid, nightMode=1)).encode(),
                     json.dumps(dict(valid, extra=True)).encode(), json.dumps(dict(valid, audioLanguage="x" * 65)).encode()]:
            with self.subTest(body=body[:40]):
                self.assertEqual(self.request("PUT", path, body, authorized=True)[0][0], 400)
                self.assertEqual(self.fixture.TITLE_PREFS, {})
        with patch.object(sys, "stdout", io.StringIO()):
            self.assertEqual(self.request("PUT", path, json.dumps(valid).encode(), authorized=True)[0],
                             (200, dict(playback=valid, overridden=True)))
        self.assertEqual(self.request("GET", path, authorized=True)[0], (200, dict(playback=valid, overridden=True)))
        self.assertEqual(self.request("DELETE", path)[0][0], 401)
        self.assertEqual(self.request("DELETE", path, authorized=True)[0], (200, dict(playback=self.fixture.PLAY, overridden=False)))
        self.assertEqual(self.fixture.TITLE_PREFS, {})

    def test_missing_backdrop_receipt_requires_successful_response_write(self):
        cases = [("/backdrop/movie-005", 404, None),
                 ("/backdrop/movie-005", 401, None),
                 ("/backdrop/movie-004", 404, None),
                 ("/backdrop/movie-005", 404, BrokenPipeError()),
                 ("/backdrop/movie-005", 404, ConnectionResetError())]
        for path, status, error in cases:
            with self.subTest(path=path, status=status, error=type(error).__name__):
                handler = object.__new__(self.fixture.Handler)
                handler.path = path
                handler.send_response = Mock()
                handler.send_header = Mock()
                handler.end_headers = Mock()
                handler.wfile = Mock()
                handler.wfile.write.side_effect = error
                output = io.StringIO()
                with patch.object(sys, "stdout", output), patch.object(self.fixture.time, "time", return_value=123.25):
                    handler.send(status, {"error": "missing image"})
                if path == "/backdrop/movie-005" and status == 404 and error is None:
                    prefix, payload = output.getvalue().strip().split(" ", 1)
                    self.assertEqual(prefix, "ARTWORK_FIXTURE_RESPONSE")
                    self.assertEqual(json.loads(payload), dict(route="missing-backdrop", status=404,
                                                               completed=True, unixTime=123.25))
                else:
                    self.assertEqual(output.getvalue(), "")

    def test_episode_extras_stay_off_unless_requested(self):
        status, plain = self.request("GET", "/api/v1/items/episode-0/playback", authorized=True)[0]
        self.assertEqual(status, 200)
        self.assertEqual((plain["markers"], plain["chapters"], plain["subtitles"], plain["next"]), ([], [], [], ""))
        self.assertEqual(self.request("GET", "/subtitle/episode-0/en.vtt", authorized=True)[0][0], 404)
        with patch.dict(self.fixture.os.environ, {"KINOSAIL_TV_PLAYBACK_EXTRAS_FIXTURE": "1"}):
            status, extra = self.request("GET", "/api/v1/items/episode-0/playback", authorized=True)[0]
            self.assertEqual(status, 200)
            self.assertEqual([m["type"] for m in extra["markers"]], ["intro", "credits"])
            self.assertTrue(all(0 <= m["start"] < m["end"] <= 30 for m in extra["markers"]))
            self.assertEqual(extra["next"], "episode-1")
            self.assertEqual([c["title"] for c in extra["chapters"]], ["Opening", "Journey", "Credits"])
            self.assertEqual(extra["subtitles"], [dict(label="English", language="en", source="/subtitle/episode-0/en.vtt", default=False)])
            self.assertEqual(self.request("GET", "/api/v1/items/episode-3/playback", authorized=True)[0][1]["next"], "")
            self.assertEqual(self.request("GET", "/api/v1/items/movie-000/playback", authorized=True)[0][1]["markers"], [])
            self.assertEqual(self.request("GET", "/subtitle/episode-0/en.vtt")[0][0], 401)
            status, captions = self.request("GET", "/subtitle/episode-0/en.vtt", authorized=True)[0]
            self.assertEqual(status, 200)
            self.assertTrue(captions.startswith(b"WEBVTT\n\n"))
            for path in ["/subtitle/movie-000/en.vtt", "/subtitle/episode-0/fr.vtt", "/subtitle/episode-9/en.vtt", "/subtitle/../en.vtt"]:
                with self.subTest(path=path):
                    self.assertEqual(self.request("GET", path, authorized=True)[0][0], 404)
        (status, _), handler = self.request("PUT", "/api/v1/items/episode-0/progress/sync", b"{}", authorized=True)
        self.assertEqual(status, 400)

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
