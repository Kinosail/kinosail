"""Closed camera profile exercises real selection and Owner admission boundaries."""
import copy
import json
import os
from pathlib import Path
import subprocess
import sys
from types import SimpleNamespace
import unittest
from unittest.mock import patch

from test_library_profile_admission import load, report
import test_library_owner_caller as owner_peers

ROOT = Path(__file__).resolve().parents[2]


class CameraAdmissionTests(unittest.TestCase):
    def setUp(self):
        self.module = load()

    def cases(self):
        return self.module.CAMERA_CASES

    def admit(self, value, project='webkit', completed=True):
        return self.module.admit(json.dumps(value).encode(), 'camera-fake', project, 'fresh', completed)

    def test_closed19_discovery_and_first_pass_for_all_engines(self):
        self.assertEqual(len(set(self.cases())), 19)
        self.assertEqual({file for file, _ in self.cases()}, {'quick-connect-scan.spec.ts'})
        for project in self.module.PROJECTS:
            for completed in (False, True):
                self.assertEqual(len(self.admit(report(self.cases(), project, completed), project, completed)), 19)
            argv = self.module.playwright_arguments(project, False, 'camera-fake')
            self.assertEqual(argv[:4], ['exec', 'playwright', 'test', 'quick-connect-scan.spec.ts'])
            for option in ('--workers=1', '--retries=0', '--repeat-each=1'):
                self.assertIn(option, argv)
            result = subprocess.run([sys.executable, str(ROOT / 'apps/player/scripts/library_profile_admission.py'),
                                     project, 'execution', 'camera-fake'], capture_output=True, timeout=3)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.decode().split('\0')[:-1], argv)

    def test_bad_identities_and_attempts_never_certify(self):
        changes = [lambda v: v['suites'].pop(),
                   lambda v: v['suites'].append(copy.deepcopy(v['suites'][0])),
                   lambda v: v['suites'][0]['specs'][0].update(title='unknown context > '+v['suites'][0]['specs'][0]['title']),
                   lambda v: v['suites'][0]['specs'][0].update(file='other.spec.ts'),
                   lambda v: v['suites'][0]['specs'][0]['tests'][0].update(projectName='chromium'),
                   lambda v: v['stats'].update(skipped=1),
                   lambda v: v['stats'].update(expected=18),
                   lambda v: v['config']['projects'][0].update(retries=1),
                   lambda v: v['suites'][0]['specs'][0]['tests'][0]['results'][0].update(retry=1),
                   lambda v: v['suites'][0]['specs'][0]['tests'][0]['results'][0].update(status='failed')]
        for change in changes:
            value=report(self.cases());change(value)
            with self.assertRaises(ValueError):self.admit(value)
        for raw in (b'', b'{}', b'{', b'x'*2097153, b'{"errors":[],"\\u0065rrors":[]}', b'{"n":1e400}', b'{"n":-1e400}'):
            with self.assertRaises(ValueError):self.module.admit(raw,'camera-fake','webkit','fresh',True)
        with self.assertRaises(ValueError):self.admit(report(self.module.CASES))

    def peer(self):
        peer=owner_peers.LibraryOwnerCallerTests()
        peer.setUp()
        self.addCleanup(peer.temporary.cleanup)
        peer.discovery.write_text(json.dumps(report(self.cases(),completed=False)))
        peer.result=report(self.cases())
        peer.arguments[peer.arguments.index('--profile')+1]='camera-fake'
        return peer

    def execute(self, peer, arguments=None, **environment):
        # Reuse the existing bounded synthetic HTTP/process peer, substituting its expected selector.
        with patch.object(owner_peers,'load',return_value=SimpleNamespace(CASES=self.cases())):
            return peer.execute(arguments,**environment)

    def test_actual_caller_admits_before_effects_then_prepares_one_owner(self):
        peer=self.peer()
        self.assertEqual(self.execute(peer,[*peer.arguments,'--admit-only']),0)
        self.assertEqual(peer.effects,[]);self.assertFalse(peer.output.exists())
        self.assertEqual(self.execute(peer),0)
        self.assertEqual([row for row in peer.effects if row[0]=='http'],[
            ('http','POST','/api/v1/setup'),('http','PUT','/api/v1/me/mfa'),('http','GET','/onboarding/finish')])
        receipt=json.loads((peer.output/'setup-and-run.json').read_text())
        self.assertEqual(receipt['journeys']['count'],19)
        self.assertEqual(receipt['profile'],'camera-fake')
        command=[row for row in peer.effects if row[0]=='process' and row[1][0]=='pnpm']
        self.assertEqual(len(command),1)
        self.assertEqual(command[0][2]['env']['KINOSAIL_CAMERA_PROFILE'],'1')
        self.assertNotIn('synthetic-token',(peer.output/'setup-and-run.json').read_text())

    def test_bad_selection_discovery_and_output_reject_before_effects(self):
        peer=self.peer()
        for flag,value in (('--profile','camera'),('--project','safari'),('--state','reused'),('--output','relative'),('--url','https://external.invalid:38127')):
            args=peer.arguments.copy();args[args.index(flag)+1]=value
            self.assertEqual(self.execute(peer,args),2)
        for flag in ('--profile','--project','--state','--discovery'):
            args=peer.arguments.copy();index=args.index(flag);del args[index:index+2]
            self.assertEqual(self.execute(peer,args),2)
        for extra in (['--profile','camera-fake'],['--','arbitrary'],['--unknown','x']):
            self.assertEqual(self.execute(peer,[*peer.arguments,*extra]),2)
        self.assertEqual(self.execute(peer,KINOSAIL_BROWSER_PROJECT='chromium'),2)
        peer.discovery.write_text(json.dumps(report(self.module.CASES,completed=False)))
        self.assertEqual(self.execute(peer),2)
        self.assertEqual(peer.effects,[]);self.assertFalse(peer.output.exists())


if __name__=='__main__':unittest.main()
