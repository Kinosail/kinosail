"""Keep the actual Server caption recovery journey mandatory in container CI.

Browser tests cannot prove that their CI launcher supplies the playable fixture
or selects the real Server journey instead of accepting a skipped opt-in test.
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


class CaptionJourneyTest(unittest.TestCase):
    def test_helper_selects_real_server_and_passes_fixture_and_artifact_paths(self):
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
            result = subprocess.run(["bash", "-c", 'set -euo pipefail; source "$1"; '
                                     'run_subtitle_recovery_journey chromium "$2" "$3" "$4"',
                                     "caption-test", str(HELPER), str(root / "playable fixture.mp4"),
                                     str(root / "results"), str(root / "artifacts")],
                                    cwd=app, env=environment, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            actual = json.loads(result.stdout)
            self.assertEqual(actual["argv"], ["go", "test", "-p", "1", "./internal/server", "-run",
                                               "^TestSubtitleRecoveryBrowserJourney$", "-count=1", "-timeout=6m"])
            self.assertEqual(actual["env"], {"GOMAXPROCS": "2", "KINOSAIL_CAPTION_BROWSER": "1",
                                             "KINOSAIL_BROWSER_PROJECT": "chromium",
                                             "KINOSAIL_CAPTION_MEDIA_FIXTURE": str(root / "playable fixture.mp4"),
                                             "KINOSAIL_E2E_OUTPUT_DIR": str(root / "results"),
                                             "KINOSAIL_E2E_ARTIFACT_DIR": str(root / "artifacts")})

    def test_container_runs_caption_journey_before_general_smoke(self):
        source = CONTAINER.read_text()
        caption = source.index('run_subtitle_recovery_journey "$project" "$media_dir/Direct Retry Control.mp4"')
        smoke = source.index('pnpm --dir e2e test "${browser_args[@]}"')
        self.assertLess(caption, smoke)
        self.assertIn('source "$app/scripts/test-browser-journeys.sh"', source)
        self.assertIn('run_library_pagination_journey "$project"', source)
        self.assertIn('run_populated_player_journeys "$project"', source)


if __name__ == "__main__":
    unittest.main()
