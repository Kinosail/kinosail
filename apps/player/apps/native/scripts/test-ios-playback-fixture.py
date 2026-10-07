"""The fixture must reject missing, unknown, conflicting, malformed, and huge state inputs without mutation."""
import importlib.util
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch


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
        for mode in ["pending", "failed", "loaded"]:
            self.assertEqual(self.request("?mode=" + mode)[0], 200)
            self.assertEqual(self.fixture.STATE, {"mode": mode})


if __name__ == "__main__":
    unittest.main()
