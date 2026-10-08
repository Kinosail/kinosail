"""Exercise the real closed workflow validator before any hosted operation."""
from pathlib import Path
import hashlib
import os
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class WorkflowInventoryTests(unittest.TestCase):
    def run_inventory(self, change=None):
        with tempfile.TemporaryDirectory(prefix="kino-workflow-inventory-") as directory:
            root = Path(directory)
            for path in ("scripts/ci/validate-workflows.sh", "scripts/tooling/gates-pause.sh",
                         ".github/dependabot.yml", ".github/pull_request_template.md"):
                target = root / path
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(ROOT / path, target)
            shutil.copytree(ROOT / ".github/workflows", root / ".github/workflows")
            if change:
                change(root / ".github/workflows")
            tools = root / "tripwires"
            tools.mkdir()
            effects = root / "effects"
            for name in ("go", "node", "docker", "podman", "curl", "npm", "pnpm"):
                command = tools / name
                command.write_text('#!/bin/sh\nprintf effect >> "$INVENTORY_EFFECTS"\nexit 99\n')
                command.chmod(0o700)
            def snapshot():
                return {str(path.relative_to(root)): hashlib.sha256(path.read_bytes()).hexdigest()
                        for path in root.rglob("*") if path.is_file()}
            before = snapshot()
            result = subprocess.run(["bash", str(root / "scripts/ci/validate-workflows.sh")],
                                    cwd=root, capture_output=True, text=True, timeout=5,
                                    env={**os.environ, "PATH": str(tools) + os.pathsep + os.environ["PATH"],
                                         "INVENTORY_EFFECTS": str(effects)})
            self.assertEqual(snapshot(), before, "validation must not write fixture state")
            self.assertFalse(effects.exists(), "validation must not invoke hosted operations")
            return result

    def test_current_nine_workflows_are_admitted_by_actual_cli(self):
        result = self.run_inventory()
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_each_missing_required_workflow_rejects_without_effects(self):
        for path in sorted((ROOT / ".github/workflows").glob("*.yml")):
            with self.subTest(workflow=path.name):
                result = self.run_inventory(lambda folder: (folder / path.name).unlink())
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("missing " + path.name, result.stderr)

    def test_unknown_extra_rejects_without_effects(self):
        result = self.run_inventory(lambda folder: (folder / "unknown.yml").write_text("name: unknown\n"))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("unexpected workflow file", result.stderr)

    def test_replacement_cannot_satisfy_inventory_cardinality(self):
        def replace(folder):
            (folder / "native-tv-e2e.yml").rename(folder / "replacement.yml")
        result = self.run_inventory(replace)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("missing native-tv-e2e.yml", result.stderr)


if __name__ == "__main__":
    unittest.main()
