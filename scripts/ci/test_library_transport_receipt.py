"""Closed owned relay output admission; no container or browser execution."""
import importlib.util,json,tempfile,unittest
from pathlib import Path
SOURCE=Path(__file__).resolve().parents[2]/'apps/player/scripts/library_transport_receipt.py'
class TransportReceipt(unittest.TestCase):
 def load(self):
  spec=importlib.util.spec_from_file_location('transport_receipt',SOURCE);module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module);return module
 def facts(self):
  codes=['ECONNRESET','ECONNREFUSED','ETIMEDOUT','EPIPE','ENETUNREACH','EHOSTUNREACH','unknown']
  return dict(schemaVersion=1,connections=1,connected=1,capacityRejected=0,connectDeadline=0,clientClosed=1,upstreamClosed=1,clientBytes=25,upstreamBytes=0,overflow=False,errors={side:{code:0 for code in codes} for side in ['client','upstream']})
 def test_closed_transport_is_preserved_from_final_owned_line(self):
  with tempfile.TemporaryDirectory() as root:
   source=Path(root)/'private';source.write_text('{}\n'+json.dumps(dict(schemaVersion=1,containerRunning=True,networkInternal=True,targetAdmitted=True,transport=self.facts()))+'\n')
   self.assertEqual(self.load().read(source),dict(schemaVersion=1,available=True,transport=self.facts()))
 def test_unknown_malformed_missing_duplicate_oversized_and_unowned_output_never_leak(self):
  module=self.load()
  with tempfile.TemporaryDirectory() as root:
   path=Path(root)/'private'
   for raw in ['PRIVATE','x'*2049,'{}','{"schemaVersion":1,"schemaVersion":2}',json.dumps({'transport':{'secret':'PRIVATE'}})]:
    path.write_text(raw);facts=module.read(path);self.assertFalse(facts['available']);self.assertNotIn('PRIVATE',json.dumps(facts))
   path.unlink();self.assertFalse(module.read(path)['available'])
   target=Path(root)/'target';target.write_text('{}');path.symlink_to(target);self.assertEqual(module.read(path)['reason'],'unowned')
 def test_counter_boolean_overflow_unknown_error_and_conflicting_connection_counts_reject(self):
  module=self.load()
  for field,value in [('connected',2),('clientBytes',True),('connections',-1),('upstreamBytes',2147483648),('overflow','PRIVATE'),('errors',{'secret':'PRIVATE'})]:
   facts=self.facts();facts[field]=value;self.assertFalse(module.valid(facts))
 def test_unknown_relative_traversal_and_conflicting_output_reject_before_write(self):
  module=self.load()
  with tempfile.TemporaryDirectory() as root:
   root=Path(root);source=root/'relay-failure.json';source.write_text('{}')
   nested=root/'nested';nested.mkdir()
   for output in [Path('relay-transport.json'),root/'unknown.json',nested/'..'/'relay-transport.json',source]:
    with self.assertRaises((ValueError,OSError)): module.write(source,output)
    self.assertFalse((root/'relay-transport.json').exists())
   target=root/'relay-transport.json';target.write_text('retained')
   with self.assertRaises((ValueError,OSError)):module.write(source,target)
   self.assertEqual(target.read_text(),'retained')
