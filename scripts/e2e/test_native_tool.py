"""Exercise the actual hosted version callback; no app, tool installation or device."""
import ast
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import time
import signal
import tempfile
import unittest
from unittest.mock import patch
ROOT=Path(__file__).resolve().parents[2]
HELPER=Path(__file__).with_name('native_tool.py')

def callback(lane, platform, project, witness):
 tree=ast.parse((ROOT/f'scripts/{lane}/hosted.py').read_text())
 function=next(n for n in ast.walk(tree) if isinstance(n,ast.FunctionDef) and n.name=='native_tool')
 ns={'subprocess':subprocess,'platform':platform,'project':project,'env':dict(os.environ),'witness':witness,'re':__import__('re')}
 module=None
 if HELPER.exists():
  spec=importlib.util.spec_from_file_location('native_probe',HELPER);module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
  ns['probe_native_tool']=module.probe_native_tool
 exec(compile(ast.Module(body=[function],type_ignores=[]),'actual-hosted-callback','exec'),ns)
 return ns['native_tool'],module

class NativeTool(unittest.TestCase):
 def invoke(self,lane,stdout=b'Xcode 27.1\nBuild version 27A9269\n',stderr=b'',code=0,sleep=False,platform='ios',args=None):
  with tempfile.TemporaryDirectory() as directory:
   project=Path(directory)
   tool=project/('xcodebuild' if platform=='ios' else 'java')
   tool.write_text('#!'+sys.executable+'\nimport os,time\n'+('time.sleep(2)\n' if sleep else '')+'os.write(1,'+repr(stdout)+')\nos.write(2,'+repr(stderr)+')\nraise SystemExit('+str(code)+')\n');tool.chmod(0o700)
   witness={};func,module=callback(lane,platform,project,witness)
   processes=[];original_popen=subprocess.Popen
   def start(*a,**k):
    child=original_popen(*a,**k);processes.append(child);return child
   original_path=os.environ['PATH'];os.environ['PATH']=directory+os.pathsep+original_path
   try:
    func,module=callback(lane,platform,project,witness)
    with patch.object(subprocess,'Popen',side_effect=start), patch.object(module,'DEADLINE_SECONDS',.2 if sleep else 2) if module else patch.object(subprocess,'run',side_effect=subprocess.TimeoutExpired(['xcodebuild'],.05)) if sleep else __import__('contextlib').nullcontext():
     try: result=func(args or (['xcodebuild','-version'] if platform=='ios' else ['java','-version']));error=None
     except Exception as failure:result=None;error=failure
   finally:os.environ['PATH']=original_path
   self.assertTrue(all(child.poll() is not None for child in processes))
   self.assertNotIn('PRIVATE-SENTINEL',json.dumps(witness))
   self.assertNotIn('PRIVATE-SENTINEL',str(error))
   return result,error,witness
 def test_valid_probes_keep_original_versions_and_closed_stage(self):
  for lane in ['e2e-mobile','e2e-tv']:
   result,error,witness=self.invoke(lane)
   self.assertIsNone(error);self.assertEqual(result,{'name':'Xcode','version':'27.1','build':'27A9269'})
   self.assertEqual(witness['nativeToolProbe']['outcome'],'valid')
   self.assertTrue(witness['nativeToolProbe']['combinedTokensRecognized'])
   result,error,witness=self.invoke(lane,b'',b'openjdk version "17.0.17" 2025\nOpenJDK Runtime Environment (build 17.0.17+10)\n',platform='android')
   self.assertIsNone(error);self.assertEqual(result['version'],'17.0.17');self.assertEqual(witness['nativeToolProbe']['outcome'],'valid')
 def test_rejections_keep_stage_without_raw_output(self):
  for lane in ['e2e-mobile','e2e-tv']:
   for kwargs,outcome in [({'stderr':b'PRIVATE-SENTINEL'},'rejected'),({'code':7},'nonzero'),({'stdout':b'\xffPRIVATE-SENTINEL'},'rejected'),({'stdout':b'PRIVATE-SENTINEL'*1000},'overflow'),({'sleep':True},'timeout')]:
    with self.subTest(lane=lane,outcome=outcome):
     result,error,witness=self.invoke(lane,**kwargs);self.assertIsNone(result);self.assertIsNotNone(error)
     self.assertEqual(witness['nativeToolProbe']['outcome'],outcome)
     if outcome=='rejected' and kwargs.get('stderr'):
      self.assertTrue(witness['nativeToolProbe']['stdoutTokensRecognized']);self.assertFalse(witness['nativeToolProbe']['combinedTokensRecognized'])
 def test_unavailable_tool_has_closed_stage_without_launch(self):
  for lane in ['e2e-mobile','e2e-tv']:
   with tempfile.TemporaryDirectory() as directory:
    witness={};func,module=callback(lane,'ios',Path(directory),witness)
    with patch.object(subprocess,'Popen',side_effect=FileNotFoundError('PRIVATE-SENTINEL')):
     with self.assertRaisesRegex(RuntimeError,'Native tool unavailable'):func(['xcodebuild','-version'])
    self.assertEqual(witness['nativeToolProbe']['outcome'],'unavailable')
    self.assertEqual(witness['nativeToolProbe']['stdoutBytes'],0);self.assertEqual(witness['nativeToolProbe']['exitCode'],None)
    self.assertNotIn('PRIVATE-SENTINEL',json.dumps(witness))
 def test_timeout_terminates_child_holding_pipes_after_tool_exit(self):
  for lane in ['e2e-mobile','e2e-tv']:
   with tempfile.TemporaryDirectory() as directory:
    project=Path(directory);marker=project/'child-survived';identity=project/'child-pid'
    tool=project/'xcodebuild'
    tool.write_text('#!'+sys.executable+'\nimport os,time\nfrom pathlib import Path\npid=os.fork()\nif pid==0:\n time.sleep(.7)\n Path('+repr(str(marker))+').write_text("UNJOINED")\n os._exit(0)\nPath('+repr(str(identity))+').write_text(str(pid))\nos._exit(0)\n');tool.chmod(0o700)
    witness={};func,module=callback(lane,'ios',project,witness)
    # Only the fake tool is launched; its descendant must be ended by the probe.
    env_path=os.environ['PATH'];os.environ['PATH']=directory+os.pathsep+env_path
    try:
     func,module=callback(lane,'ios',project,witness)
     with patch.object(module,'DEADLINE_SECONDS',.2):
      with self.assertRaisesRegex(RuntimeError,'Native tool timed out'):func(['xcodebuild','-version'])
     time.sleep(.8);self.assertFalse(marker.exists(),'tool child survived the deadline')
     self.assertEqual(witness['nativeToolProbe']['outcome'],'timeout')
    finally:
     os.environ['PATH']=env_path
     if identity.exists():
      try:os.kill(int(identity.read_text()),signal.SIGKILL)
      except ProcessLookupError:pass
 def test_invalid_tool_rejects_before_process_or_witness(self):
  for lane in ['e2e-mobile','e2e-tv']:
   for platform,args in [('ios',['xcodebuild']),('ios',['java','-version']),('unknown',['xcodebuild','-version']),('ios',['xcodebuild','-version','PRIVATE-SENTINEL'])]:
    with tempfile.TemporaryDirectory() as directory:
     witness={};func,module=callback(lane,platform,Path(directory),witness)
     with patch.object(subprocess,'Popen',side_effect=AssertionError('process tripwire')):
      with self.assertRaises(RuntimeError):func(args)
     self.assertEqual(witness,{})
if __name__=='__main__':unittest.main()
