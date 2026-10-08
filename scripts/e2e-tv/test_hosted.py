import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch
P=Path(__file__).parent

def load(name):
 spec=importlib.util.spec_from_file_location(name,P/(name+'.py'));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m

class TVAdmission(unittest.TestCase):
 def test_tv_metadata_admission(self):
  m=load('preflight')
  runtime='com.apple.CoreSimulator.SimRuntime.tvOS-27-0'
  self.assertEqual(m.apple_runtime({'runtimes':[{'identifier':runtime,'isAvailable':True,'version':'27.0'}]}, {'devicetypes':[{'identifier':m.TV_TYPE}]}),runtime)
  for runtimes,types in [({'runtimes':[]},{'devicetypes':[{'identifier':m.TV_TYPE}]}),({'runtimes':[{'identifier':runtime.replace('tvOS','iOS'),'isAvailable':True,'version':'27.0'}]},{'devicetypes':[{'identifier':m.TV_TYPE}]}),({'runtimes':[{'identifier':runtime,'isAvailable':True,'version':'27.0'}]},{'devicetypes':[]})]:
   with self.assertRaises(RuntimeError):m.apple_runtime(runtimes,types)
 def test_android_rejects_phone_or_ambiguous_metadata(self):
  m=load('preflight');valid=f'<sdk-repository><localPackage path="{m.TV_IMAGE}"><type-details><api-level>36</api-level><tag><id>android-tv</id></tag><abi>x86_64</abi></type-details></localPackage></sdk-repository>'
  self.assertEqual(m.android_image(valid.encode()),m.TV_IMAGE)
  for xml in [b'',valid.replace('android-tv','google_apis').encode(),valid.replace('>36<','>35<').encode(),valid.replace('x86_64','arm64-v8a').encode(),valid.replace('</sdk-repository>',valid[16:]).encode(),b'x'*65537]:
   with self.assertRaises(RuntimeError):m.android_image(xml)
 def test_pending_create_requires_exact_tv_type_runtime(self):
  m=load('simulator');owner={'run':'123-1','platform':'ios','complete':False,'device':None}
  with patch.object(m,'inventory',return_value=([], 'a'*64)):
   m.prepare_creation(owner,'com.apple.CoreSimulator.SimRuntime.tvOS-27-0')
  self.assertEqual(m.validate_pending(owner)['name'],'Kinosail-TV-E2E-123-1')
  owner['creationPending']['type']='com.apple.CoreSimulator.SimDeviceType.iPhone-17-Pro'
  with self.assertRaises(RuntimeError):m.validate_pending(owner)
 def test_existing_tv_collision_has_no_cleanup_mutation(self):
  m=load('simulator');owner={'run':'123-1','platform':'ios','complete':False,'device':None}
  with patch.object(m,'inventory',return_value=([('foreign',{'name':'Kinosail-TV-E2E-123-1'})], 'a'*64)):
   with self.assertRaises(RuntimeError):m.prepare_creation(owner,'com.apple.CoreSimulator.SimRuntime.tvOS-27-0')
  self.assertNotIn('creationPending',owner)
 def test_unknown_hosted_profile_rejects_before_commands(self):
  m=load('hosted')
  with patch.object(m.sys,'argv',['hosted.py','watchos']),patch.object(m.subprocess,'Popen',side_effect=AssertionError('process tripwire')):
   with self.assertRaises(RuntimeError):m.main()
if __name__=='__main__':unittest.main()
