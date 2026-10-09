"""Exercise the actual private codec wrapper with a nonmedia sentinel executable."""
import ast
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'apps/player/scripts'))
from hls_followon_public import check
from hls_remaining_capture import capture_codec


class ActualRefillCaptureTests(unittest.TestCase):
    def wrapper(self, directory, source, executable, pacing, installation=False):
        path = ROOT / 'apps/player/scripts/test-hls-remaining.py'
        journey = next(n for n in ast.parse(path.read_text()).body
            if isinstance(n, ast.FunctionDef) and n.name == 'journey')
        statement = next(n for n in journey.body if isinstance(n, ast.If) and
            any(isinstance(v, ast.Name) and v.id == 'capture_codec' for v in ast.walk(n)))
        namespace = dict(env={}, case={}, directory=directory, source=source, pacing=pacing,
            installation=installation, capture_codec=capture_codec)
        with patch.object(shutil, 'which', return_value=str(executable)):
            exec(compile(ast.Module(body=[statement], type_ignores=[]), str(path), 'exec'), namespace)
        return namespace

    def attempt(self, pacing, edit=None, existing=False):
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            source = directory / 'Fixture.flac'; source.write_bytes(b'not-media')
            executable = directory / 'sentinel'
            output = directory / 'executed.json'
            executable.write_text('#!' + sys.executable + '\nimport json,sys\n'
                + 'open(' + repr(str(output)) + ',"w").write(json.dumps(sys.argv[1:]))\n')
            executable.chmod(0o700)
            ns = self.wrapper(directory, source, executable, pacing)
            self.assertIn('KINOSAIL_FFMPEG', ns['env'])
            capture = directory / 'refill-recipe-private.json'
            if existing:
                capture.write_bytes(b'original-private-certificate')
            args = ['-ss','0.000','-i',str(source),'-hls_time','2','-start_number','4','output']
            if edit:
                edit(args)
            old = os.umask(0o022)
            try:
                result = subprocess.run([ns['env']['KINOSAIL_FFMPEG'], *args],
                    capture_output=True, timeout=3)
            finally:
                os.umask(old)
            if existing or edit:
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(output.exists())
                self.assertEqual(capture.read_bytes(), b'original-private-certificate') if existing else self.assertFalse(capture.exists())
            else:
                self.assertEqual(result.returncode, 0)
                actual = json.loads(capture.read_bytes())
                self.assertEqual(actual, json.loads(output.read_bytes()))
                self.assertEqual(capture.stat().st_mode & 0o777, 0o600)
                if pacing is None:
                    self.assertEqual(actual, args)
                else:
                    self.assertEqual(actual[actual.index('-readrate')+1], str(pacing))
                self.assertEqual(ns['case']['actualCodecInvocation']['executableSHA256'],
                    hashlib.sha256(executable.read_bytes()).hexdigest())

    def test_unpaced_actual_arguments_are_captured_without_pacing(self):
        self.attempt(None)

    def test_paced_actual_arguments_keep_the_selected_readrate(self):
        self.attempt(1.25)

    def test_existing_private_capture_is_not_replaced(self):
        self.attempt(1.25, existing=True)

    def test_oversized_actual_capture_does_not_execute_or_write(self):
        for edit in [lambda a: a.append('x'*4097), lambda a: a.extend(['x']*100),
            lambda a: a.extend(['x'*4000]*3)]:
            with self.subTest(edit=edit):
                self.attempt(1.25, edit)

    def test_installation_wrapper_retains_a_valid_scoped_counterfactual(self):
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp); source = directory / 'Fixture.flac'
            source.write_bytes(b'not-media')
            executable = directory / 'sentinel'; executable.write_bytes(b'not-a-codec')
            ns = self.wrapper(directory, source, executable, 0.75, installation=True)
            wrapper = Path(ns['env']['KINOSAIL_FFMPEG']).read_text()
            ast.parse(wrapper)
            self.assertIn('from hls_remaining_installation import installed_refill', wrapper)

    def test_invalid_pacing_rejects_before_wrapper_or_environment_mutation(self):
        for pacing in [False, True, float('nan'), float('inf'), -1, 0, 2, '1.25']:
            with self.subTest(pacing=pacing), tempfile.TemporaryDirectory() as tmp:
                directory = Path(tmp); env, case = {}, {}
                with self.assertRaises(RuntimeError):
                    capture_codec(directory, directory/'Fixture.flac', pacing, False, env, case)
                self.assertEqual(list(directory.iterdir()), [])
                self.assertEqual(env, {})
                self.assertEqual(case, {})


if __name__ == '__main__':
    unittest.main()
