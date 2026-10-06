"""Real small process trees protect renderer timeout/descendant ownership gaps."""
import subprocess
import sys
from pathlib import Path
import tempfile
import unittest
from hls_nonkey_process import owned_command


CHILD = "import subprocess,sys,time; subprocess.Popen([sys.executable,'-c','import time;time.sleep(30)'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);"


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
