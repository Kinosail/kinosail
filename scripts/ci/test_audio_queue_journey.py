"""Prevent CI from silently skipping real album queue and progress evidence.

The browser journeys cannot detect a launcher that omits their album or accepts
missing exact titles. Inspect the actual shell calls without account side effects.
"""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / "apps/player/scripts/test-browser-journeys.sh"
CONTAINER = ROOT / "apps/player/scripts/test-container.sh"
TITLES = ["real album queue advances source and all Now Playing identity to the fictional second track",
          "real album queue keeps system previous and next current and exposes only fresh current-track actions",
          "mobile R03 progress notice stays hidden after real acknowledgement and reopens only on failure"]
EXISTING = ["settings search crosses levels and preserves unsaved preferences",
            "Owner settings search finds a setting across task families",
            "real Server rejects invalid progress without changing stored state and web reports the rejection",
            "populated player retries the latest progress through the real Server and renders accessible states"]


class AudioQueueJourneyTest(unittest.TestCase):
    def invoke(self, function, *arguments):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            wrapper = root / "python3"
            wrapper.write_text(f"#!{sys.executable}\nimport json,os,sys\nfrom pathlib import Path\n"
                               "if '--fixture-only' in sys.argv: Path(sys.argv[-1]).mkdir()\n"
                               "print(json.dumps({'argv':sys.argv[1:],'project':os.environ.get('KINOSAIL_BROWSER_PROJECT')}))\n")
            wrapper.chmod(0o700)
            environment = dict(os.environ, PATH=str(root) + os.pathsep + os.environ["PATH"])
            result = subprocess.run(["bash", "-c", 'set -euo pipefail; source "$1"; shift; "$@"',
                                     "queue-test", str(HELPER), function, *arguments],
                                    cwd=ROOT / "apps/player", env=environment, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            return json.loads(result.stdout)

    def test_prepared_owner_requires_all_queue_and_existing_journeys(self):
        result = self.invoke("run_populated_player_journeys", "chromium", "http://localhost:1234", "/tmp/r08 artifacts")
        actual = result["argv"]
        required = [actual[index + 1] for index, value in enumerate(actual) if value == "--required-title"]
        self.assertCountEqual(required, EXISTING + TITLES)
        self.assertEqual(len(required), 7)
        self.assertEqual(result["project"], "chromium")
        self.assertEqual(actual[actual.index("--") + 1:],
                         ["pnpm", "--dir", "e2e", "test", "settings-discovery.spec.ts", "layout-audit-shell.spec.ts",
                          "test-instance-progress.spec.ts", "test-instance-audio-queue.spec.ts", "--grep=@smoke", "--workers=1"])

    def test_fixture_is_generated_before_any_server_scan(self):
        with tempfile.TemporaryDirectory() as media:
            result = self.invoke("prepare_audio_queue_fixture", media)
            self.assertEqual(result["argv"], [str(ROOT / "scripts/testing/test-player-audio-queue-local.py"),
                                               "--fixture-only", str(Path(media) / "R08 Fictional Session")])
        source = CONTAINER.read_text()
        preparation = source.index('prepare_audio_queue_fixture "$media_dir"')
        first_start = source.index("\nstart_server\n")
        self.assertLess(preparation, first_start)
        self.assertIn('run_library_pagination_journey "$project"', source)
        self.assertIn('run_subtitle_recovery_journey "$project"', source)
        self.assertIn('pnpm --dir e2e test "${browser_args[@]}"', source)


if __name__ == "__main__":
    unittest.main()
