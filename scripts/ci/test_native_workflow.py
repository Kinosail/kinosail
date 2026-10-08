"""Native devices run only through the exact premerge manual selection."""
import re
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / '.github/workflows/layout-stability.yml'


class NativeWorkflowTests(unittest.TestCase):
    def test_native_selection_cli_requires_manual_closed_defaults(self):
        args = ['workflow_dispatch', 'Native-platforms', 'false', 'false', 'false', 'false',
                'primary', 'protocol', 'source-format', 'false']
        command = [sys.executable, str(ROOT / 'scripts/ci/validate-layout-selection.py')]
        valid = subprocess.run(command + args, capture_output=True, text=True, timeout=10)
        self.assertEqual(valid.returncode, 0, valid.stderr)
        self.assertEqual(valid.stdout, 'validated layout selection: Native-platforms\n')
        for index, value in ((0, 'pull_request'), (2, 'true'), (3, 'true'), (4, 'true'),
                             (5, 'true'), (6, 'home'), (7, 'save-controls'), (8, 'primary'),
                             (9, 'true'), (1, 'native-platforms')):
            changed = args.copy()
            changed[index] = value
            result = subprocess.run(command + changed, capture_output=True, text=True, timeout=10)
            self.assertEqual(result.returncode, 2, (index, value))

    def test_native_only_calls_four_sha_bound_owned_lanes(self):
        source = WORKFLOW.read_text()
        for job, child in (('native-phone', 'native-phone-e2e.yml'), ('native-tv', 'native-tv-e2e.yml')):
            block = re.split(r'\n  [a-z][a-z-]*:\n', source.split('  ' + job + ':\n', 1)[1], maxsplit=1)[0]
            self.assertIn('needs: selection-admission', block)
            self.assertIn("if: github.event_name == 'workflow_dispatch' && inputs.campaign_proof == 'Native-platforms'", block)
            self.assertIn('uses: ./.github/workflows/' + child, block)
            self.assertNotIn('continue-on-error', block)
            child_source = (ROOT / '.github/workflows' / child).read_text()
            self.assertIn('  workflow_call:\n', child_source)
            self.assertEqual(child_source.count('ref: ${{ github.sha }}'), 2)
            self.assertIn('npm ci --ignore-scripts --no-audit --no-fund', child_source)
            self.assertIn('python3 cleanup.py', child_source)
            self.assertIn('node publish.mjs', child_source)
            self.assertNotIn('continue-on-error', child_source)
        campaign = source.split('  campaign-proof:\n', 1)[1].split('\n  hls-followon:', 1)[0]
        self.assertIn("inputs.campaign_proof != 'Native-platforms'", campaign)
        self.assertIn('default: none', source)
