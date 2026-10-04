"""Process-boundary proof that E2E apps cannot inherit operator credentials."""
import json
import os
from pathlib import Path
import subprocess
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


if __name__ == '__main__':
    unittest.main()
