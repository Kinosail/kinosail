"""Reject unsupported container engines before creating fixtures or invoking them."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class ContainerContextInputTest(unittest.TestCase):
    def test_invalid_engines_have_no_side_effects(self):
        script = Path(__file__).with_name("test-container-context.sh")
        with tempfile.TemporaryDirectory() as directory:
            marker = Path(directory) / "invoked"
            executable = Path(directory) / "unsupported"
            executable.write_text(f"#!/bin/sh\ntouch '{marker}'\n")
            executable.chmod(0o700)
            environment = {
                **os.environ,
                "PATH": f"{directory}:{os.environ['PATH']}",
                "TMPDIR": directory,
            }
            for value in ("unsupported", str(executable), "podman --remote", "podman\n", "x" * 4097):
                with self.subTest(engine=value[:40]):
                    result = subprocess.run(
                        [str(script)],
                        env={**environment, "CONTAINER_ENGINE": value},
                        capture_output=True,
                        text=True,
                        check=False,
                    )
                    self.assertEqual(result.returncode, 2)
                    self.assertIn("unsupported container engine", result.stderr)
                    self.assertEqual(list(Path(directory).iterdir()), [executable])


if __name__ == "__main__":
    unittest.main()
