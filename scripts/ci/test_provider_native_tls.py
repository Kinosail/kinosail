"""Provider-only native trust joins the existing shell fixture lifecycle."""
import unittest
from scripts.ci import test_browser_fixture_tls as fixture


class ProviderNativeTLS(unittest.TestCase):
    setUp = fixture.BrowserFixtureTLS.setUp
    call = fixture.BrowserFixtureTLS.call

    def test_provider_installs_and_joins_native_trust_before_owner_effects(self):
        for project in ('chromium', 'firefox'):
            result = self.call('install_browser_native_ca() { printf "native:%s\\n" "$1" >> "$EFFECTS"; }\n'
                               'remove_browser_native_ca() { printf "native-cleanup\\n" >> "$EFFECTS"; }\n'
                               'trust_browser_fixture_tls docker abcdef123456 "$2" 123-456 fake-provider\n'
                               'remove_browser_fixture_trust', KINOSAIL_BROWSER_PROJECT=project)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn('native:' + project, self.effects.read_text().splitlines())
            self.assertIn('native-cleanup', self.effects.read_text().splitlines())
            self.effects.unlink()
            (self.root / 'browser-fixture-ca.crt').unlink()

    def test_partial_native_install_failure_is_owned_before_cleanup(self):
        result=self.call('install_browser_native_ca() { printf "native-partial\\n" >> "$EFFECTS"; return 7; }\n'
                         'remove_browser_native_ca() { printf "native-cleanup\\n" >> "$EFFECTS"; }\n'
                         'if trust_browser_fixture_tls docker abcdef123456 "$2" 123-456 fake-provider; then exit 9; else failure=$?; fi\n'
                         'remove_browser_fixture_trust\nexit "$failure"', KINOSAIL_BROWSER_PROJECT='chromium')
        self.assertEqual(result.returncode,7,result.stderr)
        rows=self.effects.read_text().splitlines()
        self.assertIn('native-cleanup',rows)
        self.assertLess(rows.index('native-cleanup'),rows.index('rm'))

    def test_native_cleanup_failure_retains_authority_and_restores_node_trust(self):
        result=self.call('install_browser_native_ca() { :; }\n'
                         'remove_browser_native_ca() { printf "retained\\n" >> "$EFFECTS"; return 2; }\n'
                         'trust_browser_fixture_tls docker abcdef123456 "$2" 123-456 fake-provider\n'
                         'if remove_browser_fixture_trust; then exit 9; fi\n'
                         'test "$BROWSER_FIXTURE_NATIVE_PROJECT" = chromium\n'
                         'test "$NODE_EXTRA_CA_CERTS" = /tmp/prior-trust.crt\n'
                         'if remove_browser_fixture_trust; then exit 9; fi',
                         KINOSAIL_BROWSER_PROJECT='chromium',NODE_EXTRA_CA_CERTS='/tmp/prior-trust.crt')
        self.assertEqual(result.returncode,0,result.stderr)
        rows=self.effects.read_text().splitlines();self.assertEqual(rows.count('retained'),2)
        self.assertEqual(rows.count('rm'),1)

