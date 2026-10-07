"""The public layout launcher must not serialize inherited fixture secrets."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class LayoutReceiptPrivacy(unittest.TestCase):
    def test_child_build_failure_receipt_excludes_private_parent_settings(self):
        # Exercise the actual launcher and its exit receipt. The child build is
        # an external stand-in; browser geometry and Go behavior are not claimed.
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            scripts = root / "scripts/testing"
            scripts.mkdir(parents=True)
            for name in ("test-layout-stability-local.py", "layout-stability-local.mjs",
                         "layout-stability-flows.mjs", "layout-stability-bookmarks.mjs",
                         "layout-stability-subtitle-search.mjs",
                         "layout-stability-subtitle-background.mjs",
                         "navigation-diagnostics.mjs", "layout-stability-routing.mjs",
                         "layout-stability-failure.mjs", "layout-stability-flow-page.mjs",
                         "layout-stability-diagnostic-snapshots.mjs",
                         "layout-stability-theater-witness.mjs"):
                shutil.copyfile(ROOT / "scripts/testing" / name, scripts / name)
            tools = root / "tools"
            tools.mkdir()
            for name, body in {
                "go": '#!/bin/sh\nprintf child-build > "$LAYOUT_BUILD_PROBE"\nexit 7\n',
                "ffmpeg": '#!/bin/sh\nfor output; do :; done\nprintf fixture > "$output"\n',
            }.items():
                tool = tools / name
                tool.write_text(body)
                tool.chmod(0o755)
            env = {key: value for key, value in os.environ.items()
                   if not key.startswith("GIT_") and not key.startswith("KINOSAIL_LAYOUT_")}
            env.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull,
                       PATH=str(tools) + ":" + env["PATH"],
                       LAYOUT_BUILD_PROBE=str(root / "child-build-attempt"),
                       KINOSAIL_LAYOUT_APPS="player", KINOSAIL_LAYOUT_BROWSER="chromium",
                       KINOSAIL_LAYOUT_QUICK="1", KINOSAIL_LAYOUT_TOTP="private-factor-marker",
                       KINOSAIL_LAYOUT_UNKNOWN="private-config-marker",
                       KINOSAIL_LAYOUT_PATHS="/private?token=private-query-marker")
            for command in (["init", "--quiet"], ["add", "scripts"],
                            ["-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
                             "commit", "--quiet", "-m", "Public launcher fixture"]):
                subprocess.run(["git", *command], cwd=root, env=env, check=True,
                               capture_output=True)
            result = subprocess.run(["python3", str(scripts / "test-layout-stability-local.py")],
                                    cwd=root, env=env, capture_output=True, text=True)
            self.assertTrue((root / "child-build-attempt").is_file(),
                            "the fake Go build must run before receipt validation")
            self.assertNotEqual(result.returncode, 0)
            receipts = list((root / ".verification/layout").glob("*/receipt.json"))
            self.assertEqual(len(receipts), 1)
            text = receipts[0].read_text()
            for marker in ("private-factor-marker", "private-config-marker", "private-query-marker"):
                self.assertNotIn(marker, text)
            receipt = json.loads(text)
            self.assertEqual(receipt["results"], {"player": "not_completed"})
            self.assertEqual(receipt["settings"]["KINOSAIL_LAYOUT_BROWSER"], "chromium")
            self.assertTrue(receipt["settings"]["KINOSAIL_LAYOUT_QUICK"])


if __name__ == "__main__":
    unittest.main()
