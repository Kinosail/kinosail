import os,subprocess,tempfile,unittest
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
class PresentedWiringTests(unittest.TestCase):
 def test_presentation_cannot_use_the_legacy_baseline_owner_recipe(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d);scripts=root/'apps/player/scripts';scripts.mkdir(parents=True)
   for name in ['test-startup-local.py','hls_presented_fixture.py']:(scripts/name).write_bytes((ROOT/'apps/player/scripts'/name).read_bytes())
   result=subprocess.run([os.sys.executable,str(scripts/'test-startup-local.py')],
    env={**os.environ,'KINOSAIL_HLS_PRESENTATION_PROOF':'1','KINOSAIL_STARTUP_BASELINE':'1','PATH':str(root/'no-tools'),'PYTHONDONTWRITEBYTECODE':'1'},
    stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=3)
   self.assertEqual(result.returncode,2,'conflicting recipe rejected before tools')
   self.assertFalse((root/'.verification').exists(),'no conflicting fixture writes')
 def test_unknown_proof_flags_reject_in_actual_cli_before_any_fixture_effect(self):
  for flag in ['', 'true','01','1\n','x'*4096]:
   with tempfile.TemporaryDirectory() as d:
    root=Path(d);scripts=root/'apps/player/scripts';scripts.mkdir(parents=True)
    for name in ['test-startup-local.py','hls_presented_fixture.py']:
     (scripts/name).write_bytes((ROOT/'apps/player/scripts'/name).read_bytes())
    result=subprocess.run([os.sys.executable,str(scripts/'test-startup-local.py')],
      env={**os.environ,'KINOSAIL_HLS_PRESENTATION_PROOF':flag,'PATH':str(root/'no-tools'),'PYTHONDONTWRITEBYTECODE':'1'},
      stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=3)
    self.assertEqual(result.returncode,2,'unknown flag must be rejected before tools')
    self.assertFalse((root/'.verification').exists(),'no fixture setup writes')
    self.assertLessEqual(len(result.stderr),256,'closed diagnostic')
 def test_deep_existing_runner_and_artifact_leaf_own_presentation_proof(self):
  app=(ROOT/'.github/workflows/app.yml').read_text()
  self.assertTrue('KINOSAIL_HLS_PRESENTATION_PROOF: ${{ fromJSON(inputs.plan).deep' in app,'deep opt-in is missing')
  self.assertEqual(app.count('run: python3 apps/player/scripts/test-startup-local.py'),1)
  for name in ['source-frame-map.json','hls-presented-frame.json']:
   self.assertTrue(name in app,'bounded presentation artifact is not retained')
  self.assertEqual(app.count('scripts/testing/hls-presented-frame.test.mjs'),1,'observer controls execute once')
  self.assertEqual(app.count('test_hls_presented_fixture.py'),1,'coded source controls execute once')
  runner=(ROOT/'apps/player/scripts/test-startup-local.py').read_text()
  self.assertTrue("p.name not in {'SHA256SUMS', 'presentation-auth.json'}" in runner,'private state must be excluded from published SUMS')
if __name__=='__main__':unittest.main()
