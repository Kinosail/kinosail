"""Regression checks for the signed update channel statement."""
import datetime
import json
import unittest
from pathlib import Path

from update_channel import REVOCATIONS, choose_latest, load_revocations, statement

ROOT = Path(__file__).resolve().parents[2]


def release(tag, draft=False, prerelease=False, assets=("kinosail-release.json", "kinosail-release.json.sigstore.json")):
    return {"tag_name": tag, "draft": draft, "prerelease": prerelease, "assets": [{"name": name} for name in assets]}


class UpdateChannelTests(unittest.TestCase):
    def test_announces_the_highest_signed_published_release(self):
        releases = [release("player-v0.9.0"), release("player-v0.10.0"), release("player-v0.11.0", draft=True),
                    release("player-v0.12.0", prerelease=True), release("subtitles-v9.0.0"), release("player-channel"),
                    release("player-v0.13.0", assets=("kinosail-release.json",)), release("player-v01.0.0"), "invalid"]
        self.assertEqual(choose_latest(releases, "player", []), "v0.10.0")
        self.assertEqual(choose_latest(releases, "player", ["v0.10.0"]), "v0.9.0")
        with self.assertRaises(ValueError):
            choose_latest(releases, "player", ["v0.10.0", "v0.9.0"])
        with self.assertRaises(ValueError):
            choose_latest([{"tag_name": "player-v1.0.0", "assets": []}], "player", [])

    def test_statement_is_short_lived_and_never_announces_a_revoked_version(self):
        now = datetime.datetime(2026, 11, 22, 11, 58, 7, 123, tzinfo=datetime.timezone.utc)
        self.assertEqual(statement("player", "v0.2.0", ["v0.1.3"], now), {
            "schemaVersion": 1, "app": "player", "latest": "v0.2.0", "revoked": ["v0.1.3"],
            "issued": "2026-11-22T11:58:07Z", "expires": "2026-11-29T11:58:07Z"})
        for app, latest, revoked in (("player", "v0.2.0", ["v0.2.0"]), ("other", "v0.2.0", []), ("player", "0.2.0", [])):
            with self.subTest(app=app, latest=latest), self.assertRaises(ValueError):
                statement(app, latest, revoked, now)

    def test_revocation_list_is_strict(self):
        self.assertEqual(load_revocations('{"player":["v0.10.0","v0.9.1"],"subtitles":[]}', "player"), ["v0.9.1", "v0.10.0"])
        for text in ('{"player":[]}', '{"player":["v1"],"subtitles":[]}', '{"player":["v1.0.0","v1.0.0"],"subtitles":[]}',
                     '{"player":"v1.0.0","subtitles":[]}', '[]', '{"player":[],"subtitles":[],"extra":[]}',
                     json.dumps({"player": [f"v0.0.{index}" for index in range(101)], "subtitles": []})):
            with self.subTest(text=text[:40]), self.assertRaises(ValueError):
                load_revocations(text, "player")
        with self.assertRaises(ValueError):
            load_revocations('{"player":[],"subtitles":[]}', "other")

    def test_repository_revocation_list_is_valid_for_every_app(self):
        source = (ROOT / REVOCATIONS).read_text()
        for app in ("player", "subtitles"):
            load_revocations(source, app)


if __name__ == "__main__":
    unittest.main()
