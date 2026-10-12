"""The native build must reject stale generated copy before invoking a compiler.

This isolated command test uses the real Makefile/generator. Only compilation is
stubbed so repository CI can exercise freshness rejection on Linux. Product UI
E2E and successful Swift compilation cannot detect this stale-catalog failure.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class NativeCopyBuildGuardTests(unittest.TestCase):
    def build(self, stale):
        with tempfile.TemporaryDirectory(prefix='kinosail-native-copy-') as directory:
            root = Path(directory)
            app = root / 'apps/player'
            gates = root / 'scripts/tooling'
            gates.mkdir(parents=True)
            for name in ('gate-shell.sh', 'gates-pause.sh'):
                shutil.copy2(ROOT / 'scripts/tooling' / name, gates)
            scripts = app / 'scripts'
            scripts.mkdir(parents=True)
            shutil.copy2(ROOT / 'apps/player/Makefile', app / 'Makefile')
            shutil.copy2(ROOT / 'apps/player/scripts/generate-native-copy.py', scripts)
            shutil.copytree(ROOT / 'apps/player/internal/server/locales', app / 'internal/server/locales')
            languages = root / 'packages/localization'
            languages.mkdir(parents=True)
            shutil.copy2(ROOT / 'packages/localization/languages.go', languages)
            source = app / 'apps/native/Sources'
            source.mkdir(parents=True)
            (source / 'Copy.swift').write_text('let label = "Library"\n')
            subprocess.run(['python3', str(scripts / 'generate-native-copy.py')], check=True,
                           capture_output=True)
            catalogs = {
                'apple': app / 'apps/native/Resources/Localizable.xcstrings',
                'android': app / 'apps/android/app/src/main/assets/interface-copy/es.json',
                'wear': app / 'apps/android/wear/src/main/assets/interface-copy/es.json',
            }
            if stale:
                self.assertNotEqual(json.loads(catalogs[stale].read_text()), {})
                catalogs[stale].write_text('{}\n')
            before = {name: hashlib.sha256(path.read_bytes()).hexdigest()
                      for name, path in catalogs.items()}
            native_scripts = app / 'apps/native/scripts'
            native_scripts.mkdir()
            compiler = native_scripts / 'build-apple.sh'
            compiler.write_text('#!/bin/sh\nprintf "%s\\n" "$1" >> "$QA_COMPILER_CALLS"\n')
            compiler.chmod(0o755)
            marker = root / 'compiler-calls'
            command = ['make', '-s', '-C', str(app), 'client-check']
            result = subprocess.run(command, capture_output=True,
                                    env=os.environ | {'QA_COMPILER_CALLS': str(marker)})
            after = {name: hashlib.sha256(path.read_bytes()).hexdigest()
                     for name, path in catalogs.items()}
            calls = marker.read_text().splitlines() if marker.exists() else []
            destination = os.environ.get('KINOSAIL_QA_NATIVE_GUARD_EVIDENCE')
            if destination:
                output = Path(destination) / (stale or 'fresh')
                output.mkdir(mode=0o700, parents=True, exist_ok=False)
                log = result.stdout + result.stderr
                (output / 'command-private.log').write_bytes(log)
                receipt = {'command': command, 'exit': result.returncode,
                           'compilerCalls': calls, 'catalogsBefore': before, 'catalogsAfter': after,
                           'makefileSHA256': hashlib.sha256((app / 'Makefile').read_bytes()).hexdigest(),
                           'generatorSHA256': hashlib.sha256((scripts / 'generate-native-copy.py').read_bytes()).hexdigest(),
                           'logSHA256': hashlib.sha256(log).hexdigest(),
                           'boundary': 'Real Makefile/generator; compiler stub only, not app proof'}
                (output / 'safe-receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
            return result.returncode, calls, before == after

    def test_stale_catalogs_block_compilation_without_rewriting_copy(self):
        for platform in ('apple', 'android', 'wear'):
            with self.subTest(platform=platform):
                status, calls, unchanged = self.build(platform)
                self.assertNotEqual(status, 0)
                self.assertEqual(calls, [])
                self.assertTrue(unchanged)

    def test_fresh_catalogs_allow_both_native_builds(self):
        status, calls, unchanged = self.build(None)
        self.assertEqual(status, 0)
        self.assertEqual(calls, ['ios', 'tvos'])
        self.assertTrue(unchanged)


if __name__ == '__main__':
    unittest.main()
