"""Keep public checkpoint validation controls in the selected Player CI lane."""
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]


class CheckpointContractTest(unittest.TestCase):
    def test_player_diagnostic_step_runs_checkpoint_contract_before_public_flows(self):
        source = (ROOT / '.github/workflows/app.yml').read_text()
        start = source.index('- name: Verify setup navigation failure diagnostics')
        end = source.index('\n      - uses:', start)
        step = source[start:end]
        self.assertIn("if: inputs.app == 'player'", step)
        self.assertIn('node --test', step)
        self.assertIn('scripts/testing/checkpoint-progress.test.mjs', step)
        self.assertLess(start, source.index('Verify public flows with tester-army e2e'))
        self.assertTrue((ROOT / 'scripts/testing/checkpoint-progress.test.mjs').is_file())


if __name__ == '__main__':
    unittest.main()
