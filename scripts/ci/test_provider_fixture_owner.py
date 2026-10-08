"""Provider caller and render admission with the existing fake Owner peers."""
import copy
import json
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch
import test_library_owner_caller as caller_peers
import test_library_fixture_owner as fixture_peers
from test_library_profile_admission import ROOT
from test_playback_profile_admission import playback_report


class ProviderFixtureTests(unittest.TestCase):
    def setUp(self):
        sys.path.insert(0, str(ROOT / 'apps/player/scripts'))
        self.addCleanup(lambda: sys.path.remove(str(ROOT / 'apps/player/scripts')))
        import provider_profile_cases
        self.provider = provider_profile_cases
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.ui = Path(self.temporary.name).resolve() / 'ui'
        self.ui.mkdir()
        for name in self.provider.UI_FILES:
            (self.ui / name).write_text('<svg></svg>' if 'certificate' in name else '<!doctype html><html></html>')

    def test_owned_render_files_bind_exact_nine_hashes_and_reject_before_effects(self):
        self.assertEqual(set(self.provider.ui_fixtures(self.ui)), set(self.provider.UI_FILES))
        leaf = self.ui / self.provider.UI_FILES[0]
        original = leaf.read_bytes()
        for kind in ('missing', 'empty', 'oversized', 'encoding', 'link', 'fifo', 'unknown'):
            with self.subTest(kind=kind):
                if kind == 'missing': leaf.unlink()
                elif kind == 'empty': leaf.write_bytes(b'')
                elif kind == 'oversized': leaf.write_bytes(b'x' * 2097153)
                elif kind == 'encoding': leaf.write_bytes(b'\xff')
                elif kind == 'link':
                    leaf.unlink(); leaf.symlink_to(self.ui / self.provider.UI_FILES[1])
                elif kind == 'fifo':
                    import os
                    leaf.unlink(); os.mkfifo(leaf)
                elif kind == 'unknown': (self.ui / 'foreign.html').write_text('foreign')
                with patch('subprocess.run', side_effect=AssertionError('process effect')):
                    with self.assertRaises((ValueError, OSError)): self.provider.ui_fixtures(self.ui)
                if (self.ui / 'foreign.html').exists(): (self.ui / 'foreign.html').unlink()
                if leaf.exists() or leaf.is_symlink(): leaf.unlink()
                leaf.write_bytes(original)
        with self.assertRaises(ValueError): self.provider.ui_fixtures(Path('relative'))

    def test_actual_owner_caller_runs_once_with_guard_ui_hashes_and_strict15_proof(self):
        peer = caller_peers.LibraryOwnerCallerTests()
        peer.setUp()
        try:
            cases = self.provider.selected_cases('webkit')
            peer.discovery.write_text(json.dumps(playback_report(cases, 'webkit', False)))
            peer.result = playback_report(cases, 'webkit', True)
            args = peer.arguments.copy(); args[args.index('--profile') + 1] = 'fake-provider'
            args += ['--ui-fixtures', str(self.ui)]
            with patch.object(caller_peers, 'load', return_value=SimpleNamespace(CASES=cases)):
                self.assertEqual(peer.execute([*args, '--admit-only']), 0)
                self.assertEqual(peer.effects, [])
                self.assertEqual(peer.execute(args), 0)
            self.assertEqual(sum(row[:3] == ('http', 'POST', '/api/v1/setup') for row in peer.effects), 1)
            browsers = [row for row in peer.effects if row[0] == 'process' and row[1][0] == 'pnpm']
            self.assertEqual(len(browsers), 1)
            env = browsers[0][2]['env']
            self.assertEqual(env['KINOSAIL_PROVIDER_PROFILE'], '1')
            self.assertEqual(env['KINOSAIL_BROWSER_TEST'], '1')
            self.assertEqual(env['KINOSAIL_UI_FIXTURE_DIR'], str(self.ui))
            receipt = json.loads((peer.output / 'setup-and-run.json').read_text())
            self.assertEqual(receipt['journeys']['count'], 15)
            self.assertEqual(set(receipt['uiFixtureSHA256']), set(self.provider.UI_FILES))
        finally: peer.doCleanups()

    def test_provider_https_all_engines_admits_then_initializes_one_owner(self):
        for project in ('chromium', 'firefox', 'webkit'):
            peer = caller_peers.LibraryOwnerCallerTests(); peer.setUp()
            try:
                cases = self.provider.selected_cases(project); peer.project = project
                peer.discovery.write_text(json.dumps(playback_report(cases, project, False)))
                peer.result = playback_report(cases, project, True)
                args = peer.arguments.copy(); args[args.index('--profile') + 1] = 'fake-provider'
                args[args.index('--project') + 1] = project
                args += ['--ui-fixtures', str(self.ui)]
                with patch.object(caller_peers, 'load', return_value=SimpleNamespace(CASES=cases)):
                    self.assertEqual(peer.execute([*args, '--admit-only']), 0)
                    self.assertEqual(peer.effects, [])
                    wrong = args.copy(); wrong[wrong.index('--url') + 1] = 'http://localhost:38127'
                    self.assertEqual(peer.execute([*wrong, '--admit-only']), 2)
                    self.assertEqual(peer.effects, [])
                    self.assertEqual(peer.execute(args), 0)
                self.assertEqual(sum(row[:3] == ('http','POST','/api/v1/setup') for row in peer.effects), 1)
                tls = next(row for row in peer.effects if row[0]=='process' and row[1][0]=='bash')
                self.assertEqual(tls[1][-1], 'fake-provider')
            finally: peer.doCleanups()

    def test_invalid_or_missing_render_inputs_and_inherited_overrides_do_not_initialize_owner(self):
        peer = caller_peers.LibraryOwnerCallerTests(); peer.setUp()
        try:
            cases = self.provider.selected_cases('webkit')
            peer.discovery.write_text(json.dumps(playback_report(cases, 'webkit', False)))
            args = peer.arguments.copy(); args[args.index('--profile') + 1] = 'fake-provider'
            for extra in ([], ['--ui-fixtures', str(self.ui / 'missing')], ['--ui-fixtures', 'relative'],
                          ['--ui-fixtures', str(self.ui), '--ui-fixtures', str(self.ui)]):
                self.assertEqual(peer.execute([*args, *extra]), 2)
            for environment in ({'KINOSAIL_PROVIDER_PROFILE':'1'}, {'KINOSAIL_UI_FIXTURE_DIR':str(self.ui)}, {'KINOSAIL_BROWSER_TEST':'0'}):
                self.assertEqual(peer.execute([*args,'--ui-fixtures',str(self.ui)], **environment), 2)
            (self.ui / self.provider.UI_FILES[0]).unlink()
            self.assertEqual(peer.execute([*args,'--ui-fixtures',str(self.ui)]), 2)
            self.assertEqual(peer.effects, [])
            self.assertFalse(peer.output.exists())
        finally: peer.doCleanups()

    def test_wrapper_renders_before_single_server_and_closes_its_owned_resources(self):
        peer = fixture_peers.LibraryFixtureOwnerTests(); peer.setUp()
        try:
            cases = self.provider.selected_cases('chromium')
            peer.discovery.write_text(json.dumps(playback_report(cases, 'chromium', False)))
            script = '#!' + sys.executable + '\n' + """import os,pathlib,json
path=pathlib.Path(os.environ['KINOSAIL_UI_FIXTURE_DIR'])
with pathlib.Path(os.environ['CONTROL_EVENTS']).open('a') as file:file.write(json.dumps(['render',[]])+'\\n')
for name in """ + repr(self.provider.UI_FILES) + """:(path/name).write_text('<svg></svg>' if 'certificate' in name else '<!doctype html><html></html>')
"""
            (peer.tools / 'go').write_text(script); (peer.tools / 'go').chmod(0o755)
            import subprocess, os
            cert = peer.root / 'public-ca.crt'
            subprocess.run(['openssl','req','-x509','-newkey','ec','-pkeyopt','ec_paramgen_curve:P-256','-nodes',
                            '-keyout',str(peer.root / 'private-key'),'-out',str(cert),'-days','1','-subj','/CN=Owned CA',
                            '-addext','basicConstraints=critical,CA:TRUE'],check=True,capture_output=True)
            for name, body in {'uname':'#!/bin/sh\nprintf Linux\n', 'sudo':'#!/bin/sh\nexit 0\n'}.items():
                (peer.tools / name).write_text(body); (peer.tools / name).chmod(0o755)
            peer.env.update(CI='true',GITHUB_ACTIONS='true',RUNNER_OS='Linux',CONTROL_CA=str(cert))
            docker = peer.tools / 'docker'; content = docker.read_text()
            content = content.replace("record('engine',args)", "if args and args[0]=='exec':print(pathlib.Path(os.environ['CONTROL_CA']).read_text(),end='');sys.exit(0)\nif args[:2]==['inspect','--format']:print('abcdef123456');sys.exit(0)\nrecord('engine',args)")
            docker.write_text(content)
            result = peer.run_caller(['chromium',str(peer.discovery),str(peer.output),'fake-provider'])
            self.assertEqual(result.returncode, 0, result.stderr)
            rows = peer.rows(); self.assertEqual(rows[0][0], 'admit')
            self.assertIn('https://localhost:38127', rows[0][1])
            run = next(argv for kind,argv in rows if kind=='engine' and argv[0]=='run' and '--detach' in argv)
            self.assertIn('KINOSAIL_TLS_ENABLED=true', run)
            self.assertFalse(any(arg.startswith('KINOSAIL_AUTH_URL') for arg in run))
            self.assertIn('https://localhost:49152', next(argv for kind,argv in rows if kind=='owner'))
            self.assertEqual(sum(kind == 'render' for kind, _ in rows), 1)
            owner = next(argv for kind, argv in rows if kind == 'owner')
            self.assertIn('--ui-fixtures', owner)
            self.assertLess(next(i for i,(kind,_) in enumerate(rows) if kind=='render'), next(i for i,(kind,args) in enumerate(rows) if kind=='engine' and args[0]=='run' and '--detach' in args))
            self.assertEqual(sum(kind == 'owner' for kind, _ in rows), 1)
            peer.assert_cleanup(rows)
        finally: peer.doCleanups()


if __name__ == '__main__': unittest.main()
