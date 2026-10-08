"""Actual supervisor cleanup must not remove a substituted fixture root."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class OwnedFixtureRootTests(unittest.TestCase):
    def test_joined_child_cleanup_preserves_substituted_root(self):
        for mode in ("unchanged", "replacement", "symlink"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                owned = Path(directory)
                tools = owned / "tools"
                tools.mkdir()
                tool = tools / "ffmpeg"
                tool.write_text("#!/bin/sh\nexit 0\n")
                tool.chmod(0o755)
                receipt = owned / "child.json"
                app = tools / "app"
                app.write_text("#!" + sys.executable + "\n"
                    "import json,os\nfrom pathlib import Path\n"
                    "root=Path(os.environ[\"KINOSAIL_DATA_DIR\"]).parent\n"
                    "moved=root.with_name(root.name+\"-captured\")\n"
                    + ("root.rename(moved)\n" if mode != "unchanged" else "")
                    + ("root.mkdir();(root/\"foreign\").write_text(\"preserve\")\n" if mode == "replacement" else "")
                    + ("target=root.with_name(root.name+\"-foreign\");target.mkdir();(target/\"foreign\").write_text(\"preserve\");root.symlink_to(target,target_is_directory=True)\n" if mode == "symlink" else "")
                    + f"Path({str(receipt)!r}).write_text(json.dumps({{\"root\":str(root),\"moved\":str(moved)}}))\n")
                app.chmod(0o755)
                env = {key: os.environ[key] for key in ("PATH","LANG","LC_ALL") if key in os.environ}
                env.update(PATH=str(tools)+":"+env["PATH"], TMPDIR=str(owned),
                    KINOSAIL_E2E_SUBTITLES_BINARY=str(app))
                result = subprocess.run(["node",str(ROOT/"scripts/e2e/fixture.mjs"),"subtitles","49129"],
                    cwd=owned,env=env,capture_output=True,text=True,timeout=10)
                self.assertEqual(result.returncode,0,result.stderr)
                child=json.loads(receipt.read_text())
                root=Path(child["root"])
                if mode=="unchanged":
                    self.assertFalse(root.exists())
                else:
                    self.assertTrue(root.exists() or root.is_symlink(),"foreign root entry was removed")
                    self.assertEqual((root/"foreign").read_text(),"preserve")
                    self.assertTrue(Path(child["moved"]).is_dir(),"captured root must not be confused with replacement")
                self.assertFalse((owned/".e2e/fixtures/49129.json").exists())


if __name__ == "__main__":
    unittest.main()
