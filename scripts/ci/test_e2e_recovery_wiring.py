"""Execute the actual SDK source collector, without app build or runner."""
import hashlib
from pathlib import Path
import subprocess
import unittest
ROOT=Path(__file__).resolve().parents[2]
class RecoverySourceClosure(unittest.TestCase):
    def test_actual_collector_binds_executed_recovery_import(self):
        line=(ROOT/"scripts/e2e/run.sh").read_text().split("shasum -a 256",1)[1].split(" > ",1)[0]
        result=subprocess.run(["bash","-c","shasum -a 256"+line],cwd=ROOT/"scripts/e2e",capture_output=True,text=True,timeout=10)
        self.assertEqual(result.returncode,0,result.stderr)
        rows=dict(row.split("  ",1)[::-1] for row in result.stdout.splitlines())
        self.assertIn("recovery-control.mjs",rows)
        self.assertEqual(rows["recovery-control.mjs"],hashlib.sha256((ROOT/"scripts/e2e/recovery-control.mjs").read_bytes()).hexdigest())
    def test_descriptor_helper_is_bound_and_called_with_owned_fd(self):
        source=(ROOT/"scripts/e2e/fixture.mjs").read_text()
        collector=(ROOT/"scripts/e2e/run.sh").read_text()
        self.assertIn("recovery-data-digest.py",collector)
        self.assertIn("stdio: ['ignore', 'pipe', 'ignore', rootFD]",source)
        self.assertEqual(source.count("dataDigest(rootFD)"),3)
        self.assertNotIn("opendirSync",source)
    def test_required_player_controls_execute_the_actual_mailbox_suite_once(self):
        source=(ROOT/".github/workflows/app.yml").read_text()
        command=next(line for line in source.splitlines() if "run: node --test scripts/testing/player-setup-navigation.test.mjs" in line)
        self.assertEqual(command.count("scripts/e2e/tests/recovery-control.test.mjs"),1)
        self.assertEqual(command.count("scripts/e2e/tests/fixture-reader.test.mjs"),1)
