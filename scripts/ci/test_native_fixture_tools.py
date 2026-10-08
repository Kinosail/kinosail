"""Execute the actual Apple prerequisite step without installers or devices."""
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class NativeFixtureTools(unittest.TestCase):
    def test_apple_tools_are_installed_once_and_required_before_devices(self):
        for workflow in ("native-phone-e2e.yml", "native-tv-e2e.yml"):
            source = (ROOT / ".github/workflows" / workflow).read_text()
            script = re.search(r"      - name: Require fixture tools\n        run: \|\n(.*?)      - name: Compile", source, re.S).group(1)
            script = "\n".join(line[10:] for line in script.splitlines()) + "\n"
            for available, install, status in [
                (("ffmpeg", "ffprobe"), "success", 0),
                (("ffprobe",), "success", 0),
                (("ffmpeg",), "success", 0),
                ((), "success", 0),
                ((), "failure", 17),
                ((), "incomplete", 1),
            ]:
                with self.subTest(workflow=workflow, available=available, install=install):
                    with tempfile.TemporaryDirectory() as directory:
                        root = Path(directory)
                        def executable(name, body):
                            path = root / name
                            path.write_text("#!/bin/bash\n" + body + "\n")
                            path.chmod(0o700)
                        for name in available:
                            executable(name, "exit 0")
                        executable("brew", 'echo brew >> "$CALLS"; if [[ "$*" != "install ffmpeg" ]]; then exit 31; fi; '
                                   'if [[ "$INSTALL" == failure ]]; then exit 17; fi; '
                                   'for name in ffmpeg ffprobe; do '
                                   'if [[ "$INSTALL" == incomplete && "$name" == ffprobe ]]; then continue; fi; '
                                   'printf "#!/bin/bash\\nexit 0\\n" > "$TOOLS/$name"; /bin/chmod 700 "$TOOLS/$name"; done')
                        executable("xcodebuild", '[[ "$*" == -version ]] || exit 32; echo xcode >> "$CALLS"')
                        environment = {"PATH": str(root), "TOOLS": str(root), "CALLS": str(root / "calls"), "INSTALL": install}
                        result = subprocess.run(["/bin/bash", "-e", "-c", script], env=environment,
                                                capture_output=True, timeout=10)
                        self.assertEqual(result.returncode, status, result.stderr.decode())
                        calls = (root / "calls").read_text().splitlines() if (root / "calls").exists() else []
                        self.assertEqual(calls.count("brew"), 0 if len(available) == 2 else 1)
                        self.assertEqual(calls.count("xcode"), int(status == 0))
                        self.assertNotIn("hosted.py", script)
                        self.assertNotIn("simctl", script)
