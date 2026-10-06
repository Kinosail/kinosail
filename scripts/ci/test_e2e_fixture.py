"""Process-boundary proof that E2E apps cannot inherit operator credentials."""
import json
import os
import signal
from pathlib import Path
import subprocess
import time
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SAFE = ('PATH', 'TMPDIR', 'TMP', 'TEMP', 'LANG', 'LC_ALL', 'LC_CTYPE', 'TZ', 'SystemRoot', 'WINDIR')
ISOLATED = ('KINOSAIL_LISTEN', 'KINOSAIL_TLS_ENABLED', 'KINOSAIL_MEDIA_DIR',
            'KINOSAIL_DATA_DIR', 'KINOSAIL_CACHE_DIR', 'KINOSAIL_BACKUP_DIR',
            'KINOSAIL_LIBRARIES', 'KINOSAIL_SCAN_INTERVAL', 'KINOSAIL_BACKUP_INTERVAL',
            'KINOSAIL_SERVER_NAME')
HOST_ONLY = ('KINOSAIL_BACKUP_KEY', 'KINOSAIL_PROXY_TOKEN', 'KINOSAIL_DUCKDNS_TOKEN',
             'KINOSAIL_TMDB_TOKEN', 'KINOSAIL_OIDC_CLIENT_SECRET', 'KINOSAIL_SCIM_TOKEN',
             'KINOSAIL_REMOTE_ACCESS_ENABLED', 'KINOSAIL_MEDIA_DIR', 'UNRELATED_API_KEY',
             'HOME', 'NODE_OPTIONS', 'HTTP_PROXY')


class E2EFixtureTests(unittest.TestCase):
    def test_only_safe_runtime_and_isolated_app_configuration_reach_child(self):
        for app in ('player', 'subtitles'):
            with self.subTest(app=app), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                tools = root / 'bin'
                tools.mkdir()
                captures = {}
                for name in ('ffmpeg', 'application'):
                    receipt = root / f'{name}.json'
                    captures[name] = receipt
                    binary = tools / name
                    binary.write_text('#!' + sys.executable + '\nimport json, os\n'
                                      'from pathlib import Path\n'
                                      f'Path({str(receipt)!r}).write_text(json.dumps(dict(os.environ)))\n')
                    binary.chmod(0o755)
                # Synthetic host values only: real host credentials never enter the test.
                env = {key: os.environ[key] for key in SAFE if key in os.environ}
                env.update(dict.fromkeys(HOST_ONLY, 'synthetic-untrusted-host-value'))
                if app == 'player':
                    (root / 'temp').mkdir()
                    (root / 'temp-link').symlink_to(root / 'temp', target_is_directory=True)
                    env['TMPDIR'] = str(root / 'temp-link')
                env['NODE_OPTIONS'] = '--no-warnings'  # Harmless host runtime setting.
                env['PATH'] = str(tools) + ':' + env['PATH']
                env[f'KINOSAIL_E2E_{app.upper()}_BINARY'] = str(tools / 'application')
                result = subprocess.run(['node', str(ROOT / 'scripts/e2e/fixture.mjs'), app, '49123'],
                                        env=env, capture_output=True, text=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                for name, receipt in captures.items():
                    child_env = json.loads(receipt.read_text())
                    # macOS Python adds this locale key even under `env -i`.
                    locale_keys = {'__CF_USER_TEXT_ENCODING'} if sys.platform == 'darwin' else set()
                    allowed = set(SAFE) | locale_keys | (set() if name == 'ffmpeg' else set(ISOLATED))
                    with self.subTest(child=name):
                        self.assertEqual(set(child_env) - allowed, set(), name)
                app_env = json.loads(captures['application'].read_text())
                self.assertEqual(app_env['KINOSAIL_LISTEN'], '127.0.0.1:49123')
                self.assertEqual(app_env['KINOSAIL_TLS_ENABLED'], 'false')
                self.assertEqual(app_env['KINOSAIL_LIBRARIES'], '["Movies"]')
                for key in ('KINOSAIL_MEDIA_DIR', 'KINOSAIL_DATA_DIR', 'KINOSAIL_CACHE_DIR', 'KINOSAIL_BACKUP_DIR'):
                    self.assertTrue(str(Path(app_env[key]).resolve()).startswith(str(Path(directory).parent.resolve()) + '/'))
                    self.assertFalse(Path(app_env[key]).exists(), 'fixture data must be cleaned after child exit')


    def test_media_generator_rejects_unowned_missing_and_ambiguous_roots(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "link").symlink_to(root, target_is_directory=True)
            for args in ([], [""], [str(root)], [str(root / "missing")],
                         [str(root / "link")], ["x" * 4097], [str(root), str(root)]):
                result = subprocess.run([sys.executable, str(ROOT / "scripts/e2e/media-fixture.py"), *args],
                                        capture_output=True, text=True)
                self.assertEqual(result.returncode, 2)
                self.assertIn("invalid disposable media root", result.stderr)
                self.assertEqual(sorted(p.name for p in root.iterdir()), ["link"])


    def test_fixture_invalid_cli_inputs_have_no_allocation_or_process_effects(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for args in ([], ["unknown", "39060"], ["player", ""], ["player", "NaN"],
                         ["player", "1023"], ["player", "65536"], ["player", "39060", "extra"],
                         ["player", "1" * 4097]):
                result = subprocess.run(["node", str(ROOT / "scripts/e2e/fixture.mjs"), *args],
                    cwd=root, env={**os.environ, "TMPDIR": directory}, capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("invalid fixture app/port", result.stderr)
                self.assertEqual(list(root.iterdir()), [], "rejected CLI created fixture state")

    def test_restart_writer_invalid_inputs_have_no_file_effects(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = (ROOT / "scripts/e2e/restart-control.mjs").as_uri()
            cases = [(p, {"generation": 0, "childPID": 123}) for p in ("", "../foreign", "1023", "65536", "x" * 4097)]
            cases += [("49123", body) for body in (None, {}, {"generation": -1, "childPID": 123},
                {"generation": 0, "childPID": 1}, {"generation": 0, "childPID": 123, "unknown": True})]
            for port, body in cases:
                result = subprocess.run(["node", "--input-type=module", "-e",
                    f"import {{ requestRestart }} from {json.dumps(source)}; requestRestart({json.dumps(port)}, {json.dumps(body)});"],
                    cwd=root, capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("invalid fixture restart request", result.stderr)
                self.assertEqual(list(root.iterdir()), [])

    def test_restart_writer_rejects_foreign_symlink_without_mutation(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            target = root / "foreign"
            target.write_text("unchanged")
            state = root / ".e2e/fixtures"
            state.mkdir(parents=True)
            (state / "49123.restart").symlink_to(target)
            source = (ROOT / "scripts/e2e/restart-control.mjs").as_uri()
            result = subprocess.run(["node", "--input-type=module", "-e",
                f"import {{ requestRestart }} from {json.dumps(source)}; requestRestart('49123', {{generation:0, childPID:123}});"],
                cwd=root, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("EEXIST", result.stderr)
            self.assertEqual(target.read_text(), "unchanged")

    def test_supervisor_restarts_only_owned_child_and_preserves_its_data(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            tools = root / "bin"
            tools.mkdir()
            ffmpeg = tools / "ffmpeg"
            ffmpeg.write_text("#!/bin/sh\nexit 0\n")
            ffmpeg.chmod(0o755)
            receipt = root / "started.json"
            app = tools / "app"
            app.write_text("#!" + sys.executable + "\nimport json, os, time\nfrom pathlib import Path\n"
                "data = Path(os.environ['KINOSAIL_DATA_DIR'])\n"
                "marker = data / 'persistent'\nmarker.write_text(marker.read_text() if marker.exists() else 'original')\n"
                + f"Path({str(receipt)!r}).write_text(json.dumps({{'pid':os.getpid(),'data':str(data),'marker':marker.read_text()}}))\n"
                + "while True: time.sleep(0.1)\n")
            app.chmod(0o755)
            env = {key: os.environ[key] for key in SAFE if key in os.environ}
            env.update(PATH=str(tools)+":"+env["PATH"], TMPDIR=directory, KINOSAIL_E2E_PLAYER_BINARY=str(app))
            # Replace the owned regular request with a FIFO immediately before
            # open, reproducing a path-type race without selecting another PID.
            interceptor = root / "open-race.mjs"
            interceptor.write_text("import fs from 'node:fs';import {spawnSync} from 'node:child_process';"
                "import {syncBuiltinESMExports} from 'node:module';const open=fs.openSync;"
                "fs.openSync=(path,flags,mode)=>{"
                "if(String(path).endsWith('/49123.restart')&&fs.existsSync('race-armed')){"
                "fs.unlinkSync(path);spawnSync('mkfifo',[path]);fs.unlinkSync('race-armed');}"
                "return open(path,flags,mode);};"
                # A reader opening the receipt during replacement must still see
                # the previous complete generation, never a truncated JSON file.
                "const write=fs.writeFileSync;fs.writeFileSync=(path,data,options)=>{"
                "if(String(path).endsWith('/49123.json')&&fs.existsSync(path)&&fs.statSync(path).size){"
                "write(path,'',options);JSON.parse(fs.readFileSync(path,'utf8'));}"
                "return write(path,data,options);};syncBuiltinESMExports();")
            child = subprocess.Popen(["node", "--import", interceptor.as_uri(),
                str(ROOT / "scripts/e2e/fixture.mjs"), "player", "49123"],
                cwd=root, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                start_new_session=True)
            try:
                metadata = root / ".e2e/fixtures/49123.json"
                deadline = time.monotonic() + 5
                while not receipt.exists() and time.monotonic() < deadline:
                    time.sleep(0.02)
                self.assertTrue(metadata.exists(), "owned supervisor receipt missing")
                initial = json.loads(metadata.read_text())
                started = json.loads(receipt.read_text())
                self.assertEqual(initial["supervisorPID"], child.pid)
                self.assertEqual(initial["childPID"], started["pid"])
                control = root / ".e2e/fixtures/49123.restart"
                # Malformed, stale and foreign IDs must never reach process control.
                for body in ("null", "{", "x" * 129, "{}", "[]",
                             '{"generation":0,"childPID":1,"childPID":' + str(initial["childPID"]) + '}',
                             '{"generation":0,"childPID":NaN}',
                             json.dumps({"generation": "0", "childPID": initial["childPID"]}),
                             json.dumps({"generation": 0, "childPID": os.getpid()}),
                             json.dumps({"generation": -1, "childPID": initial["childPID"]}),
                             json.dumps({"generation": 0, "childPID": initial["childPID"], "unknown": True})):
                    control.write_text(body)
                    time.sleep(0.15)
                    self.assertEqual(json.loads(metadata.read_text())["generation"], 0)
                    self.assertEqual(json.loads(metadata.read_text())["childPID"], initial["childPID"])
                    self.assertIsNone(child.poll())
                    self.assertEqual(Path(started["data"], "persistent").read_text(), "original")
                (root / "race-armed").write_text("owned fixture fault")
                control.write_text("{}")
                deadline = time.monotonic() + 2
                while control.exists() and time.monotonic() < deadline:
                    time.sleep(0.02)
                self.assertFalse(control.exists(), "nonregular control blocked the supervisor")
                self.assertEqual(json.loads(metadata.read_text())["generation"], 0)
                self.assertEqual(json.loads(metadata.read_text())["childPID"], initial["childPID"])
                self.assertEqual(Path(started["data"], "persistent").read_text(), "original")
                for generation in (1, 2, 3):
                    current = json.loads(metadata.read_text())
                    control.write_text(json.dumps({"generation": current["generation"], "childPID": current["childPID"]}, separators=(",", ":")))
                    deadline = time.monotonic() + 5
                    while time.monotonic() < deadline:
                        current = json.loads(metadata.read_text())
                        if current["generation"] == generation and json.loads(receipt.read_text())["pid"] == current["childPID"]:
                            break
                        time.sleep(0.02)
                    self.assertEqual(current["generation"], generation)
                    self.assertNotEqual(current["childPID"], initial["childPID"])
                    self.assertEqual(json.loads(receipt.read_text())["data"], started["data"])
                    self.assertEqual(json.loads(receipt.read_text())["marker"], "original")
                current = json.loads(metadata.read_text())
                control.write_text(json.dumps({"generation": current["generation"], "childPID": current["childPID"]}, separators=(",", ":")))
                time.sleep(0.2)
                self.assertEqual(json.loads(metadata.read_text())["generation"], 3, "restart budget exceeded")
            finally:
                child.terminate()
                try:
                    child.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    # The test owns this session from Popen, not from a receipt.
                    os.killpg(child.pid, signal.SIGKILL)
                    child.wait(timeout=5)
            self.assertFalse(metadata.exists())
            self.assertFalse(Path(started["data"]).exists())


if __name__ == '__main__':
    unittest.main()
