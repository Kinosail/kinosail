#!/usr/bin/env python3
"""CLI regression: every production source must load in its build context.

Existing metric tests do not exercise the gate's platform, tag, and workspace
selection. Product browser journeys cannot reach this source-tool boundary.
"""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


REPO = Path(__file__).resolve().parents[2]


class HalsteadConfigurationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.directory = tempfile.TemporaryDirectory(prefix="kinosail-halstead-")
        cls.addClassCleanup(cls.directory.cleanup)
        cls.root = Path(cls.directory.name).resolve()
        cls.tool = cls.root / "metrics"
        subprocess.run(["go", "build", "-o", str(cls.tool), "."],
                       cwd=REPO / "scripts/quality/metrics",
                       env=dict(os.environ, GOWORK="off"), check=True,
                       capture_output=True, text=True)

    def test_gate_loads_excluded_platform_tagged_fixture_and_workspace_command(self):
        with tempfile.TemporaryDirectory(dir=self.root) as directory:
            root = Path(directory)
            files = {
                "go.work": "go 1.27.3\nuse (\n./apps/player\n./packages\n)\n",
                "apps/player/go.mod": "module example.test/player\ngo 1.27.3\n",
                "packages/go.mod": "module example.test/packages\ngo 1.27.3\n",
                "packages/value/value.go": "package value\nfunc Value() int { return 3 }\n",
                "apps/player/internal/server/base.go": "package server\n",
                "apps/player/internal/server/hls_copied_process_other.go":
                    "//go:build !linux && !darwin && !windows\n\npackage server\nfunc Fallback() int { return 1 }\n",
                "apps/player/e2e/compose-template-fixture.go":
                    "//go:build q47proof\n\npackage main\nfunc main() {}\n",
                "scripts/testing/hls-readiness/main.go":
                    'package main\nimport "example.test/packages/value"\nfunc main() { _ = value.Value() }\n',
            }
            for name, content in files.items():
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(content)
            subprocess.run(["git", "init", "--quiet"], cwd=root, check=True)
            result = subprocess.run(["python3", str(REPO / "scripts/quality/check-halstead.py"),
                                     str(self.tool), str(root)], cwd=root,
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)


if __name__ == "__main__":
    unittest.main()
