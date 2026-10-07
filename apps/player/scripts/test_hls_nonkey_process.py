"""Real small process trees protect renderer timeout/descendant ownership gaps."""
import subprocess
import json
import sys
from pathlib import Path
import tempfile
import unittest
from unittest import mock
from hls_nonkey_process import owned_command


CHILD = "import subprocess,sys,time; subprocess.Popen([sys.executable,'-c','import time;time.sleep(30)'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);"


class AACPartialEndpointEvidence(unittest.TestCase):
    def test_late_decode_timeout_retains_every_completed_count_and_hash(self):
        import hls_nonkey_diagnostics as module
        keys = ['sourceNative', 'sourceSeekNative', 'publicNative', 'publicResampledEOF']
        phases = ['source_native', 'source_seek', 'public_native', 'resampled_eof']
        clock = {'sampleRate':48000,'codecName':'aac','channels':2,'frameRows':[[0,100]],'decodedSamplesAtSourceRate':100}
        facts = {'samples':100,'sha256':'a'*64}
        for cut in range(4):
            case = {'markedAAC':{'decoderBudgetControl':{'sourceClock':clock,'publicClock':clock}},'failures':['historical_failure']}
            completed = [(facts,b'\0'*400)]*cut
            with mock.patch.object(module,'decode_audio_pcm',side_effect=completed+[subprocess.TimeoutExpired('private',40)]):
                module.audio_endpoint_proof(Path('source'),Path('public'),12.5,case)
            evidence = case['markedAAC']['nativeEndpointControl']
            self.assertFalse(evidence['complete'])
            self.assertEqual(evidence['phase'],phases[cut])
            self.assertEqual([k for k in keys if k in evidence],keys[:cut])
            self.assertTrue(all(evidence[k]==facts for k in keys[:cut]))
            self.assertEqual(case['failures'],['historical_failure','aac_endpoint_unqualified'])


class CaptureFailureEvidence(unittest.TestCase):
    def test_closed_failure_schema_preserves_counts_and_never_exception_text(self):
        module = (Path(__file__).resolve().parents[1] / 'e2e/hls-renderer-timeline.mjs').as_uri()
        script = """import {recordCaptureFailure} from MODULE;
const proof = {captureErrors: 0, rows: []};
const media = {readyState: 2, videoWidth: 640, videoHeight: 360};
recordCaptureFailure(proof, media, {name:'InvalidStateError', message:'private URL credential'}, 'event-construction', 'before-gesture', 12.5);
for (let i=0;i<23;i++) recordCaptureFailure(proof, {readyState:true,videoWidth:Infinity,videoHeight:-1}, {name:'private',message:'private'}, 'foreign', 'foreign', NaN);
process.stdout.write(JSON.stringify(proof));""".replace('MODULE', json.dumps(module))
        run = subprocess.run(['node', '--input-type=module', '-e', script], capture_output=True, timeout=10)
        self.assertEqual(run.returncode, 0)
        facts = json.loads(run.stdout)
        self.assertEqual(facts['captureErrors'], 24)
        self.assertEqual(len(facts['captureFailures']), 16)
        self.assertEqual(facts['captureFailures'][0], {'stage':'event-construction', 'phase':'before-gesture',
            'exceptionClass':'InvalidStateError', 'readyState':2, 'width':640, 'height':360, 'callbacks':0, 'nativeTime':12.5})
        self.assertNotIn('private', run.stdout.decode())
        self.assertEqual(facts['captureFailures'][1]['exceptionClass'], 'UnknownError')
        self.assertIsNone(facts['captureFailures'][1]['readyState'])
        self.assertIsNone(facts['captureFailures'][1]['nativeTime'])

    def test_control_resume_reset_is_closed_bounded_and_before_network_effects(self):
        module = (Path(__file__).resolve().parents[1] / 'e2e/hls-direct-delivery.mjs').as_uri()
        script = """import {resetProofResume} from MODULE;
const input={url:'http://localhost:12345',itemID:'a'.repeat(16),token:'a'.repeat(16),resumeSeconds:12.5};
let calls=0,options;
const context={request:{put:async(url,value)=>{calls++;options=value;return{status:()=>200,body:async()=>Buffer.from('{"seconds":12.5}')}}}};
const ok=await resetProofResume(context,input,Date.now()+20000);
for (const change of [{url:'https://foreign'},{itemID:'foreign'},{resumeSeconds:13},{token:'private'}]) {try{await resetProofResume(context,{...input,...change},Date.now()+20000)}catch{}}
process.stdout.write(JSON.stringify({ok,calls,redirects:options.maxRedirects,timeout:options.timeout,seconds:options.data.seconds}));""".replace('MODULE',json.dumps(module))
        run=subprocess.run(['node','--input-type=module','-e',script],capture_output=True,timeout=10)
        self.assertEqual(run.returncode,0)
        self.assertEqual(json.loads(run.stdout),{'ok':True,'calls':1,'redirects':0,'timeout':5000,'seconds':12.5})

    def test_optional_event_control_requires_actual_composition_and_preserves_baseline(self):
        module = (Path(__file__).resolve().parents[1] / 'e2e/hls-renderer-timeline.mjs').as_uri()
        script = """import {checkpointWithoutComposition} from MODULE;
const proof = {rows:[],captureErrors:2};
const facts = [checkpointWithoutComposition(proof,false),checkpointWithoutComposition(proof,true),checkpointWithoutComposition(proof,1)];
proof.rows.push([0]); facts.push(checkpointWithoutComposition(proof,true),proof.captureErrors);
process.stdout.write(JSON.stringify(facts));""".replace('MODULE', json.dumps(module))
        run = subprocess.run(['node', '--input-type=module', '-e', script], capture_output=True, timeout=10)
        self.assertEqual(run.returncode, 0)
        self.assertEqual(json.loads(run.stdout), [False, True, False, False, 2])


class RendererProcessOwnership(unittest.TestCase):
    def test_timeout_terminates_and_joins_owned_leader_and_descendant(self):
        result = owned_command([sys.executable, '-c', CHILD + 'time.sleep(30)'], b'', 0.2)
        self.assertTrue(result['timedOut'])
        self.assertTrue(result['ownedGroupJoined'])
        self.assertEqual(result['liveOwnedProcesses'], 0)
        self.assertGreaterEqual(result['joinedSamples'], 2)

    def test_normal_leader_exit_still_joins_retained_descendant(self):
        result = owned_command([sys.executable, '-c', CHILD + 'print("complete")'], b'', 2)
        self.assertFalse(result['timedOut'])
        self.assertEqual(result['exitCode'], 0)
        self.assertEqual(result['stdout'], b'complete\n')
        self.assertTrue(result['ownedGroupJoined'])
        self.assertEqual(result['liveOwnedProcesses'], 0)

    def test_timeout_cleanup_preserves_unrelated_process(self):
        unrelated = subprocess.Popen([sys.executable, '-c', 'import time;time.sleep(30)'], start_new_session=True)
        try:
            result = owned_command([sys.executable, '-c', CHILD + 'time.sleep(30)'], b'', 0.2)
            self.assertTrue(result['ownedGroupJoined'])
            self.assertIsNone(unrelated.poll())
        finally:
            unrelated.terminate()
            unrelated.wait(timeout=2)

    def test_timeout_joins_verified_detached_browser_group(self):
        with tempfile.TemporaryDirectory() as directory:
            owner = Path(directory) / 'owner.json'
            script = "import json,pathlib,subprocess,sys,time; p=subprocess.Popen([sys.executable,'-c','import time;time.sleep(30)'],start_new_session=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);o=pathlib.Path(sys.argv[1]);t=o.with_suffix('.tmp');t.write_text(json.dumps({'browserPID':p.pid}));t.replace(o);time.sleep(30)"
            result = owned_command([sys.executable, '-c', script, str(owner)], b'', 0.4, owner)
            self.assertTrue(result['timedOut'])
            self.assertTrue(result['browserOwnershipVerified'])
            self.assertTrue(result['ownedGroupJoined'])
            self.assertEqual(result['liveOwnedProcesses'], 0)
            self.assertGreaterEqual(result['joinedSamples'], 2)

    def test_unrelated_pid_claim_is_rejected_without_signalling_it(self):
        unrelated = subprocess.Popen([sys.executable, '-c', 'import time;time.sleep(30)'], start_new_session=True)
        try:
            with tempfile.TemporaryDirectory() as directory:
                owner = Path(directory) / 'owner.json'
                script = "import json,pathlib,sys,time;o=pathlib.Path(sys.argv[1]);t=o.with_suffix('.tmp');t.write_text(json.dumps({'browserPID':int(sys.argv[2])}));t.replace(o);time.sleep(30)"
                with self.assertRaisesRegex(RuntimeError, 'renderer_browser_owner'):
                    owned_command([sys.executable, '-c', script, str(owner), str(unrelated.pid)], b'', 0.4, owner)
                self.assertIsNone(unrelated.poll())
        finally:
            unrelated.terminate()
            unrelated.wait(timeout=2)


if __name__ == '__main__':
    unittest.main()
