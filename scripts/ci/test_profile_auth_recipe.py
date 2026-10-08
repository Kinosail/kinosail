"""Executed auth-form imports must be bound before profile receipt publication."""
import ast
import hashlib
import json
import os
from pathlib import Path
import textwrap
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
AUTH = 'scripts/testing/auth-form-navigation.ts'
JOBS = [('library-owner', 'LIBRARY_PROJECT', 46),
        ('playback-owner', 'PROFILE_PROJECT', 25), ('offline-owner', 'PROFILE_PROJECT', 17),
        ('provider-owner', 'PROFILE_PROJECT', 15), ('responsive-owner', 'RESPONSIVE_PROJECT', 99)]

class ProfileAuthRecipeTests(unittest.TestCase):
    def test_actual_playback_recipe_binds_startup_observers_before_write(self):
        helpers = ['apps/player/e2e/responsive-failure-witness.mjs', 'apps/player/e2e/startup-media-hold.mjs']
        with patch.dict(os.environ, {'PROFILE_PROJECT': 'webkit', 'PROOF_REVISION': 'fixture-revision'}), patch.object(Path, 'write_text') as write:
            exec(compile(self.recipe('playback-owner'), 'actual-playback-recipe', 'exec'), {})
            receipt = json.loads(write.call_args.args[0])
            for name in helpers:
                self.assertIn(name, receipt['sourceSHA256'])
                self.assertEqual(receipt['sourceSHA256'][name], hashlib.sha256((ROOT / name).read_bytes()).hexdigest())
        read = Path.read_bytes
        for name in helpers:
            def missing(path):
                if str(path) == name:
                    raise FileNotFoundError('missing startup observer')
                return read(path)
            with patch.dict(os.environ, {'PROFILE_PROJECT': 'webkit', 'PROOF_REVISION': 'fixture-revision'}), patch.object(Path, 'read_bytes', missing), patch.object(Path, 'write_text') as write:
                with self.assertRaises(FileNotFoundError):
                    exec(compile(self.recipe('playback-owner'), 'actual-playback-recipe', 'exec'), {})
                write.assert_not_called()

    def recipe(self, job):
        source = (ROOT / '.github/workflows/layout-stability.yml').read_text()
        block = source.split('  ' + job + ':\n', 1)[1]
        return textwrap.dedent(block.split("python3 - <<'PYTHON'\n", 1)[1].split('          PYTHON', 1)[0])

    def test_actual_profile_recipes_bind_the_executed_auth_import(self):
        for job, key, count in JOBS:
            with self.subTest(job=job), patch.dict(os.environ, {key: 'webkit', 'PROOF_REVISION': 'fixture-revision'}), patch.object(Path, 'write_text') as write:
                exec(compile(self.recipe(job), 'actual-profile-recipe', 'exec'), {})
                receipt = json.loads(write.call_args.args[0])
                self.assertEqual(len(receipt['identities']), count)
                self.assertTrue(AUTH in receipt['sourceSHA256'], 'executed auth import absent from receipt')
                self.assertEqual(receipt['sourceSHA256'][AUTH], hashlib.sha256((ROOT / AUTH).read_bytes()).hexdigest())

    def test_missing_auth_import_prevents_any_profile_receipt_write(self):
        read = Path.read_bytes
        def missing(path):
            if str(path) == AUTH:
                raise FileNotFoundError('missing executed auth helper')
            return read(path)
        for job, key, _ in JOBS:
            with self.subTest(job=job), patch.dict(os.environ, {key: 'webkit', 'PROOF_REVISION': 'fixture-revision'}), patch.object(Path, 'read_bytes', missing), patch.object(Path, 'write_text') as write:
                with self.assertRaises(FileNotFoundError):
                    exec(compile(self.recipe(job), 'actual-profile-recipe', 'exec'), {})
                write.assert_not_called()

    def test_actual_native_layout_input_collector_binds_auth_helper(self):
        source = (ROOT / 'scripts/testing/test-layout-stability-local.py').read_text()
        assignment = next(node for node in ast.parse(source).body if isinstance(node, ast.Assign) and any(isinstance(target, ast.Name) and target.id == 'initial_scripts' for target in node.targets))
        scope = {'root': ROOT, 'hashlib': hashlib}
        exec(compile(ast.Module(body=[assignment], type_ignores=[]), 'actual-layout-inputs', 'exec'), scope)
        self.assertEqual(scope['initial_scripts']['auth-form-navigation.ts'], hashlib.sha256((ROOT / AUTH).read_bytes()).hexdigest())
