"""Offline proof admits only a measured, named OPFS capability exception."""
import base64
import copy
import json
import sys
import unittest
from unittest.mock import patch
from test_library_profile_admission import ROOT, load
from test_playback_profile_admission import playback_report

OPFS_FILE = 'test-instance-large-offline-b.spec.ts'
OPFS_TITLE = 'large offline transfers › offline resume does not count an orphaned OPFS write twice against quota'
REASON = 'This engine lacks the OPFS sync writer; IndexedDB quota resume is covered separately.'


def offline_report(cases, project, completed, supported=True):
    value = playback_report(cases, project, completed)
    if not completed: return value
    suite = next(s for s in value['suites'] if s['title'] == OPFS_FILE)
    spec = next(s for s in suite['suites'][0]['specs'] if s['title'] == OPFS_TITLE.split(' › ')[1])
    test = spec['tests'][0]; result = test['results'][0]
    body = json.dumps({'schemaVersion': 1, 'writer': 'opfs-sync-worker', 'supported': supported}).encode()
    result['attachments'] = [{'name': 'offline-opfs-capability', 'contentType': 'application/json', 'body': base64.b64encode(body).decode()}]
    if not supported:
        test.update(expectedStatus='skipped', status='skipped', annotations=[{'type': 'skip', 'description': REASON}])
        result.update(status='skipped')
        value['stats'].update(expected=16, skipped=1)
    return value


class OfflineProfileTests(unittest.TestCase):
    def setUp(self):
        directory = str(ROOT/'apps/player/scripts');sys.path.insert(0,directory)
        self.addCleanup(lambda:sys.path.remove(directory));self.module=load()

    def cases(self, project): return self.module.profile_cases('offline-storage',project)
    def admit(self, value, project, completed=True):
        return self.module.admit(json.dumps(value).encode(),'offline-storage',project,'fresh',completed)

    def test_exact17_collection_and_supported17passes_are_not_capability_skips(self):
        for project in ('chromium','firefox','webkit'):
            cases=self.cases(project);self.assertEqual(len(cases),17);self.assertEqual(len(set(cases)),17)
            self.assertIn((OPFS_FILE,OPFS_TITLE),cases)
            with patch('subprocess.run',side_effect=AssertionError('process effect')):
                self.assertEqual(len(self.admit(offline_report(cases,project,False),project,False)),17)
                self.assertEqual(len(self.admit(offline_report(cases,project,True),project)),17)

    def test_only_measured_unsupported_writer_admits16passes_plus_one_named_skip(self):
        for project in ('chromium','firefox','webkit'):
            value=offline_report(self.cases(project),project,True,False)
            self.assertEqual(len(self.admit(value,project)),17)
            value['stats'].update(expected=17,skipped=0)
            with self.assertRaises(ValueError):self.admit(value,project)

    def test_missing_malformed_unknown_oversized_duplicate_capability_or_wrong_status_reject_before_effects(self):
        good=offline_report(self.cases('webkit'),'webkit',True,False)
        def result(value):return next(s for s in value['suites'] if s['title']==OPFS_FILE)['suites'][0]['specs'][0]['tests'][0]['results'][0]
        with patch('subprocess.run',side_effect=AssertionError('process effect')),patch('os.mkdir',side_effect=AssertionError('output effect')):
            for raw in (b'',b'{',b'{}',b'{"schemaVersion":1,"writer":"opfs-sync-worker","supported":true}',
                        b'{"schemaVersion":1,"writer":"opfs-sync-worker","supported":false,"supported":false}',
                        b'{"schemaVersion":1,"writer":"opfs-sync-worker","supported":"false"}',
                        b'{"schemaVersion":1,"writer":"unknown","supported":false}',b'x'*4097):
                value=copy.deepcopy(good);result(value)['attachments'][0]['body']=base64.b64encode(raw).decode()
                with self.assertRaises(ValueError):self.admit(value,'webkit')
            for change in (lambda r:r.pop('attachments'),lambda r:r['attachments'].append(copy.deepcopy(r['attachments'][0])),
                           lambda r:r['attachments'][0].update(path='/private/capability.json'),
                           lambda r:r['attachments'][0].update(body='not-base64'),lambda r:r.update(retry=1),
                           lambda r:r.update(status='passed')):
                value=copy.deepcopy(good);change(result(value))
                with self.assertRaises(ValueError):self.admit(value,'webkit')
            value=copy.deepcopy(good);value['suites'].pop()
            with self.assertRaises(ValueError):self.admit(value,'webkit')
            value=copy.deepcopy(good);value['suites'].append(copy.deepcopy(value['suites'][0]))
            with self.assertRaises(ValueError):self.admit(value,'webkit')

    def test_actual_wrapper_admits_offline_discovery_before_one_owner_and_owned_cleanup(self):
        import test_library_fixture_owner as fixture_peers
        peer = fixture_peers.LibraryFixtureOwnerTests()
        peer.setUp()
        try:
            peer.discovery.write_text(json.dumps(offline_report(self.cases('chromium'), 'chromium', False)))
            arguments = ['chromium', str(peer.discovery), str(peer.output), 'offline-storage']
            result = peer.run_caller(arguments)
            self.assertEqual(result.returncode, 0, result.stderr)
            rows = peer.rows()
            self.assertEqual(rows[0][0], 'admit')
            owners = [argv for event, argv in rows if event == 'owner']
            self.assertEqual(len(owners), 1)
            self.assertEqual(owners[0][owners[0].index('--profile') + 1], 'offline-storage')
            peer.assert_cleanup(rows)
            peer.events.unlink()
            peer.discovery.write_text('{}')
            rejected = peer.run_caller([*arguments[:2], str(peer.root / 'rejected-output'), arguments[3]])
            self.assertEqual(rejected.returncode, 2)
            self.assertTrue(all(event == 'admit' for event, _ in peer.rows()))
            self.assertFalse((peer.root / 'rejected-output').exists())
        finally:
            peer.doCleanups()

    def test_supported_probe_cannot_hide_a_skip_or_unregistered_capability_exception(self):
        value=offline_report(self.cases('firefox'),'firefox',True)
        test=next(s for s in value['suites'] if s['title']==OPFS_FILE)['suites'][0]['specs'][0]['tests'][0]
        test.update(expectedStatus='skipped',status='skipped');test['results'][0]['status']='skipped'
        value['stats'].update(expected=16,skipped=1)
        with self.assertRaises(ValueError):self.admit(value,'firefox')

if __name__=='__main__':unittest.main()
