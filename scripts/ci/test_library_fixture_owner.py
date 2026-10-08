"""Run the fixture CLI with command peers, never Docker/media/browser runtime."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

from test_library_profile_admission import load, report

ROOT = Path(__file__).resolve().parents[2]
CALLER = ROOT / 'apps/player/scripts/run-library-profile.sh'


class LibraryFixtureOwnerTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.tools = self.root / 'bin'
        self.tools.mkdir()
        self.events = self.root / 'events.jsonl'
        self.discovery = self.root / 'discovery.json'
        self.discovery.write_text(json.dumps(report(load().CASES, 'chromium', False)))
        self.output = self.root / 'results'
        self.env = {key: value for key, value in os.environ.items() if not key.startswith('KINOSAIL_')}
        self.env.update(PATH=str(self.tools) + ':' + os.environ['PATH'], PYTHONDONTWRITEBYTECODE='1',
                        CONTROL_EVENTS=str(self.events), CONTROL_PYTHON=sys.executable,
                        CONTAINER_ENGINE='docker')
        program = '''import json,os,pathlib,subprocess,sys
args=sys.argv[1:]; name=pathlib.Path(sys.argv[0]).name
events=pathlib.Path(os.environ['CONTROL_EVENTS'])
def record(kind,argv):
 with events.open('a') as file: file.write(json.dumps([kind,argv])+'\\n')
if name=='python3':
 if args and args[0].endswith('run-populated-settings.py'):
  record('admit' if '--admit-only' in args else 'owner',args)
  if '--admit-only' not in args:
   output=pathlib.Path(args[args.index('--output')+1]);output.mkdir()
   (output/'setup-and-run.json').write_text('{}')
   sys.exit(int(os.environ.get('CONTROL_OWNER_EXIT','0')))
 os.execv(os.environ['CONTROL_PYTHON'],[os.environ['CONTROL_PYTHON'],*args])
if name=='node':
 import signal,time,hashlib
 if len(args)>1 and args[1]=='topology':
  record('topology',[])
  if os.environ.get('DOCKER_HOST','') not in ('','unix:///var/run/docker.sock') or os.environ.get('DOCKER_CONTEXT','') not in ('','default') or args[2]!='docker':sys.exit(2)
  print('local-rootful-docker');sys.exit(0)
 if len(args)>1 and args[1]=='owned':
  _,_,engine,kind,resource,token,expected=args
  identity=hashlib.sha256((kind+':'+resource).encode()).hexdigest()
  record('resource-proof',[kind,resource,expected,identity])
  if kind=='volume' and os.environ.get('CONTROL_VOLUME_COLLISION')=='1' or kind=='container' and os.environ.get('CONTROL_CONTAINER_COLLISION')=='1':sys.exit(2)
  failure=os.environ.get('CONTROL_PROOF_FAIL_KIND')
  if failure==kind:
   prior=sum(1 for line in events.read_text().splitlines() if json.loads(line)[0]=='resource-proof' and json.loads(line)[1][0]==kind)
   if os.environ.get('CONTROL_PROOF_ALWAYS')=='1' or prior==1:sys.exit(2)
  print(identity);sys.exit(0)
 record('relay',args)
 def stop(*_):
  record('relay-close',[]);sys.exit(0)
 signal.signal(signal.SIGTERM,stop)
 time.sleep(float(os.environ.get('CONTROL_RELAY_DELAY','0')))
 print(os.environ.get('CONTROL_RELAY','{"schemaVersion":1,"port":49152}'),flush=True)
 while True: time.sleep(1)
if name=='curl':
 record('health',args);print('{"status":"ok"}');sys.exit(0)
if name=='go':
 record('unexpected-go',args);sys.exit(99)
if name=='mktemp':
 workspace=subprocess.check_output(['/usr/bin/mktemp',*args],text=True).strip()
 record('workspace',[workspace]);print(workspace);sys.exit(0)
record('engine',args)
if args and args[0]=='build' and os.environ.get('CONTROL_BUILD_FAIL')=='1':
 sys.exit(7)
elif args[:2]==['network','create']:
 print(args[-1])
elif args[:2]==['volume','create']:
 print(args[-1])
elif args[:2]==['image','inspect']:
 print(os.environ.get('CONTROL_IMAGE','sha256:'+'a'*64))
elif args and args[0]=='port':
 print(os.environ.get('CONTROL_PORT','127.0.0.1:38127'))
elif args and args[0]=='run' and '--detach' in args:
 sys.exit(7) if os.environ.get('CONTROL_START_FAIL')=='1' or os.environ.get('CONTROL_CONTAINER_COLLISION')=='1' else print('abcdef123456')
elif args and args[0]=='rm' and os.environ.get('CONTROL_CONTAINER_COLLISION')=='1':
 record('foreign-removed',['container'])
elif args[:2]==['volume','rm'] and os.environ.get('CONTROL_VOLUME_COLLISION')=='1':
 record('foreign-removed',['volume'])
elif args and args[0]=='run' and '--entrypoint' in args and '/bin/sh' in args:
 destination=args[args.index('--volume')+1].split(':')[0]
 for name in ('Example Photo One.jpg','Example Photo Two.jpg'):
  (pathlib.Path(destination)/'Photos/Geometry'/name).write_bytes(b'controlled image peer')
elif args and args[0]=='run' and ('ffmpeg' in args or 'ffprobe' in args):
 print('controlled codec peer; no actual codec')
'''
        for name in ('python3', 'docker', 'podman', 'curl', 'go', 'mktemp', 'node'):
            path = self.tools / name
            path.write_text('#!' + sys.executable + '\n' + program)
            path.chmod(0o755)

    def run_caller(self, arguments=None, **environment):
        return subprocess.run(['bash', str(CALLER), *(arguments or [
            'chromium', str(self.discovery), str(self.output)])], cwd=ROOT,
            env=self.env | environment, capture_output=True, text=True, timeout=20)

    def rows(self):
        return [json.loads(line) for line in self.events.read_text().splitlines()] if self.events.exists() else []

    def assert_cleanup(self, rows):
        engines = [args for kind, args in rows if kind == 'engine']
        volumes = [args[-1] for args in engines if args[:2] == ['volume', 'create']]
        self.assertEqual(len(volumes), 3)
        self.assertEqual({args[-1] for args in engines if args[:2] == ['volume', 'rm']}, set(volumes))
        network = next(args[-1] for args in engines if args[:2] == ['network', 'create'])
        network_id = next(argv[3] for kind, argv in rows if kind == 'resource-proof' and argv[:2] == ['network', network])
        self.assertIn(['network', 'rm', network_id], engines)
        run = next(args for args in engines if args[0] == 'run' and '--detach' in args)
        container_name = run[run.index('--name') + 1]
        container_id = next(argv[3] for kind, argv in rows if kind == 'resource-proof' and argv[:2] == ['container', container_name])
        self.assertIn(['rm', '--force', container_id], engines)
        media = next(args[args.index('--volume') + 1].split(':')[0] for args in engines
                     if args[0] == 'run' and '--entrypoint' in args and '/bin/sh' in args)
        self.assertFalse(Path(media).parent.exists())
        if any(kind == 'relay' for kind, _ in rows):
            self.assertEqual(sum(kind == 'relay-close' for kind, _ in rows), 1)

    def test_invalid_selection_or_profile_environment_has_no_fixture_effects(self):
        for arguments in ([], ['chromium'], ['all', str(self.discovery), str(self.output)],
                          ['chromium', str(self.discovery), str(self.output), 'extra']):
            # Empty argv is intentionally passed directly, not the default valid tuple.
            result = subprocess.run(['bash', str(CALLER), *arguments], cwd=ROOT,
                                    env=self.env, capture_output=True, timeout=20)
            self.assertEqual(result.returncode, 2)
        for environment in ({'CONTAINER_ENGINE': 'arbitrary-engine'},
                            {'KINOSAIL_TEST_IMAGE_READY': 'unknown'},
                            {'KINOSAIL_AUTH_URL': 'https://outside.invalid'},
                            {'KINOSAIL_BROWSER_PROJECT': 'firefox'}):
            self.assertEqual(self.run_caller(**environment).returncode, 2)
        self.assertEqual(self.rows(), [])
        self.assertFalse(self.output.exists())

    def test_bad_discovery_rejects_before_image_network_server_or_owner(self):
        self.discovery.write_text('{}')
        result = self.run_caller()
        self.assertEqual(result.returncode, 2, result.stderr)
        self.assertTrue(all(kind == 'admit' for kind, _ in self.rows()))
        self.assertFalse(self.output.exists())

    def test_one_selected_internal_server_delegates_one_owner_and_cleans_resources(self):
        result = self.run_caller()
        self.assertEqual(result.returncode, 0, result.stderr)
        rows = self.rows()
        self.assertEqual(rows[0][0], 'admit')
        self.assertEqual(sum(kind == 'topology' for kind, _ in rows), 1)
        self.assertEqual(sum(kind == 'owner' for kind, _ in rows), 1)
        engines = [args for kind, args in rows if kind == 'engine']
        self.assertEqual(sum(args[0] == 'run' and '--detach' in args for args in engines), 1)
        self.assertIn('--internal', next(args for args in engines if args[:2] == ['network', 'create']))
        owner = next(args for kind, args in rows if kind == 'owner')
        self.assertIn('--profile', owner)
        self.assertEqual(owner[owner.index('--state') + 1], 'fresh')
        self.assertFalse(any(kind == 'unexpected-go' for kind, _ in rows))
        self.assertEqual((self.output / 'fixture-image.txt').read_text().strip(), 'sha256:' + 'a' * 64)
        self.assert_cleanup(rows)

    def test_partial_start_and_owner_failure_still_close_only_owned_resources(self):
        for environment, expected in (({'CONTROL_START_FAIL': '1'}, 7), ({'CONTROL_OWNER_EXIT': '9'}, 9)):
            with self.subTest(environment=environment):
                result = self.run_caller(**environment)
                self.assertEqual(result.returncode, expected, result.stderr)
                rows = self.rows()
                self.assert_cleanup(rows)
                self.assertEqual(sum(kind == 'owner' for kind, _ in rows), int(expected == 9))
                self.events.unlink()
                if self.output.exists():
                    shutil.rmtree(self.output)

    def test_build_failure_cleans_workspace_before_resources_exist(self):
        result = self.run_caller(CONTROL_BUILD_FAIL='1')
        rows = self.rows()
        workspace = Path(next(args[0] for kind, args in rows if kind == 'workspace')).resolve()
        self.addCleanup(lambda: shutil.rmtree(workspace) if workspace.exists() else None)
        self.assertEqual(result.returncode, 7, result.stderr)
        self.assertFalse(workspace.exists())
        self.assertFalse(any(kind == 'owner' for kind, _ in rows))
        self.assertFalse(any(kind == 'engine' and args[0] in ('network', 'volume', 'run')
                             for kind, args in rows))

    def test_absent_internal_publish_uses_owned_relay_without_external_network(self):
        result = self.run_caller(CONTROL_PORT='')
        self.assertEqual(result.returncode, 0, result.stderr)
        rows = self.rows()
        self.assertEqual(sum(kind == 'owner' for kind, _ in rows), 1)
        self.assertEqual(sum(kind == 'relay' for kind, _ in rows), 1)
        self.assertFalse(any(kind == 'engine' and args[0] == 'port' for kind, args in rows))
        self.assert_cleanup(rows)

    def test_bad_relay_ready_rejects_before_health_or_owner_and_closes_fixture(self):
        for address in ('{"schemaVersion":1,"port":0}', '{"schemaVersion":1,"port":65536}',
                        '{"schemaVersion":1,"port":"49152"}', '{}',
                        '{"schemaVersion":1,"port":49152,"private":true}'):
            with self.subTest(address=address):
                result = self.run_caller(CONTROL_RELAY=address)
                rows = self.rows()
                try:
                    self.assertEqual(result.returncode, 2, result.stderr)
                    failure = json.loads((self.output / 'fixture-startup.json').read_text())
                    self.assertEqual(failure, {'schemaVersion': 1, 'phase': 'relay', 'exitCode': 2,
                                              'containerRunning': None, 'networkInternal': None,
                                              'targetAdmitted': False})
                    self.assertFalse(any(kind in ('health', 'owner') for kind, _ in rows))
                    self.assert_cleanup(rows)
                finally:
                    self.events.unlink()
                    if self.output.exists():
                        shutil.rmtree(self.output)

    def test_fixed_canonical_passkey_origin_preserves_selected_transport_and_port(self):
        for project in ('chromium', 'firefox'):
            with self.subTest(project=project):
                self.discovery.write_text(json.dumps(report(load().CASES, project, False)))
                result = self.run_caller([project, str(self.discovery), str(self.output)],
                                         CONTROL_PORT='127.0.0.1:49152')
                rows = self.rows()
                try:
                    self.assertEqual(result.returncode, 0, result.stderr)
                    server = next(args for kind, args in rows
                                  if kind == 'engine' and '--detach' in args)
                    self.assertIn('KINOSAIL_AUTH_URL=https://localhost:38127', server)
                    self.assertIn('KINOSAIL_TLS_ENABLED=false', server)
                    owner = next(args for kind, args in rows if kind == 'owner')
                    self.assertEqual(owner[owner.index('--url') + 1], 'http://localhost:49152')
                    self.assertEqual(sum(kind == 'owner' for kind, _ in rows), 1)
                    self.assert_cleanup(rows)
                finally:
                    self.events.unlink()
                    if self.output.exists():
                        shutil.rmtree(self.output)

    def test_valid_near_budget_relay_startup_reaches_owner(self):
        result = self.run_caller(CONTROL_RELAY_DELAY='13')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(sum(kind == 'owner' for kind, _ in self.rows()), 1)
        self.assert_cleanup(self.rows())

    def test_expired_relay_startup_has_no_owner_and_joins_cleanup(self):
        import time
        start = time.monotonic()
        result = self.run_caller(CONTROL_RELAY_DELAY='16')
        self.assertEqual(result.returncode, 2, result.stderr)
        self.assertLess(time.monotonic() - start, 20)
        self.assertFalse(any(kind == 'owner' for kind, _ in self.rows()))
        self.assert_cleanup(self.rows())

    def test_unsupported_or_remote_daemon_has_no_fixture_effects(self):
        for environment in ({'DOCKER_HOST': 'tcp://outside.invalid:2375'},
                            {'DOCKER_CONTEXT': 'remote-fixture'}, {'CONTAINER_ENGINE': 'podman'}):
            with self.subTest(environment=environment):
                result = self.run_caller(**environment)
                try:
                    self.assertEqual(result.returncode, 2)
                    self.assertFalse(any(kind in ('owner', 'health', 'workspace', 'engine', 'relay')
                                         for kind, _ in self.rows()))
                finally:
                    if self.events.exists(): self.events.unlink()
                    if self.output.exists(): shutil.rmtree(self.output)

    def test_foreign_container_collision_is_never_removed(self):
        result = self.run_caller(CONTROL_CONTAINER_COLLISION='1')
        workspace = Path(next(args[0] for kind, args in self.rows() if kind == 'workspace'))
        self.addCleanup(lambda: shutil.rmtree(workspace) if workspace.exists() else None)
        self.assertEqual(result.returncode, 7, result.stderr)
        self.assertFalse(any(kind == 'foreign-removed' for kind, _ in self.rows()))
        self.assertFalse(any(kind == 'owner' for kind, _ in self.rows()))

    def test_foreign_existing_volume_is_not_adopted_or_removed(self):
        result = self.run_caller(CONTROL_VOLUME_COLLISION='1')
        workspace = Path(next(args[0] for kind, args in self.rows() if kind == 'workspace'))
        self.addCleanup(lambda: shutil.rmtree(workspace) if workspace.exists() else None)
        self.assertFalse(any(kind == 'foreign-removed' for kind, _ in self.rows()))
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(kind == 'owner' for kind, _ in self.rows()))

    def test_unknown_image_identity_rejects_before_server_or_owner(self):
        result = self.run_caller(CONTROL_IMAGE='unknown-image')
        rows = self.rows()
        self.assertEqual(result.returncode, 2, result.stderr)
        self.assertFalse(any(kind == 'owner' or kind == 'health' for kind, _ in rows))
        self.assertFalse(any(kind == 'engine' and '--detach' in args for kind, args in rows))
        workspace = Path(next(args[0] for kind, args in rows if kind == 'workspace')).resolve()
        self.assertFalse(workspace.exists())


if __name__ == '__main__':
    unittest.main()
