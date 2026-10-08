"""Execute the SDK input collector; never build or launch browsers locally."""
import hashlib
import os
import shlex
import tempfile
from pathlib import Path
import subprocess
import unittest
ROOT=Path(__file__).resolve().parents[2]
class SDKGapWiring(unittest.TestCase):
    def test_actual_runner_keeps_quick_and_executes_closed_player_gap_mode(self):
        source=(ROOT/'scripts/e2e/run.sh').read_text()
        for args,gaps in [(['player'],False),(['subtitles'],False),(['player','--gaps'],True)]:
            with tempfile.TemporaryDirectory() as directory:
                root=Path(directory); app=root/'scripts/e2e'; app.mkdir(parents=True); tools=root/'tools'; tools.mkdir()
                script=app/'run.sh';script.write_text(source);script.chmod(0o700)
                line=source.split('shasum -a 256',1)[1].split(' > ',1)[0]
                for name in shlex.split(line):
                    target=app/name.replace('*','controlled');target.parent.mkdir(parents=True,exist_ok=True);target.write_text('owned source')
                log=root/'calls'
                for name,body in [('go','printf "fake" > "$3"'),('python3','true')]:
                    path=tools/name;path.write_text('#!/bin/sh\nprintf "%s\\n" "$@" >> "$KINOSAIL_TEST_CALLS"\n'+body+'\n');path.chmod(0o700)
                result=subprocess.run(['bash',str(script),*args],cwd=root,env=dict(os.environ,PATH=str(tools)+':'+os.environ['PATH'],KINOSAIL_TEST_CALLS=str(log)),capture_output=True,text=True,timeout=10)
                self.assertEqual(result.returncode,0,result.stderr)
                calls=log.read_text();self.assertEqual('gap-launch.mjs' in calls,gaps)
                self.assertEqual(list((app/'.e2e/bin').iterdir()),[])
        for args in [['player','unknown'],['subtitles','--gaps'],['player','--gaps','extra']]:
            result=subprocess.run(['bash',str(ROOT/'scripts/e2e/run.sh'),*args],capture_output=True,text=True,timeout=5)
            self.assertEqual(result.returncode,2)
    def test_actual_collector_hashes_every_gap_import(self):
        source=(ROOT/'scripts/e2e/run.sh').read_text()
        line=source.split('shasum -a 256',1)[1].split(' > ',1)[0]
        result=subprocess.run(['bash','-c','shasum -a 256'+line],cwd=ROOT/'scripts/e2e',capture_output=True,text=True,timeout=10)
        self.assertEqual(result.returncode,0,result.stderr)
        rows=dict(row.split('  ',1)[::-1] for row in result.stdout.splitlines())
        for name in ['gap-launch.mjs','public-flow-gap.config.ts','public-flow-gap-engine.ts','deep-tests/public-flow-gap.e2e.ts','pdf-pixels.mjs','fixture-setup.mjs','fixture-response.mjs','recovery-control.mjs']:
            self.assertIn(name,rows)
            self.assertEqual(rows[name],hashlib.sha256((ROOT/'scripts/e2e'/name).read_bytes()).hexdigest())
    def test_player_gaps_run_inside_existing_artifact_and_binary_lifecycle(self):
        source=(ROOT/'scripts/e2e/run.sh').read_text()
        self.assertTrue('if [[ "$gaps" == --gaps ]]; then' in source,'closed optional gap mode required')
        self.assertTrue('node gap-launch.mjs "$run"' in source,'actual gap launcher required')
        self.assertTrue('--output ".e2e/runs/$run/gap-context" --' in source,'existing artifact context required')
        self.assertEqual(source.count('go build'),1)
        config=(ROOT/'scripts/e2e/public-flow-gap.config.ts').read_text()
        self.assertIn("args: ['fixture.mjs', 'player', '{port}']",config)
        self.assertIn("'tests/owner.setup.e2e.ts'",config)
        self.assertNotIn('fixture-recovery',config)
    def test_gap_regression_controls_run_in_required_ci(self):
        source=(ROOT/'.github/workflows/app.yml').read_text()
        command=next(line for line in source.splitlines() if 'run: node --test scripts/testing/player-setup-navigation.test.mjs' in line)
        for name in ['gap-launch.test.mjs','pdf-pixels.test.mjs','sdk-gap-boundary.test.mjs']:
            self.assertEqual(command.count('scripts/e2e/tests/'+name),1)
        self.assertTrue('command -v xvfb-run' in source,'closed gap execution contract required')
        self.assertTrue("DEEP: ${{ fromJSON(inputs.plan).deep && 'true' || 'false' }}" in source,'closed gap execution contract required')
        self.assertTrue('if [[ "$APP" == player && "$DEEP" == true ]]; then' in source,'closed gap execution contract required')
        self.assertTrue('scripts/e2e/run.sh "$APP" --gaps' in source,'closed gap execution contract required')
