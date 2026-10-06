"""Native-server TLS uses one live shell owner through readiness and cleanup."""
import os
import selectors
import subprocess
import unittest
from scripts.ci import test_browser_fixture_tls as fixture_tls

HELPER = fixture_tls.HELPER
ROOT = fixture_tls.ROOT


class NativeBrowserFixtureTLS(unittest.TestCase):
    setUp = fixture_tls.BrowserFixtureTLS.setUp
    call = fixture_tls.BrowserFixtureTLS.call

    def test_native_owner_holds_trust_until_caller_eof_or_termination(self):
        # A second cleanup shell cannot own the first shell installation. Keep
        # the installing shell alive while Python owns the fixture lifecycle.
        binary = self.root / "server"
        binary.write_text('#!/bin/sh\ncat "$CERT"\n')
        binary.chmod(0o755)
        for ending in ("eof", "terminate"):
            owner = subprocess.Popen(["bash", "-c", 'set -euo pipefail; source "$1"; hold_native_browser_fixture_tls "$2/server" "$2" 123-456',
                                      "fixture", str(HELPER), str(self.root)],
                                     env=self.env | {"BROWSER_FIXTURE_CA_PATH": "/usr/local/share/ca-certificates/kinosail-browser-fixture-999-888.crt"},
                                     stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            self.addCleanup(lambda process=owner: process.kill() if process.poll() is None else None)
            with selectors.DefaultSelector() as ready:
                ready.register(owner.stdout, selectors.EVENT_READ)
                self.assertTrue(ready.select(timeout=5), "trust owner never reported readiness")
            self.assertEqual(owner.stdout.readline(), "ready\n", owner.stderr.read() if owner.poll() is not None else "")
            self.assertIsNone(owner.poll())
            self.assertEqual(self.effects.read_text().splitlines(), ["install", "update-ca-certificates"])
            if ending == "terminate":
                owner.terminate()
            owner.stdin.close()
            self.assertIn(owner.wait(timeout=5), (0, 143))
            owner.stdout.close()
            owner.stderr.close()
            self.assertEqual(self.effects.read_text().splitlines(), ["install", "update-ca-certificates", "rm", "update-ca-certificates"])
            self.effects.unlink()

    def test_native_owner_failed_install_cleans_only_its_installation(self):
        binary = self.root / "server"
        binary.write_text('#!/bin/sh\ncat "$CERT"\n')
        binary.chmod(0o755)
        sudo = self.tools / "sudo"
        sudo.write_text('#!/bin/sh\necho "$1" >> "$EFFECTS"\n[ "$1" != install ] || exit 7\n')
        sudo.chmod(0o755)
        result = self.call('hold_native_browser_fixture_tls "$2/server" "$2" 123-456')
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("ready", result.stdout)
        self.assertEqual(self.effects.read_text().splitlines(), ["install", "rm", "update-ca-certificates"])

    def test_native_server_exports_public_ca_and_removes_owned_trust(self):
        binary = self.root / "server"
        binary.write_text('#!/bin/sh\n[ "$#" = 1 ] && [ "$1" = tls-certificate ] || exit 8\necho native-export >> "$EFFECTS"\ncat "$CERT"\n')
        binary.chmod(0o755)
        result = self.call('trust_native_browser_fixture_tls "$2/server" "$2" 123-456\n'
                           'remove_browser_fixture_trust')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.effects.read_text().splitlines(),
                         ["native-export", "install", "update-ca-certificates", "rm", "update-ca-certificates"])

    def test_native_invalid_arguments_and_exports_have_no_trust_effects(self):
        binary = self.root / "server"
        binary.write_text('#!/bin/sh\necho native-export >> "$EFFECTS"\ncat "$CERT"\n')
        binary.chmod(0o755)
        (self.root / "link").symlink_to(binary)
        for code in (
                'trust_native_browser_fixture_tls',
                'trust_native_browser_fixture_tls "$2/missing" "$2" 123-456',
                'trust_native_browser_fixture_tls "$2/link" "$2" 123-456',
                'trust_native_browser_fixture_tls "$2/server" "$2/missing" 123-456',
                'trust_native_browser_fixture_tls "$2/server" "$2" ../escape',
                'trust_native_browser_fixture_tls "$2/server" "$2" 123-456 extra',
                'trust_native_browser_fixture_tls "$2/server" "$2" ' + "1" * 10000 + "-2"):
            result = self.call(code)
            self.assertEqual(result.returncode, 2, result.stderr)
            self.assertFalse(self.effects.exists())
        for body in ("invalid", self.cert.read_text() + "PRIVATE KEY", "x" * 262145):
            self.cert.write_text(body)
            result = self.call('trust_native_browser_fixture_tls "$2/server" "$2" 123-456')
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(self.effects.read_text().splitlines(), ["native-export"])
            self.effects.unlink()

    def test_native_layout_validates_before_allocation_and_keeps_browser_tls_verification(self):
        source = (ROOT / "scripts/testing/test-layout-stability-local.py").read_text()
        self.assertLess(source.index('validate_browser_fixture_tls'),
                        source.index('run.mkdir(parents=True)'))
        self.assertIn('hold_native_browser_fixture_tls', source)
        self.assertIn('ssl.create_default_context', source)
        self.assertIn('ignoreHTTPSErrors: false',
                      (ROOT / "scripts/testing/layout-stability-local.mjs").read_text())


    def test_failed_native_install_reports_failure_and_cleans_owned_trust(self):
        binary = self.root / "server"
        binary.write_text('#!/bin/sh\ncat "$CERT"\n')
        binary.chmod(0o755)
        sudo = self.tools / "sudo"
        sudo.write_text('#!/bin/sh\necho "$1" >> "$EFFECTS"\n[ "$1" != install ] || exit 7\n')
        sudo.chmod(0o755)
        result = self.call('trust_native_browser_fixture_tls "$2/server" "$2" 123-456 || { remove_browser_fixture_trust; exit 7; }')
        self.assertEqual(result.returncode, 7, result.stderr)
        self.assertEqual(self.effects.read_text().splitlines(),
                         ["install", "rm", "update-ca-certificates"])

    def test_native_export_refuses_symlink_without_writing_target(self):
        binary = self.root / "server"
        binary.write_text('#!/bin/sh\necho native-export >> "$EFFECTS"\ncat "$CERT"\n')
        binary.chmod(0o755)
        target = self.root / "protected"
        target.write_text("keep")
        (self.root / "browser-fixture-ca.crt").symlink_to(target)
        result = self.call('trust_native_browser_fixture_tls "$2/server" "$2" 123-456')
        self.assertEqual(result.returncode, 2, result.stderr)
        self.assertEqual(target.read_text(), "keep")
        self.assertFalse(self.effects.exists())


if __name__ == '__main__':
    unittest.main()
