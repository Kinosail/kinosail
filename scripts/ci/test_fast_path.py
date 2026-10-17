"""The fast path keeps one cached image per job, trusts only the merge queue for main, and keeps watcher names."""
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / '.github/workflows'


class FastPathContracts(unittest.TestCase):
    def test_each_e2e_job_builds_one_cached_amd64_image_without_artifact_transfer(self):
        ci = (WORKFLOWS / 'ci.yml').read_text()
        publish = (WORKFLOWS / 'publish.yml').read_text()
        e2e = ci.split('\n  e2e:\n', 1)[1].split('\n  proofs:\n', 1)[0]
        arm = ci.split('\n  arm:\n', 1)[1].split('\n  tools:\n', 1)[0]
        self.assertEqual(ci.count('uses: docker/build-push-action@'), 2)
        self.assertEqual(e2e.count('uses: docker/build-push-action@'), 1)
        self.assertIn('platforms: linux/amd64', e2e)
        self.assertIn('load: true', e2e)
        self.assertIn('push: false', e2e)
        self.assertIn('cache-to: type=gha,mode=max,scope=${{ matrix.app }}-linux/amd64', e2e)
        self.assertIn('platforms: linux/arm64', arm)
        self.assertIn('cache-to: type=gha,mode=max,scope=${{ matrix.app }}-linux/arm64', arm)
        self.assertNotIn('download-artifact', ci)
        self.assertNotIn('image.tar', ci)
        self.assertIn('cache-from: type=gha,scope=${{ matrix.app }}-linux/${{ matrix.arch }}', publish)
        versions = set(re.findall(r'^\s+(VERSION=.+)$', ci + publish, re.M))
        self.assertEqual(versions, {'VERSION=sha-${{ github.sha }}'})

    def test_main_push_already_verified_by_the_merge_queue_only_publishes(self):
        ci = (WORKFLOWS / 'ci.yml').read_text()
        plan = ci.split('\n  plan:\n', 1)[1].split('\n  repository:\n', 1)[0]
        self.assertIn('      actions: read\n', plan)
        self.assertIn('GH_TOKEN: ${{ github.token }}', plan)
        self.assertIn('verified: ${{ steps.select.outputs.verified }}', plan)
        for job, nxt in (('repository', 'go'), ('go', 'e2e'), ('e2e', 'proofs'), ('proofs', 'arm'), ('arm', 'tools'),
                         ('tools', 'client'), ('client', 'android'), ('android', 'docs')):
            section = ci.split(f'\n  {job}:\n', 1)[1].split(f'\n  {nxt}:\n', 1)[0]
            condition = section.split('    if: ', 1)[1].split('\n', 1)[0]
            self.assertTrue(condition.startswith("needs.plan.outputs.verified != 'true'"), (job, condition))
        for job, nxt in (('docs', 'security'), ('security', 'codeql-compiled'), ('codeql-compiled', 'repository-required')):
            section = ci.split(f'\n  {job}:\n', 1)[1].split(f'\n  {nxt}:\n', 1)[0]
            self.assertIn("needs.plan.outputs.verified != 'true' || github.event_name == 'push'", section)
        publish = (WORKFLOWS / 'publish.yml').read_text()
        verified = publish.split('\n  verified:\n', 1)[1].split('\n  images:\n', 1)[0]
        self.assertIn('      actions: read\n', verified)
        self.assertIn('GH_TOKEN: ${{ github.token }}', verified)
        self.assertIn('run: python3 scripts/ci/delivery.py', verified)
        self.assertNotIn('  schedule:', ci)
        self.assertIn('  schedule:', (WORKFLOWS / 'deep.yml').read_text())

    def test_layout_evidence_is_opt_in_for_pull_requests(self):
        source = (WORKFLOWS / 'layout-stability.yml').read_text()
        self.assertIn('types: [opened, synchronize, reopened, labeled]', source)
        self.assertIn('  schedule:\n', source)
        native = source.split('\n  native:\n', 1)[1].split('\n  populated-instance:\n', 1)[0]
        self.assertIn("contains(github.event.pull_request.labels.*.name, 'layout-evidence')", native)
        self.assertIn("github.event_name == 'schedule'", native)

    def test_publication_keeps_the_deployment_watcher_job_name(self):
        publish = (WORKFLOWS / 'publish.yml').read_text()
        production = publish.split('\n  production:\n', 1)[1]
        self.assertIn('name: Advance ${{ matrix.app }} production tags', production)
        self.assertIn('needs: [verified, images]', production)
        self.assertLess(production.index('cosign sign --yes'), production.index('name: Publish deployable commit tag'))
        self.assertLess(production.index('name: Publish deployable commit tag'), production.index('python3 scripts/ci/promote.py'))
        for script in ('scripts/tooling/deploy-nox-app.sh', 'scripts/tooling/watch-nox-main.sh'):
            self.assertIn('Publish verified containers / Advance $app production tags', (ROOT / script).read_text())

    def test_go_tests_run_alone_while_lint_and_vulnerability_checks_share_the_policy_runner(self):
        ci = (WORKFLOWS / 'ci.yml').read_text()
        go = ci.split('\n  go:\n', 1)[1].split('\n  e2e:\n', 1)[0]
        self.assertNotIn('parallel:', go)
        self.assertNotIn('golangci-lint-action', go)
        self.assertNotIn('govulncheck', go)
        repository = ci.split('\n  repository:\n', 1)[1].split('\n  go:\n', 1)[0]
        self.assertEqual(repository.count('uses: golangci/golangci-lint-action@'), 3)
        for directory in ('apps/player', 'apps/subtitles', 'packages'):
            self.assertIn(f'working-directory: {directory}\n', repository)
        self.assertIn('golang.org/x/vuln/cmd/govulncheck@v1.8.0', repository)
        self.assertIn("jq -r '.[].directory'", repository)

if __name__ == '__main__':
    unittest.main()
