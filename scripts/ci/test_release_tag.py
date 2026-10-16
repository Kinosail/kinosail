"""Version tag and installer inputs are bounded before release work."""

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest
from unittest.mock import patch

from release_tag import main, parse

ROOT = Path(__file__).resolve().parents[2]


class ReleaseTagTests(unittest.TestCase):
    def test_two_independent_app_tags(self):
        for app in ("player", "subtitles"):
            with self.subTest(app=app):
                self.assertEqual(parse(f"{app}-v1.2.3"),
                                 {"app": app, "version": "v1.2.3", "container_version": "1.2.3"})

    def test_invalid_tag_or_context_creates_no_output(self):
        for tag in ("", "v1.2.3", "retired-v1.2.3", "player-v01.2.3", "player-v1.2", "player-v1.2.3-extra",
                    "player-v1.2.3\nname=other", "other-v1.2.3", "player-v" + "1" * 80):
            with self.subTest(tag=tag[:20]), tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "output"
                env = {"GITHUB_REF_NAME": tag, "GITHUB_REF": f"refs/tags/{tag}",
                       "GITHUB_EVENT_NAME": "push", "GITHUB_REPOSITORY": "Kinosail/kinosail",
                       "GITHUB_OUTPUT": str(output)}
                with patch.dict(os.environ, env), self.assertRaises(ValueError):
                    main()
                self.assertFalse(output.exists())
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "output"
            env = {"GITHUB_REF_NAME": "player-v1.2.3", "GITHUB_REF": "refs/heads/main",
                   "GITHUB_EVENT_NAME": "push", "GITHUB_REPOSITORY": "Kinosail/kinosail",
                   "GITHUB_OUTPUT": str(output)}
            with patch.dict(os.environ, env), self.assertRaises(ValueError):
                main()
            self.assertFalse(output.exists())

    def test_installer_invalid_inputs_never_invoke_tools(self):
        with tempfile.TemporaryDirectory() as directory:
            marker = Path(directory) / "executed"
            for tool in ("cosign", "tar", "python3"):
                binary = Path(directory) / tool
                binary.write_text(f'#!/bin/sh\ntouch "{marker}"\n')
                binary.chmod(0o755)
            for args in ([], ["player"], ["retired", "v1.2.3"], ["../player", "v1.2.3"],
                         ["player", "v01.2.3"], ["player", "x" * 10000]):
                result = subprocess.run(["bash", str(ROOT / "scripts/ci/package-release.sh"), *args],
                                        env=os.environ | {"PATH": directory + ":" + os.environ["PATH"]},
                                        capture_output=True)
                self.assertEqual(result.returncode, 2)
                self.assertFalse(marker.exists())

    def test_native_update_identity_accepts_only_the_manifest_signing_workflow(self):
        workflow = "https://github.com/Kinosail/kinosail/.github/workflows/release.yml"
        release = (ROOT / ".github/workflows/release.yml").read_text()
        signing = (ROOT / "scripts/ci/package-release.sh").read_text()
        self.assertIn(f'--certificate-identity "{workflow}@refs/tags/$GITHUB_REF_NAME"', release)
        self.assertLess(release.index("cosign sign --yes"), release.index("run: ./scripts/ci/package-release.sh"))
        self.assertIn("native-release/kinosail-release.json.sigstore.json native-release/kinosail-release.json", signing)
        for app, other in (("player", "subtitles"), ("subtitles", "player")):
            with self.subTest(app=app):
                contract = json.loads((ROOT / f"apps/{app}/packaging/native-installation.json").read_text())
                identity = re.compile(contract["update"]["signatureIdentity"])
                parse(f"{app}-v1.2.3")
                self.assertTrue(identity.fullmatch(f"{workflow}@refs/tags/{app}-v1.2.3"))
                for rejected in (f"{workflow}@refs/tags/{other}-v1.2.3", f"{workflow}@refs/heads/main",
                                 f"{workflow}@refs/tags/{app}-v01.2.3",
                                 f"https://github.com/Kinosail/kinosail/.github/workflows/{app}-release.yml@refs/tags/{app}-v1.2.3",
                                 f"https://github.com/Kinosail/kinosail/.github/workflows/publish.yml@refs/tags/{app}-v1.2.3",
                                 f"https://github.com/Other/kinosail/.github/workflows/release.yml@refs/tags/{app}-v1.2.3"):
                    self.assertIsNone(identity.fullmatch(rejected), rejected)


if __name__ == "__main__":
    unittest.main()
