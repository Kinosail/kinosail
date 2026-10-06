"""Public initial receipt visibility, exclusive ownership and failed launch proof."""
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


class InitialReceiptTests(unittest.TestCase):
    def fixture(self, root):
        tools = root / 'tools'
        tools.mkdir()
        for name, contents in {
            'ffmpeg': '#!/bin/sh\nexit 0\n',
            'app': '#!' + sys.executable + '\nimport time\nwhile True: time.sleep(0.1)\n',
        }.items():
            path = tools / name
            path.write_text(contents)
            path.chmod(0o755)
        state = root / '.e2e/fixtures'
        state.mkdir(parents=True)
        env = {key: os.environ[key] for key in ('PATH', 'LANG', 'LC_ALL') if key in os.environ}
        env.update(PATH=str(tools) + ':' + env['PATH'], TMPDIR=str(root),
                   KINOSAIL_E2E_SUBTITLES_BINARY=str(tools / 'app'))
        return state, env

    def process(self, root, env, interceptor=None):
        args = ['node']
        if interceptor:
            args += ['--import', interceptor.as_uri()]
        args += [str(ROOT / 'scripts/e2e/fixture.mjs'), 'subtitles', '49124']
        return subprocess.Popen(args, cwd=root, env=env, stdout=subprocess.DEVNULL,
                                stderr=subprocess.PIPE, text=True, start_new_session=True)

    def stop(self, child):
        if child.poll() is None:
            child.terminate()
        try:
            child.communicate(timeout=5)
        except subprocess.TimeoutExpired:
            os.killpg(child.pid, signal.SIGKILL)
            child.communicate(timeout=5)

    def test_initial_publication_is_complete_at_first_visibility(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            state, env = self.fixture(root)
            observed = root / 'observed.json'
            interceptor = root / 'observe.mjs'
            # Observe synchronously after the actual public directory entry is
            # created: the publisher cannot advance before the reader parses it.
            interceptor.write_text("import fs from 'node:fs';import {syncBuiltinESMExports} from 'node:module';"
                "const write=fs.writeFileSync,link=fs.linkSync,rename=fs.renameSync;"
                "function observe(path){if(!String(path).endsWith('/49124.json'))return;"
                "let value;try{value={valid:true,state:JSON.parse(fs.readFileSync(path,'utf8'))};}"
                "catch{value={valid:false};}"
                f"write({json.dumps(str(observed))},JSON.stringify(value));"
                "if(!value.valid)throw new Error('initial receipt is incomplete');}"
                "fs.writeFileSync=(path,...args)=>{const result=write(path,...args);observe(path);return result;};"
                "fs.linkSync=(from,to)=>{const result=link(from,to);observe(to);return result;};"
                "fs.renameSync=(from,to)=>{const result=rename(from,to);observe(to);return result;};"
                "syncBuiltinESMExports();")
            child = self.process(root, env, interceptor)
            try:
                deadline = time.monotonic() + 5
                while not observed.exists() and child.poll() is None and time.monotonic() < deadline:
                    time.sleep(0.01)
                self.assertTrue(observed.exists(), 'no public receipt was observed')
                value = json.loads(observed.read_text())
                self.assertTrue(value['valid'], 'first visible public receipt was empty or malformed JSON')
                receipt = value['state']
                self.assertEqual(set(receipt), {'app', 'port', 'supervisorPID', 'childPID', 'generation'})
                self.assertEqual(receipt['supervisorPID'], child.pid)
                self.assertGreater(receipt['childPID'], 1)
                self.assertEqual(receipt['generation'], 0)
                self.assertEqual(receipt['app'], 'subtitles')
                self.assertEqual(receipt['port'], '49124')
                self.assertEqual(json.loads((state / '49124.json').read_text()), receipt)
            finally:
                self.stop(child)
            self.assertFalse((state / '49124.json').exists())
            self.assertFalse((state / '49124.json.pending').exists())
            self.assertFalse(list(root.glob('kinosail-e2e-*')))

    def test_foreign_initial_files_are_preserved_without_launch(self):
        for name in ('49124.json', '49124.json.pending', '49124.restart'):
            for symlink in (False, True):
                with self.subTest(name=name, symlink=symlink), tempfile.TemporaryDirectory() as directory:
                    root = Path(directory)
                    state, env = self.fixture(root)
                    foreign = root / 'foreign'
                    foreign.write_text('foreign-owned-by-someone-else')
                    target = state / name
                    if symlink:
                        target.symlink_to(foreign)
                    else:
                        target.write_text(foreign.read_text())
                    # The application must never be executed after a failed claim.
                    app = root / 'tools/app'
                    app.write_text('#!' + sys.executable + '\nfrom pathlib import Path\n'
                                   + f'Path({str(root / "unexpected-child")!r}).touch()\n')
                    child = self.process(root, env)
                    _, error = child.communicate(timeout=5)
                    self.assertNotEqual(child.returncode, 0, error)
                    self.assertFalse((root / 'unexpected-child').exists())
                    self.assertEqual(target.read_text(), 'foreign-owned-by-someone-else')
                    self.assertEqual(target.is_symlink(), symlink)
                    self.assertEqual(foreign.read_text(), 'foreign-owned-by-someone-else')
                    self.assertEqual(sorted(path.name for path in state.iterdir()), [name])
                    self.assertFalse(list(root.glob('kinosail-e2e-*')))

    def test_failed_spawn_leaves_no_public_receipt_or_owned_temporary_files(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            state, env = self.fixture(root)
            env['KINOSAIL_E2E_SUBTITLES_BINARY'] = str(root / 'missing-application')
            child = self.process(root, env)
            _, error = child.communicate(timeout=5)
            self.assertNotEqual(child.returncode, 0, error)
            self.assertEqual(list(state.iterdir()), [])
            self.assertFalse(list(root.glob('kinosail-e2e-*')))

    def test_restart_claim_conflict_preserves_foreign_pending_and_cleans_owned_state(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            state, env = self.fixture(root)
            child = self.process(root, env)
            metadata = state / '49124.json'
            pending = state / '49124.json.pending'
            try:
                deadline = time.monotonic() + 5
                while (not metadata.exists() or pending.exists()) and child.poll() is None and time.monotonic() < deadline:
                    time.sleep(0.01)
                receipt = json.loads(metadata.read_text())
                # A foreign claimant arrives between completed generations.
                with pending.open('x') as claim:
                    claim.write('foreign pending claim')
                (state / '49124.restart').write_text(json.dumps({
                    'childPID': receipt['childPID'], 'generation': receipt['generation']}, separators=(',', ':')))
                _, error = child.communicate(timeout=7)
                self.assertNotEqual(child.returncode, 0, error)
                self.assertEqual(pending.read_text(), 'foreign pending claim')
                self.assertEqual(sorted(path.name for path in state.iterdir()), [pending.name])
                self.assertFalse(list(root.glob('kinosail-e2e-*')))
            finally:
                self.stop(child)

    def test_initial_publication_conflict_preserves_foreign_receipt_and_stops_child(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            state, env = self.fixture(root)
            child_receipt = root / 'child.json'
            ready = root / 'ready'
            (root / 'tools/app').write_text('#!' + sys.executable + '\nimport signal,time\nfrom pathlib import Path\n'
                + 'signal.signal(signal.SIGTERM,signal.SIG_IGN)\n'
                + f'Path({str(ready)!r}).touch()\nwhile True: time.sleep(0.1)\n')
            interceptor = root / 'conflict.mjs'
            interceptor.write_text("import fs from 'node:fs';import cp from 'node:child_process';"
                "import {syncBuiltinESMExports} from 'node:module';"
                "const spawn=cp.spawn,link=fs.linkSync;"
                "cp.spawn=(...args)=>{const child=spawn(...args);"
                f"fs.writeFileSync({json.dumps(str(child_receipt))},JSON.stringify({{pid:child.pid}}));"
                + f"const until=Date.now()+2000;while(!fs.existsSync({json.dumps(str(ready))})&&Date.now()<until)"
                + "Atomics.wait(new Int32Array(new SharedArrayBuffer(4)),0,0,5);"
                "return child;};"
                "fs.linkSync=(from,to)=>{if(String(to).endsWith('/49124.json')){"
                "fs.writeFileSync(to,'foreign publication',{flag:'wx'});"
                "fs.writeFileSync(to.replace('.json','.restart'),'foreign control',{flag:'wx'});}return link(from,to);};"
                "syncBuiltinESMExports();")
            child = self.process(root, env, interceptor)
            try:
                _, error = child.communicate(timeout=7)
                self.assertNotEqual(child.returncode, 0, error)
                self.assertTrue(ready.exists(), 'unresponsive child handshake was not reached')
                self.assertEqual((state / '49124.json').read_text(), 'foreign publication')
                self.assertEqual((state / '49124.restart').read_text(), 'foreign control')
                self.assertFalse((state / '49124.json.pending').exists())
                self.assertFalse(list(root.glob('kinosail-e2e-*')))
                pid = json.loads(child_receipt.read_text())['pid']
                with self.assertRaises(ProcessLookupError):
                    os.kill(pid, 0)
            finally:
                self.stop(child)


if __name__ == '__main__':
    unittest.main()
