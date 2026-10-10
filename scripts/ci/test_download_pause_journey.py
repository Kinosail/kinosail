"""Keep both populated download Pause modes mandatory with separate artifacts.

The browser cases cannot detect a launcher that omits their opt-in native Server
journey or inherits the supplemental flag and silently drops the eight cases.
"""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / "apps/player/scripts/test-browser-journeys.sh"
CONTAINER = ROOT / "apps/player/scripts/test-container.sh"


class DownloadPauseJourneyTest(unittest.TestCase):
    def test_both_modes_run_with_explicit_flags_and_distinct_results(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            app = root / "apps/player"
            app.mkdir(parents=True)
            tooling = root / "scripts/tooling"
            tooling.mkdir(parents=True)
            wrapper = tooling / "with-go-module.sh"
            wrapper.write_text("#!/usr/bin/env python3\nimport json,os,sys\n"
                               "print(json.dumps({'argv':sys.argv[1:],'env':"
                               "{k:v for k,v in os.environ.items() if k.startswith('KINOSAIL_') or k=='GOMAXPROCS'}}))\n")
            wrapper.chmod(0o700)
            environment = {k: v for k, v in os.environ.items() if not k.startswith("KINOSAIL_")}
            environment["KINOSAIL_DOWNLOAD_PAUSE_HIT_TARGETS"] = "1"
            result = subprocess.run(["bash", "-c", 'set -euo pipefail; source "$1"; '
                                     'run_download_pause_journeys firefox "$2" "$3"',
                                     "download-test", str(HELPER), str(root / "results with spaces"),
                                     str(root / "artifacts with spaces")],
                                    cwd=app, env=environment, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            actual = [json.loads(line) for line in result.stdout.splitlines()]
            self.assertEqual(len(actual), 2)
            for index, mode in enumerate(["downloads-pause", "downloads-hit-targets"]):
                self.assertEqual(actual[index]["argv"], ["go", "test", "-p", "1", "./internal/server", "-run",
                                                        "^TestDownloadPauseBrowserJourney$", "-count=1", "-timeout=6m"])
                self.assertEqual(actual[index]["env"], {
                    "GOMAXPROCS": "2", "KINOSAIL_DOWNLOAD_PAUSE_BROWSER": "1",
                    "KINOSAIL_DOWNLOAD_PAUSE_HIT_TARGETS": str(index), "KINOSAIL_BROWSER_PROJECT": "firefox",
                    "KINOSAIL_E2E_OUTPUT_DIR": str(root / "results with spaces") + "-" + mode,
                    "KINOSAIL_E2E_ARTIFACT_DIR": str(root / "artifacts with spaces" / mode)})

    def test_container_runs_both_modes_before_general_smoke(self):
        source = CONTAINER.read_text()
        pause = source.index('run_download_pause_journeys "$project"')
        smoke = source.index('pnpm --dir e2e test "${browser_args[@]}"')
        self.assertLess(pause, smoke)
        self.assertIn('run_local_player_journeys "$project"', source)
        self.assertIn('run_subtitle_recovery_journey "$project"', source)
        self.assertIn('run_populated_player_journeys "$project"', source)
        # This clean Q09 branch starts at main; R08's unmerged fixture remains
        # protected by its separate original branch/controls, not this feature.
        helper = HELPER.read_text()
        for title in ('settings search crosses levels and preserves unsaved preferences',
                      'Owner settings search finds a setting across task families',
                      'real Server rejects invalid progress without changing stored state and web reports the rejection',
                      'populated player retries the latest progress through the real Server and renders accessible states'):
            self.assertIn("--required-title '" + title + "'", helper)


if __name__ == "__main__":
    unittest.main()
