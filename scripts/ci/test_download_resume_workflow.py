import shlex
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


class DownloadResumeWorkflowTests(unittest.TestCase):
    def test_actual_player_command_runs_shipping_callback_control_once(self):
        workflow = (ROOT / '.github/workflows/app.yml').read_text()
        step = workflow.split('      - name: Verify setup navigation failure diagnostics\n', 1)[1].split('\n      - ', 1)[0]
        self.assertIn("if: inputs.app == 'player'\n", step)
        self.assertNotIn('matrix.engine', step)
        command = shlex.split(step.split('run: ', 1)[1].strip())
        self.assertEqual(command[:2], ['node', '--test'])
        self.assertEqual(command.count('scripts/testing/download-resume-guard.test.mjs'), 1)
        self.assertTrue((ROOT / 'scripts/testing/download-resume-guard.test.mjs').is_file())
