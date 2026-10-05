"""Real shell contracts for the disposable WebKit fixture's HTTPS trust lifecycle."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / "scripts/ci/browser-fixture-tls.sh"


class BrowserFixtureTLS(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.tools = self.root / "bin"
        self.tools.mkdir()
        self.effects = self.root / "effects"
        self.cert = self.root / "fixture.crt"
        subprocess.run(["openssl", "req", "-x509", "-newkey", "ec",
                        "-pkeyopt", "ec_paramgen_curve:P-256", "-nodes",
                        "-keyout", str(self.root / "key"), "-out", str(self.cert),
                        "-days", "1", "-subj", "/CN=Disposable test CA",
                        "-addext", "basicConstraints=critical,CA:TRUE",
                        "-addext", "subjectAltName=DNS:localhost,IP:127.0.0.1"],
                       check=True, capture_output=True)
        for name, text in {
            "uname": '#!/bin/sh\nprintf "%s\\n" "$FAKE_OS"\n',
            "docker": '#!/bin/sh\nprintf "export\\n" >> "$EFFECTS"\ncat "$CERT"\n',
            "sudo": '#!/bin/sh\nprintf "%s\\n" "$1" >> "$EFFECTS"\n',
        }.items():
            path = self.tools / name
            path.write_text(text)
            path.chmod(0o755)
        self.env = os.environ | {
            "PATH": str(self.tools) + ":" + os.environ["PATH"],
            "EFFECTS": str(self.effects), "CERT": str(self.cert), "FAKE_OS": "Linux",
            "CI": "true", "GITHUB_ACTIONS": "true", "RUNNER_OS": "Linux",
            "KINOSAIL_BROWSER_TEST": "1", "KINOSAIL_BROWSER_PROJECT": "webkit",
        }

    def call(self, code, **env):
        return subprocess.run(["bash", "-c", 'set -euo pipefail\nsource "$1"\n' + code,
                               "fixture", str(HELPER), str(self.root)],
                              env=self.env | env, capture_output=True, text=True)

    def test_http_engines_do_not_install_trust(self):
        for engine in ("chromium", "firefox", ""):
            result = self.call('validate_browser_fixture_tls\n'
                               'if browser_fixture_uses_tls; then exit 4; fi',
                               KINOSAIL_BROWSER_PROJECT=engine,
                               CI="", GITHUB_ACTIONS="", RUNNER_OS="")
            self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(self.effects.exists())

    def test_webkit_requires_disposable_linux_runner_before_effects(self):
        for env in ({"CI": ""}, {"GITHUB_ACTIONS": ""}, {"RUNNER_OS": "macOS"},
                    {"FAKE_OS": "Darwin"}, {"CI": "unknown"}):
            result = self.call('validate_browser_fixture_tls', **env)
            self.assertEqual(result.returncode, 2, result.stderr)
            self.assertFalse(self.effects.exists())

    def test_exports_only_ca_and_cleans_up_its_own_trust(self):
        result = self.call('validate_browser_fixture_tls\n'
                           'trust_browser_fixture_tls docker abcdef123456 "$2" 123-456\n'
                           'remove_browser_fixture_trust\n'
                           'remove_browser_fixture_trust')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.effects.read_text().splitlines(),
                         ["export", "install", "update-ca-certificates",
                          "rm", "update-ca-certificates"])
        self.assertEqual((self.root / "browser-fixture-ca.crt").read_bytes(),
                         self.cert.read_bytes())

    def test_bad_trust_arguments_have_no_effects(self):
        cases = [
            'trust_browser_fixture_tls',
            'trust_browser_fixture_tls curl abcdef123456 "$2" 123-456',
            'trust_browser_fixture_tls docker --privileged "$2" 123-456',
            'trust_browser_fixture_tls docker abcdef123456 "$2" "../escape"',
            'trust_browser_fixture_tls docker abcdef123456 "$2/missing" 123-456',
            'trust_browser_fixture_tls docker abcdef123456 "$2" 123-456 extra',
            'trust_browser_fixture_tls docker abcdef123456 "$2" ' + "1" * 10000 + "-2",
        ]
        for code in cases:
            result = self.call(code)
            self.assertEqual(result.returncode, 2, result.stderr)
            self.assertFalse(self.effects.exists())

    def test_private_malformed_and_oversized_exports_never_install(self):
        for body in ("not a certificate\n", self.cert.read_text() +
                     "-----BEGIN PRIVATE KEY-----\nfixture\n",
                     "x" * (256 * 1024 + 1), self.cert.read_text() + "unknown\n"):
            self.cert.write_text(body)
            result = self.call('trust_browser_fixture_tls docker abcdef123456 "$2" 123-456')
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(self.effects.read_text().splitlines(), ["export"])
            self.effects.unlink()

    def test_cleanup_refuses_unowned_paths_without_effects(self):
        for path in ("/etc/ssl/certs", "/tmp/fixture.crt", "../escape"):
            result = self.call('BROWSER_FIXTURE_CA_PATH="$INVALID"\nremove_browser_fixture_trust',
                               INVALID=path)
            self.assertEqual(result.returncode, 2, result.stderr)
            self.assertFalse(self.effects.exists())

    def test_header_gate_requires_hsts_for_tls_and_excludes_it_for_http(self):
        curl = self.tools / "curl"
        curl.write_text('#!/bin/sh\nprintf "%s\\n" "$HEADERS"\n')
        curl.chmod(0o755)
        base = "Content-Security-Policy: default-src 'self'\nX-Content-Type-Options: nosniff\n"
        for url, headers, expected in (
                ("http://localhost:1234", base, 0),
                ("https://localhost:1234", base, 1),
                ("https://localhost:1234", base + "Strict-Transport-Security: max-age=31536000\n", 0)):
            result = self.call('source "$(dirname "$1")/test-container-transport.sh"\n'
                               'assert_container_test_headers "$URL"', URL=url, HEADERS=headers)
            self.assertEqual(result.returncode, expected, result.stderr)

    def test_real_app_startup_keeps_readonly_and_readwrite_media_distinct(self):
        docker = self.tools / "docker"
        docker.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$EFFECTS"\n'
                          'if [ "$1" = port ]; then echo 127.0.0.1:39077; '
                          'else echo abcdef123456; fi\n')
        curl = self.tools / "curl"
        curl.write_text('#!/bin/sh\nprintf \'{"status":"ok"}\\n\'\n')
        docker.chmod(0o755)
        curl.chmod(0o755)
        for app in ("player", "subtitles"):
            owner = ROOT / f"apps/{app}/scripts/test-container.sh"
            if app == "player":
                owner = ROOT / "apps/player/scripts/test-browser-journeys.sh"
            source = owner.read_text()
            start = source.index("start_server() {")
            end = source.index("\n}\n", start) + 3
            function = source[start:end]
            for engine, scheme in (("chromium", "http"), ("webkit", "https")):
                result = self.call('source "$(dirname "$1")/test-container-transport.sh"\n' +
                    function + '\nengine=docker; run=(run); image=fixture; '
                    'config_volume=config; cache_volume=cache; backup_volume=backup; '
                    'media_dir="$2"; start_server 39077\n'
                    'test "$url" = "$EXPECTED"', KINOSAIL_BROWSER_PROJECT=engine,
                    EXPECTED=scheme + "://localhost:39077")
                self.assertEqual(result.returncode, 0, result.stderr)
                effects = self.effects.read_text()
                self.assertIn("KINOSAIL_AUTH_URL=" + scheme + "://localhost:39077", effects)
                self.assertEqual("KINOSAIL_TLS_ENABLED=false" in effects, scheme == "http")
                self.assertIn(str(self.root) + ":/media:" + ("ro" if app == "player" else "rw"), effects)
                self.effects.unlink()

    def test_fresh_player_config_replaces_only_its_own_ca_trust(self):
        second = self.root / "second.crt"
        subprocess.run(["openssl", "req", "-x509", "-newkey", "ec",
                        "-pkeyopt", "ec_paramgen_curve:P-256", "-nodes",
                        "-keyout", str(self.root / "second.key"), "-out", str(second),
                        "-days", "1", "-subj", "/CN=Second disposable CA",
                        "-addext", "basicConstraints=critical,CA:TRUE"],
                       check=True, capture_output=True)
        self.assertNotEqual(self.cert.read_bytes(), second.read_bytes())
        sudo = self.tools / "sudo"
        sudo.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$EFFECTS"\n')
        sudo.chmod(0o755)
        docker = self.tools / "docker"
        docker.write_text('#!/bin/sh\nif [ "$1" = exec ]; then '
                          'echo export >> "$EFFECTS"; cat "$CERT"; '
                          'else echo container-remove >> "$EFFECTS"; fi\n')
        docker.chmod(0o755)
        source = (ROOT / "apps/player/scripts/test-container.sh").read_text()
        start = source.index("start_fresh_server() {")
        function = source[start:source.index("\n}\n", start) + 3]
        result = self.call(
            function + '\nremove_state_volumes() { :; }\n'
            'create_state_volumes() { :; }\nstart_server() { container=abcdef123456; }\n'
            'engine=docker; container=abcdef123456; mcp_dir="$2"; suffix=123-456\n'
            'trust_browser_fixture_tls "$engine" "$container" "$mcp_dir" "$suffix"\n'
            'CERT="$SECOND"; export CERT\nstart_fresh_server 39077\n'
            'remove_browser_fixture_trust', SECOND=str(second))
        self.assertEqual(result.returncode, 0, result.stderr)
        lines = self.effects.read_text().splitlines()
        self.assertEqual(lines[0], "export")
        self.assertEqual(lines[3], "container-remove")
        self.assertIn("rm -f -- /usr/local/share/ca-certificates/kinosail-browser-fixture-123-456.crt", lines)
        self.assertEqual(sum(line == "export" for line in lines), 2)
        installs = [line for line in lines if line.startswith("install ")]
        self.assertEqual(len(installs), 2)
        self.assertNotEqual(installs[0].split()[-1], installs[1].split()[-1])
        self.assertEqual((self.root / "browser-fixture-ca.crt").read_bytes(), second.read_bytes())
        self.assertEqual(sum(line.startswith("rm -f -- ") for line in lines), 2)

    def test_apps_validate_before_allocating_and_clean_trust(self):
        for app in ("player", "subtitles"):
            script = (ROOT / f"apps/{app}/scripts/test-container.sh").read_text()
            self.assertLess(script.index("validate_browser_fixture_tls"),
                            script.index('media_dir="$(mktemp -d)"'))
            self.assertIn("remove_browser_fixture_trust", script)
            self.assertIn('trust_browser_fixture_tls "$engine" "$container" "$mcp_dir" "$suffix"', script)
            transport = script
            if app == "player":
                transport += (ROOT / "apps/player/scripts/test-browser-journeys.sh").read_text()
            self.assertIn("browser_fixture_uses_tls", transport)
            self.assertIn("ignoreHTTPSErrors: false",
                          (ROOT / f"apps/{app}/e2e/playwright.config.ts").read_text())


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
        self.assertIn('trust_native_browser_fixture_tls', source)
        self.assertIn('remove_browser_fixture_trust', source)
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


if __name__ == "__main__":
    unittest.main()
