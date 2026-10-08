"""Real fixture process protocol; fake CLI proves ordering, not Go encryption."""
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest

ROOT = Path(__file__).resolve().parents[2]

class FixtureRecovery(unittest.TestCase):
    def _run_cli_recovery(self, replace_archive=False):
        with tempfile.TemporaryDirectory() as directory:
            owned=Path(directory);tools=owned/"tools";tools.mkdir()
            ffmpeg=tools/"ffmpeg";ffmpeg.write_text("#!/bin/sh\nexit 0\n");ffmpeg.chmod(0o755)
            starts=owned/"starts.json";calls=owned/"calls.jsonl"
            app=tools/"app"
            app.write_text("#!"+sys.executable+"\n"+"""
import json,os,sys,time
from pathlib import Path
starts=Path(STARTS);calls=Path(CALLS)
args=sys.argv[1:]
if not args:
    tmp=starts.with_suffix(".pending");tmp.write_text(json.dumps({"pid":os.getpid(),"data":os.environ["KINOSAIL_DATA_DIR"]}));tmp.replace(starts)
    while True: time.sleep(.05)
else:
    old=json.loads(starts.read_text())
    try: os.kill(old["pid"],0); alive=True
    except ProcessLookupError: alive=False
    with calls.open("a") as out: out.write(json.dumps({"args":args,"alive":alive,"env":dict(os.environ)})+"\\n")
    if args==["backup"]:
        sys.stdout.buffer.write(b"KINOSAIL-BACKUP-1\\n"+b"synthetic-callback-only"*4)
    elif args==["backup","verify"]:
        assert sys.stdin.buffer.read().startswith(b"KINOSAIL-BACKUP-1")
    elif args==["restore"]:
        body=sys.stdin.buffer.read()
        if not body.startswith(b"KINOSAIL-BACKUP-1"): sys.exit(3)
        Path(os.environ["KINOSAIL_DATA_DIR"],"restored").write_text("persisted")
    else: sys.exit(4)
""".replace("STARTS",repr(str(starts))).replace("CALLS",repr(str(calls))))
            app.chmod(0o755)
            env={k:os.environ[k] for k in ("PATH","LANG","LC_ALL") if k in os.environ}
            env.update(PATH=str(tools)+":"+env["PATH"],TMPDIR=directory,KINOSAIL_E2E_PLAYER_BINARY=str(app),KINOSAIL_BACKUP_KEY="untrusted-host-key")
            child=subprocess.Popen(["node",str(ROOT/"scripts/e2e/fixture.mjs"),"player","49133"],cwd=owned,env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,start_new_session=True)
            def wait(predicate,seconds=3):
                deadline=time.monotonic()+seconds
                while time.monotonic()<deadline:
                    if predicate():return
                    if child.poll() is not None:break
                    time.sleep(.02)
                self.fail("owned fixture did not complete recovery phase; CLI calls="+str(len(calls.read_text().splitlines()) if calls.exists() else 0)+", fixture exit="+str(child.poll()))
            state=owned/".e2e/fixtures/49133.json"
            try:
                wait(lambda:state.exists() and starts.exists())
                original=json.loads(state.read_text())
                for generation,operation in ((0,"backup"),(1,"restore")):
                    current=json.loads(state.read_text())
                    if replace_archive and generation==1:
                        archive=Path(json.loads(starts.read_text())["data"]).parent/"recovery.backup"
                        saved=archive.read_bytes();archive.rename(archive.with_name("captured-backup"));archive.write_bytes(saved)
                    control=state.with_suffix(".restart")
                    control.write_text(json.dumps({"childPID":current["childPID"],"generation":generation,"operation":operation},separators=(",",":")))
                    if replace_archive and generation==1:
                        deadline=time.monotonic()+2
                        while child.poll() is None and time.monotonic()<deadline:time.sleep(.02)
                        rows=[json.loads(line) for line in calls.read_text().splitlines()]
                        self.assertEqual(len(rows),2,"substituted archive reached a CLI operation")
                        self.assertIsNotNone(child.poll(),"rejected archive did not stop its owned supervisor")
                        return
                    wait(lambda:state.exists() and json.loads(state.read_text())["generation"]==generation+1)
                    result=json.loads(state.read_text())
                    self.assertEqual(result["recovery"]["operation"],operation)
                    self.assertTrue(result["recovery"]["verified"])
                    self.assertNotEqual(result["childPID"],current["childPID"])
                rows=[json.loads(line) for line in calls.read_text().splitlines()]
                self.assertEqual([row["args"] for row in rows],[["backup"],["backup","verify"],["backup","verify"],["restore"],["restore"]])
                self.assertTrue(all(not row["alive"] for row in rows),"CLI overlapped live owned server")
                self.assertTrue(all(row["env"]["KINOSAIL_BACKUP_KEY"]!="untrusted-host-key" for row in rows))
                self.assertEqual(len({row["env"]["KINOSAIL_BACKUP_KEY"] for row in rows}),1)
                result=json.loads(state.read_text())
                self.assertTrue(result["recovery"]["corruptRejected"])
                self.assertTrue(result["recovery"]["corruptDataUnchanged"])
                self.assertTrue(result["recovery"]["restored"])
                data=Path(json.loads(starts.read_text())["data"])
                self.assertEqual((data/"restored").read_text(),"persisted")
                self.assertEqual(result["supervisorPID"],original["supervisorPID"])
            finally:
                child.terminate()
                try:child.wait(timeout=6)
                except subprocess.TimeoutExpired:
                    os.killpg(child.pid,signal.SIGKILL);child.wait(timeout=5)
            self.assertFalse(state.exists())


    def test_substituted_ownership_rejects_without_cli_or_foreign_control_mutation(self):
        for mode in ("root-replaced","root-symlink","control-replaced","control-symlink","data-symlink","binary-replaced"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                owned=Path(directory);tools=owned/"tools";tools.mkdir()
                ffmpeg=tools/"ffmpeg";ffmpeg.write_text("#!/bin/sh\nexit 0\n");ffmpeg.chmod(0o755)
                started=owned/"started.json";calls=owned/"calls"
                app=tools/"app";app.write_text("#!"+sys.executable+"\nimport json,os,sys,time\nfrom pathlib import Path\n"
                    +f"started=Path({str(started)!r});calls=Path({str(calls)!r})\n"
                    +"if len(sys.argv)>1: calls.write_text('unexpected CLI');sys.exit(1)\n"
                    +"started.write_text(json.dumps({'root':str(Path(os.environ['KINOSAIL_DATA_DIR']).parent),'pid':os.getpid()}))\n"
                    +"while True: time.sleep(.05)\n");app.chmod(0o755)
                env={k:os.environ[k] for k in ("PATH","LANG","LC_ALL") if k in os.environ}
                env.update(PATH=str(tools)+":"+env["PATH"],TMPDIR=directory,KINOSAIL_E2E_PLAYER_BINARY=str(app))
                child=subprocess.Popen(["node",str(ROOT/"scripts/e2e/fixture.mjs"),"player","49135"],cwd=owned,env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,start_new_session=True)
                state=owned/".e2e/fixtures/49135.json"
                try:
                    deadline=time.monotonic()+3
                    while (not state.exists() or not started.exists()) and time.monotonic()<deadline:time.sleep(.02)
                    self.assertTrue(state.exists());current=json.loads(state.read_text())
                    root=Path(json.loads(started.read_text())["root"]);foreign=owned/"foreign";foreign.mkdir();(foreign/"marker").write_text("preserve")
                    if mode.startswith("root-"):
                        root.rename(root.with_name(root.name+"-captured"))
                        if mode=="root-replaced":root.mkdir();(root/"marker").write_text("preserve")
                        else:root.symlink_to(foreign,target_is_directory=True)
                    elif mode.startswith("control-"):
                        parent=state.parent;parent.rename(parent.with_name("captured"))
                        if mode=="control-replaced":parent.mkdir()
                        else:parent.symlink_to(foreign,target_is_directory=True)
                    elif mode=="data-symlink":
                        (root/"data").rename(root/"captured-data");(root/"data").symlink_to(foreign,target_is_directory=True)
                    else:
                        app.rename(tools/"captured-app");app.write_text("#!/bin/sh\nexit 0\n");app.chmod(0o755)
                    control=state.with_suffix(".restart")
                    raw=json.dumps({"childPID":current["childPID"],"generation":0,"operation":"backup"},separators=(",",":"))
                    control.write_text(raw);time.sleep(.35)
                    self.assertFalse(calls.exists(),"rejected authority reached CLI")
                    self.assertEqual((foreign/"marker").read_text(),"preserve")
                    if mode.startswith("control-"):self.assertEqual(control.read_text(),raw,"foreign parent control was consumed")
                finally:
                    if child.poll() is None:child.terminate()
                    try:child.wait(timeout=6)
                    except subprocess.TimeoutExpired:
                        os.killpg(child.pid,signal.SIGKILL);child.wait(timeout=5)


    def test_cli_recovery_runs_only_after_owned_child_exit_and_preserves_generation(self):
        self._run_cli_recovery()

    def test_substituted_archive_is_rejected_before_restore_cli_effects(self):
        self._run_cli_recovery(replace_archive=True)
