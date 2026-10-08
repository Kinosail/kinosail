"""Fixed responsive dispatch admits closed discovery before Owner effects."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import textwrap
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / '.github/workflows/layout-stability.yml'
DEFAULTS = ['false', 'false', 'false', 'false', 'primary', 'protocol', 'source-format', 'false']

class ResponsiveWorkflowTests(unittest.TestCase):
    def block(self):
        return re.split(r'\n  [a-z][\w-]*:\n', WORKFLOW.read_text().split('  responsive-owner:\n', 1)[1])[0]

    def recipe(self):
        return textwrap.dedent(self.block().split("python3 - <<'PYTHON'\n", 1)[1].split('          PYTHON', 1)[0])

    def test_actual_selection_rejects_every_unused_override_before_effects(self):
        base = ['workflow_dispatch', 'Responsive', *DEFAULTS]
        command = [sys.executable, str(ROOT / 'scripts/ci/validate-layout-selection.py')]
        result = subprocess.run([*command, *base], capture_output=True, timeout=3)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, b'validated layout selection: Responsive\n')
        bad = [['pull_request', *base[1:]], base[:-1], [*base, 'extra']]
        for index, value in enumerate(('true', 'true', 'true', 'true', 'navigation', 'save-body', 'primary', 'true')):
            values = DEFAULTS.copy(); values[index] = value
            bad.append(['workflow_dispatch', 'Responsive', *values])
        for index in range(10):
            values = base.copy(); values[index] = 'x' * 33; bad.append(values)
        for values in bad:
            result = subprocess.run([*command, *values], capture_output=True, timeout=3)
            self.assertEqual(result.returncode, 2)
            self.assertEqual(result.stdout, b'')

    def test_fixed_three_engine_route_preserves_normal_lanes_and_one_owner(self):
        source = WORKFLOW.read_text(); block = self.block()
        for text in ('needs: selection-admission', "inputs.campaign_proof == 'Responsive'",
                     'engine: [chromium, firefox, webkit]', 'fail-fast: false',
                     'install --frozen-lockfile', 'discovery responsive-shell', '--profile responsive-shell',
                     'timeout --kill-after=2s 60s', '2097153', '2097152', 'if: always()',
                     'if-no-files-found: error', 'fixture-startup.json'):
            self.assertIn(text, block)
        self.assertEqual(block.count('run: ./apps/player/scripts/run-library-profile.sh'), 1)
        self.assertIn('"$RUNNER_TEMP/responsive99-${RESPONSIVE_PROJECT}" responsive-shell', block)
        self.assertLess(block.index('--admit-only'), block.index('Record immutable selection'))
        self.assertNotRegex(block, r'continue-on-error|--retries=[1-9]|test-instance.sh up')
        campaign = source.split('  campaign-proof:\n', 1)[1].split('  hls-followon:', 1)[0]
        self.assertIn("inputs.campaign_proof != 'Responsive'", campaign)
        for job in ('library-owner', 'camera-owner', 'hls-navigation', 'literal-verify', 'native'):
            self.assertIn('  ' + job + ':', source)
        for path in ('apps/player/scripts/responsive_profile_cases.py', 'scripts/ci/test_responsive_workflow.py',
                     'scripts/ci/test_responsive_profile_admission.py',
                     'apps/player/e2e/responsive-failure-witness.mjs', 'scripts/testing/responsive-failure-witness.test.mjs'):
            self.assertIn('"' + path + '"', source)

    def test_actual_recipe_binds_all99_existing_inputs_before_receipt_write(self):
        with patch.dict(os.environ, {'RESPONSIVE_PROJECT': 'webkit', 'PROOF_REVISION': 'fixture-revision'}), \
                patch.object(Path, 'write_text') as write:
            exec(compile(self.recipe(), 'responsive-source-recipe', 'exec'), {})
        receipt = json.loads(write.call_args.args[0])
        self.assertEqual(receipt['project'], 'webkit')
        self.assertEqual(receipt['profile'], 'responsive-shell')
        self.assertEqual(len(receipt['identities']), 99)
        self.assertEqual(len({(row['file'], row['fullTitle']) for row in receipt['identities']}), 99)
        for path in ('apps/player/scripts/responsive_profile_cases.py', 'apps/player/e2e/test-instance-helpers.ts',
                     'scripts/ci/library-internal-relay.mjs', 'apps/player/e2e/layout-audit-helpers.ts',
                     'apps/player/e2e/responsive-failure-witness.mjs', 'apps/player/e2e/playback-state-witness.mjs'):
            self.assertIn(path, receipt['sourceSHA256'])
        for path, digest in receipt['sourceSHA256'].items():
            self.assertEqual(digest, hashlib.sha256((ROOT / path).read_bytes()).hexdigest())
        self.assertIn('--workers=1', receipt['command'])
        self.assertIn('--retries=0', receipt['command'])
        self.assertIn('--repeat-each=1', receipt['command'])

    def test_actual_recipe_writes_only_into_the_fresh_discovery_artifact_directory(self):
        read = Path.read_bytes
        def source_bytes(path):
            return read(ROOT / path)
        with tempfile.TemporaryDirectory() as temporary:
            original = Path.cwd()
            fresh = Path(temporary)
            artifact = fresh / '.verification/responsive-webkit'
            artifact.mkdir(parents=True)
            (artifact / 'discovery.json').write_text('{}')
            sys.path.insert(0, str(ROOT / 'apps/player/scripts'))
            try:
                os.chdir(fresh)
                with patch.dict(os.environ, {'RESPONSIVE_PROJECT': 'webkit', 'PROOF_REVISION': 'fixture-revision'}), \
                        patch.object(Path, 'read_bytes', source_bytes), \
                        patch('subprocess.run', side_effect=AssertionError('fixture process effect')):
                    exec(compile(self.recipe(), 'responsive-source-recipe', 'exec'), {})
                receipt = json.loads((artifact / 'source-receipt.json').read_text())
                self.assertEqual(len(receipt['identities']), 99)
                self.assertEqual(receipt['profile'], 'responsive-shell')
                for path, digest in receipt['sourceSHA256'].items():
                    self.assertEqual(digest, hashlib.sha256(read(ROOT / path)).hexdigest())
                self.assertEqual({p.relative_to(fresh).as_posix() for p in fresh.rglob('*') if p.is_file()},
                                 {'.verification/responsive-webkit/discovery.json',
                                  '.verification/responsive-webkit/source-receipt.json'})
            finally:
                os.chdir(original)
                sys.path.remove(str(ROOT / 'apps/player/scripts'))

    def test_only_the_named_quick_connect_failure_png_is_retained(self):
        block = self.block()
        png_paths = [line.strip() for line in block.splitlines() if '.png' in line]
        self.assertEqual(png_paths, ['${{ runner.temp }}/responsive99-${{ matrix.engine }}/browser-results/**/720-quick-connect-failure.png'])
        self.assertNotIn('browser-results/**/*.png', block)

    def test_missing_source_rejects_before_any_receipt_write(self):
        read = Path.read_bytes
        def missing(path):
            if str(path) == 'apps/player/e2e/test-instance-helpers.ts':
                raise FileNotFoundError('fixed missing source')
            return read(path)
        with patch.dict(os.environ, {'RESPONSIVE_PROJECT': 'webkit', 'PROOF_REVISION': 'fixture-revision'}), \
                patch.object(Path, 'read_bytes', missing), patch.object(Path, 'write_text') as write:
            with self.assertRaises(FileNotFoundError):
                exec(compile(self.recipe(), 'responsive-source-recipe', 'exec'), {})
            write.assert_not_called()

if __name__ == '__main__': unittest.main()
