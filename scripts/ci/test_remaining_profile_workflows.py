"""Closed remaining-profile lanes admit before any disposable fixture effect."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import textwrap
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / '.github/workflows/layout-stability.yml'
PROFILES = (('Playback', 'playback-owner', 'playback', 'playback-start'),
            ('Offline', 'offline-owner', 'offline', 'offline-storage'),
            ('Provider', 'provider-owner', 'provider', 'fake-provider'))
DEFAULTS = ['false', 'false', 'false', 'false', 'primary', 'protocol', 'source-format', 'false']


class RemainingProfileWorkflowTests(unittest.TestCase):
    def setUp(self):
        # Inline recipes modify import paths; a fresh-root test must not leak its importer cache.
        before_path = sys.path.copy()
        before_cache = sys.path_importer_cache.copy()
        def restore():
            sys.path[:] = before_path
            sys.path_importer_cache.clear()
            sys.path_importer_cache.update(before_cache)
        self.addCleanup(restore)

    def block(self, job):
        return re.split(r'\n  [a-z][\w-]*:\n', WORKFLOW.read_text().split('  '+job+':\n', 1)[1])[0]

    def recipe(self, job):
        return textwrap.dedent(self.block(job).split("python3 - <<'PYTHON'\n", 1)[1].split('          PYTHON', 1)[0])

    def test_actual_cli_closed_selection_rejects_every_unused_override_before_effects(self):
        command = [sys.executable, str(ROOT/'scripts/ci/validate-layout-selection.py')]
        for campaign, _, _, _ in PROFILES:
            valid = ['workflow_dispatch', campaign, *DEFAULTS]
            accepted = subprocess.run([*command, *valid], capture_output=True, timeout=3)
            self.assertEqual(accepted.returncode, 0, accepted.stderr)
            bad = [['pull_request', *valid[1:]], valid[:-1], [*valid, 'extra']]
            for index, value in enumerate(('true','true','true','true','navigation','save-body','primary','true')):
                values=DEFAULTS.copy();values[index]=value;bad.append(['workflow_dispatch',campaign,*values])
            for index in range(10):
                values=valid.copy();values[index]='x'*33;bad.append(values)
            for values in bad:
                result=subprocess.run([*command,*values],capture_output=True,timeout=3)
                self.assertEqual(result.returncode,2)
                self.assertEqual(result.stdout,b'')

    def test_fixed_three_engine_jobs_preserve_other_lanes_and_strict_owner(self):
        source=WORKFLOW.read_text()
        campaign=source.split('  campaign-proof:\n',1)[1].split('  hls-followon:',1)[0]
        for title,job,prefix,profile in PROFILES:
            block=self.block(job)
            for required in ('needs: selection-admission', "inputs.campaign_proof == '"+title+"'",
                             'engine: [chromium, firefox, webkit]', 'fail-fast: false',
                             'install --frozen-lockfile', 'discovery '+profile, '--profile '+profile,
                             'timeout --kill-after=2s 60s', '2097153', '2097152',
                             'if: always()', 'if-no-files-found: error','fixture-startup.json'):
                self.assertIn(required,block)
            self.assertEqual(block.count('run: ./apps/player/scripts/run-library-profile.sh'),1)
            self.assertIn('"$RUNNER_TEMP/'+prefix+'-${PROFILE_PROJECT}" '+profile,block)
            self.assertLess(block.index('--admit-only'),block.index('Record immutable selection'))
            self.assertIn("inputs.campaign_proof != '"+title+"'",campaign)
            self.assertNotRegex(block,r'continue-on-error|--retries=[1-9]|test-instance.sh up')
        for job in ('library-owner','responsive-owner','camera-owner','hls-navigation','literal-verify','native'):
            self.assertIn('  '+job+':',source)
        self.assertIn('env -u KINOSAIL_PROVIDER_PROFILE python3 scripts/ci/run-populated-settings.py',self.block('provider-owner'))
        self.assertIn('KINOSAIL_BROWSER_TEST: "1"',self.block('provider-owner'))

    def test_provider_discovery_uses_the_same_required_https_transport_before_effects(self):
        block = self.block('provider-owner')
        discovery = block.split('- name: Discover and admit only the closed synthetic Provider15 selection',1)[1].split('      - name:',1)[0]
        self.assertIn('KINOSAIL_E2E_URL: https://localhost:38127', discovery)
        self.assertIn('--url "https://localhost:38127"', discovery)
        self.assertNotIn('scheme=', discovery)
        for job in ('playback-owner','offline-owner','camera-owner','library-owner','responsive-owner'):
            self.assertIn("scheme=http",self.block(job))

    def test_only_provider_installs_pinned_host_go_before_the_owned_ui_renderer(self):
        action = 'actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e'
        provider = self.block('provider-owner')
        self.assertEqual(provider.count(action), 1)
        step = provider.split('      - uses: '+action+'\n', 1)[1].split('\n      - ', 1)[0]
        self.assertIn('go-version-file: apps/player/go.mod', step)
        self.assertIn('cache: false', step)
        self.assertNotIn('if:', step)
        self.assertLess(provider.index(action), provider.index('run: ./apps/player/scripts/run-library-profile.sh'))
        for job in ('playback-owner', 'offline-owner'):
            self.assertNotIn('actions/setup-go@', self.block(job))
        caller = (ROOT/'apps/player/scripts/run-library-profile.sh').read_text()
        self.assertIn("go test ./internal/server -run '^TestWriteUIStateFixtures$' -count=1", caller)
        self.assertTrue((ROOT/'apps/player/go.mod').is_file())

    def test_actual_recipes_write_only_at_fresh_discovery_paths_with_exact_current_inputs(self):
        sys.path.insert(0,str(ROOT/'apps/player/scripts'))
        try:
            from library_profile_admission import profile_cases,playwright_arguments
            read=Path.read_bytes
            for _,job,prefix,profile in PROFILES:
                for project in ('chromium','firefox','webkit'):
                    with tempfile.TemporaryDirectory() as temporary:
                        original=Path.cwd();fresh=Path(temporary);artifact=fresh/('.verification/'+prefix+'-'+project)
                        artifact.mkdir(parents=True);(artifact/'discovery.json').write_text('{}')
                        try:
                            os.chdir(fresh)
                            with patch.dict(os.environ,{'PROFILE_PROJECT':project,'PROOF_REVISION':'fixture-revision'}), \
                                    patch.object(Path,'read_bytes',lambda path:read(ROOT/path)), \
                                    patch('subprocess.run',side_effect=AssertionError('fixture process effect')):
                                exec(compile(self.recipe(job),'profile-source-recipe','exec'),{})
                            value=json.loads((artifact/'source-receipt.json').read_text())
                            cases=profile_cases(profile,project)
                            self.assertEqual(value['identities'],[{'file':file,'fullTitle':title} for file,title in cases])
                            self.assertEqual(value['command'],playwright_arguments(project,False,profile))
                            self.assertEqual(value['profile'],profile)
                            for path,digest in value['sourceSHA256'].items():
                                self.assertEqual(digest,hashlib.sha256(read(ROOT/path)).hexdigest())
                            self.assertIn('scripts/ci/library-internal-relay.mjs',value['sourceSHA256'])
                            if profile == 'offline-storage':
                                self.assertIn('apps/player/e2e/offline-browser-api.mjs',value['sourceSHA256'])
                            self.assertEqual({p.relative_to(fresh).as_posix() for p in fresh.rglob('*') if p.is_file()},
                                             {'.verification/'+prefix+'-'+project+'/discovery.json',
                                              '.verification/'+prefix+'-'+project+'/source-receipt.json'})
                        finally:os.chdir(original)
            provider=self.block('provider-owner')
            for path in ('apps/player/e2e/provider-profile-fixture.ts','apps/player/internal/server/ui_state_fixtures_test.go'):
                self.assertIn("'"+path+"'",provider)
        finally:sys.path.remove(str(ROOT/'apps/player/scripts'))

    def test_missing_recipe_input_prevents_all_receipt_writes(self):
        read=Path.read_bytes
        def missing(path):
            if str(path)=='scripts/ci/library-internal-relay.mjs':raise FileNotFoundError('fixed missing source')
            return read(path)
        for _,job,_,_ in PROFILES:
            with patch.dict(os.environ,{'PROFILE_PROJECT':'webkit','PROOF_REVISION':'fixture-revision'}), \
                    patch.object(Path,'read_bytes',missing),patch.object(Path,'write_text') as write:
                with self.assertRaises(FileNotFoundError):exec(compile(self.recipe(job),'profile-source-recipe','exec'),{})
                write.assert_not_called()

if __name__=='__main__':unittest.main()
