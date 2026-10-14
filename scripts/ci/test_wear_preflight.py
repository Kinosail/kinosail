"""Isolated CLI checks for Wear runner ownership before emulator creation.

The hosted journey cannot safely seed foreign devices or malformed ADB output.
Failure modes: daemon diagnostics reject a clean host; existing devices or an
invalid inventory allow emulator creation; subprocess errors lose the receipt.
All SDK commands and APK bytes here are inert stand-ins, not native acceptance.
"""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
RUNNER = ROOT / "apps/player/scripts/test-wear-journeys.py"


class WearPreflightTests(unittest.TestCase):
    def test_inventory_separates_diagnostics_and_rejects_unsafe_hosts(self):
        header = "List of devices attached\n"
        for label, stdout, stderr, exit_code, create, failure in (
            ("cold-empty", header + "\n", "* daemon not running; starting now\n* daemon started successfully\n", 0, True, "CalledProcessError"),
            ("warm-empty", header + "\n", "", 0, True, "CalledProcessError"),
            ("connected", header + "fixture-serial\tdevice\n", "", 0, False, "runner_device_already_present"),
            ("offline", header + "fixture-serial\toffline\n", "", 0, False, "runner_device_already_present"),
            ("unauthorized", header + "fixture-serial\tunauthorized\n", "", 0, False, "runner_device_already_present"),
            ("missing-header", "", "", 0, False, "runner_device_inventory_invalid"),
            ("unknown-prefix", "unknown\n" + header, "", 0, False, "runner_device_inventory_invalid"),
            ("header-suffix", header.rstrip() + " invalid\n", "", 0, False, "runner_device_inventory_invalid"),
            ("command-error", header, "fixture failure\n", 1, False, "CalledProcessError"),
        ):
            with self.subTest(case=label), tempfile.TemporaryDirectory() as directory:
                folder = Path(directory)
                for path in ("apps/player/scripts/test-wear-journeys.py",
                             "apps/player/scripts/inspect-wear-evidence.py",
                             "apps/player/scripts/wear_home_readiness.py",
                             ".github/workflows/app.yml",
                             "apps/player/apps/android/wear/build.gradle.kts"):
                    target = folder / path
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_bytes((ROOT / path).read_bytes())
                for variant, name in (("debug", "wear-debug.apk"),
                                      ("androidTest/debug", "wear-debug-androidTest.apk")):
                    target = folder / "apps/player/apps/android/wear/build/outputs/apk" / variant / name
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_bytes(b"inert APK stand-in")
                scripts = {
                    "bin/git": "import sys\nprint('a' * 40) if sys.argv[1] == 'rev-parse' else None\n",
                    "sdk/platform-tools/adb": (
                        "import sys\nassert sys.argv[1:] == ['devices']\n"
                        f"sys.stderr.write({stderr!r})\nsys.stderr.flush()\n"
                        f"sys.stdout.write({stdout!r})\nraise SystemExit({exit_code})\n"),
                    "sdk/cmdline-tools/latest/bin/avdmanager": (
                        "from pathlib import Path\nPath('creation-attempted').touch()\nraise SystemExit(1)\n"),
                }
                for path, script in scripts.items():
                    target = folder / path
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_text(f"#!{sys.executable}\n" + script)
                    target.chmod(0o755)
                result = subprocess.run([sys.executable, "-B", str(RUNNER), "--profile", "small-round", "--output", str(folder / "evidence")],
                    cwd=folder, capture_output=True, text=True, timeout=10,
                    env={**os.environ, "PATH": str(folder / "bin") + os.pathsep + os.environ["PATH"],
                         "GITHUB_ACTIONS": "true", "ANDROID_HOME": str(folder / "sdk")})
                self.assertEqual(result.returncode, 1, result.stderr)
                receipt = json.loads((folder / "evidence/safe/receipt.json").read_text())
                self.assertEqual((folder / "creation-attempted").exists(), create, receipt)
                self.assertEqual(receipt["failureClass"], failure)
                self.assertEqual(receipt["cases"], [])
                self.assertEqual(receipt["result"], "failed")
                self.assertTrue(receipt["sourceUnchanged"])
                self.assertNotIn("fixture-serial", json.dumps(receipt))


if __name__ == "__main__":
    unittest.main()
