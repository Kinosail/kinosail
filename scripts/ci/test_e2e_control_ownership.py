"""Public supervisor process proof for foreign restart mailbox ownership."""
import json
import os
from pathlib import Path
import signal
import stat
import subprocess
import sys
import tempfile
import time
import unittest

ROOT = Path(__file__).resolve().parents[2]


class RestartControlOwnership(unittest.TestCase):
    def test_foreign_controls_survive_rejection_and_graceful_shutdown(self):
        for kind in ("directory", "symlink", "fifo", "replacement"):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                tools = root / "bin"
                tools.mkdir()
                ffmpeg = tools / "ffmpeg"
                ffmpeg.write_text("#!/bin/sh\nexit 0\n")
                ffmpeg.chmod(0o755)
                app = tools / "app"
                started = root / "started.json"
                app.write_text("#!" + sys.executable + "\nimport json,os,time\n"
                               "from pathlib import Path\n"
                               f"Path({str(started)!r}).write_text(json.dumps({{'pid':os.getpid()}}))\n"
                               "while True: time.sleep(.1)\n")
                app.chmod(0o755)
                interceptor = root / "observe.mjs"
                interceptor.write_text(
                    "import fs from 'node:fs';import {syncBuiltinESMExports} from 'node:module';"
                    "const open=fs.openSync,read=fs.readSync;let controlFD;"
                    "fs.openSync=(path,...args)=>{"
                    "if(!String(path).endsWith('/49123.restart'))return open(path,...args);"
                    "try{controlFD=open(path,...args);return controlFD;}"
                    "finally{if(fs.existsSync('armed'))fs.writeFileSync('observed','attempt');}};"
                    "fs.readSync=(fd,...args)=>{"
                    "if(fd===controlFD&&fs.existsSync('replace')){"
                    "fs.renameSync('.e2e/fixtures/49123.restart','original-control');"
                    "fs.symlinkSync('../../foreign','.e2e/fixtures/49123.restart');"
                    "fs.unlinkSync('replace');fs.writeFileSync('replaced','complete');}"
                    "return read(fd,...args);};syncBuiltinESMExports();")
                env = {key: os.environ[key] for key in ("PATH", "TMPDIR", "LANG", "TZ")
                       if key in os.environ}
                env.update(PATH=str(tools) + ":" + env["PATH"], TMPDIR=directory,
                           KINOSAIL_E2E_SUBTITLES_BINARY=str(app))
                supervisor = subprocess.Popen(
                    ["node", "--import", interceptor.as_uri(),
                     str(ROOT / "scripts/e2e/fixture.mjs"), "subtitles", "49123"],
                    cwd=root, env=env, stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL, start_new_session=True)
                child_pid = None
                try:
                    receipt = root / ".e2e/fixtures/49123.json"
                    self.wait_for(lambda: started.exists() and receipt.exists())
                    initial = json.loads(receipt.read_text())
                    child_pid = json.loads(started.read_text())["pid"]
                    self.assertEqual(initial["childPID"], child_pid)
                    control = receipt.with_suffix(".restart")
                    sentinel = root / "foreign"
                    sentinel.write_text("foreign sentinel")
                    if kind == "directory":
                        control.mkdir()
                        (control / "sentinel").write_text("directory sentinel")
                    elif kind == "symlink":
                        control.symlink_to(sentinel)
                    elif kind == "fifo":
                        os.mkfifo(control)
                    else:
                        (root / "replace").write_text("owned race control")
                        control.write_text("{}")
                    (root / "armed").write_text("observe rejection")
                    self.wait_for(lambda: (root / "observed").exists())
                    if kind == "replacement":
                        self.wait_for(lambda: (root / "replaced").exists())
                    # A second timer cycle proves the nonregular mailbox did
                    # not throw asynchronously after the observed open attempt.
                    time.sleep(.25)
                    self.assertIsNone(supervisor.poll(), "mailbox rejection killed supervisor")
                    self.assertEqual(json.loads(receipt.read_text()), initial)
                    os.kill(child_pid, 0)
                    self.assert_control(control, kind)
                    supervisor.terminate()
                    self.assertEqual(supervisor.wait(timeout=6), 0)
                    self.wait_for(lambda: not self.alive(child_pid))
                    self.assertFalse(receipt.exists())
                    self.assert_control(control, kind)
                    self.assertEqual(sentinel.read_text(), "foreign sentinel")
                finally:
                    # This test owns the Popen session, never a PID supplied in
                    # a mailbox. Reap even an orphan from the failing version.
                    try:
                        os.killpg(supervisor.pid, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
                    supervisor.wait(timeout=6)

    def assert_control(self, control, kind):
        self.assertTrue(control.exists() or control.is_symlink(), "foreign mailbox was removed")
        if kind == "directory":
            self.assertEqual((control / "sentinel").read_text(), "directory sentinel")
        elif kind == "symlink":
            self.assertTrue(control.is_symlink())
        elif kind == "fifo":
            self.assertTrue(stat.S_ISFIFO(control.stat().st_mode))
        else:
            self.assertTrue(control.is_symlink())
            self.assertEqual(control.read_text(), "foreign sentinel")

    @staticmethod
    def alive(pid):
        try:
            os.kill(pid, 0)
            return True
        except ProcessLookupError:
            return False

    def wait_for(self, condition):
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            if condition():
                return
            time.sleep(.02)
        self.fail("owned process observation did not settle")


if __name__ == "__main__":
    unittest.main()
