"""Exact owned relay summary retention and source binding for all six profiles."""
import hashlib,json,os,re,textwrap,unittest
from pathlib import Path
from unittest.mock import patch
ROOT=Path(__file__).resolve().parents[2]
SOURCE=ROOT/'.github/workflows/layout-stability.yml'
COLLECTOR='apps/player/scripts/library_transport_receipt.py'
JOBS=[('library-owner','LIBRARY_PROJECT','library46'),('responsive-owner','RESPONSIVE_PROJECT','responsive99'),('playback-owner','PROFILE_PROJECT','playback'),('offline-owner','PROFILE_PROJECT','offline'),('provider-owner','PROFILE_PROJECT','provider'),('camera-owner','CAMERA_PROJECT','camera19')]
class RelayPublication(unittest.TestCase):
 def block(self,job):return re.split(r'\n  [a-z][a-z0-9-]*:\n', SOURCE.read_text().split('  '+job+':\n',1)[1], maxsplit=1)[0]
 def test_all_six_upload_only_explicit_owned_relay_summary(self):
  for job,_,prefix in JOBS:
   with self.subTest(job=job):
    upload=self.block(job).split('uses: actions/upload-artifact@',1)[1]
    rows=[line.strip() for line in upload.splitlines() if 'relay-' in line]
    self.assertEqual(rows,['${{ runner.temp }}/'+prefix+'-${{ matrix.engine }}/relay-transport.json'])
 def test_actual_source_receipts_bind_collector_before_publication(self):
  import sys
  sys.path.insert(0,str(ROOT/'apps/player/scripts'));self.addCleanup(lambda:sys.path.remove(str(ROOT/'apps/player/scripts')))
  for job,key,_ in JOBS:
   with self.subTest(job=job),patch.dict(os.environ,{key:'webkit','PROOF_REVISION':'fixture-revision'}),patch.object(Path,'write_text') as write:
    recipe=textwrap.dedent(self.block(job).split("python3 - <<'PYTHON'\n",1)[1].split('          PYTHON',1)[0])
    exec(compile(recipe,'relay-owned-source-recipe','exec'),{})
    value=json.loads(write.call_args.args[0]);self.assertEqual(value['sourceSHA256'][COLLECTOR],hashlib.sha256((ROOT/COLLECTOR).read_bytes()).hexdigest())
