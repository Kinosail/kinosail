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

    def test_no_trust_exit_cleanup_is_success_and_keeps_original_failure(self):
        # A bare return inside a trap can inherit the original failure on newer Bash.
        result = self.call('trap \'status=$?; remove_browser_fixture_trust; '
                           'cleanup_status=$?; printf "%s\\n" "$cleanup_status"; '
                           'exit "$status"\' EXIT\nexit 7',
                           KINOSAIL_BROWSER_PROJECT='chromium')
        self.assertEqual(result.returncode, 7, result.stderr)
        self.assertEqual(result.stdout.strip(), '0')
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

    def test_non_ca_subject_cannot_impersonate_basic_constraints(self):
        subprocess.run(["openssl", "req", "-x509", "-newkey", "ec",
                        "-pkeyopt", "ec_paramgen_curve:P-256", "-nodes",
                        "-keyout", str(self.root / "leaf.key"), "-out", str(self.cert),
                        "-days", "1", "-subj", "/CN=CA:TRUE",
                        "-addext", "basicConstraints=critical,CA:FALSE"],
                       check=True, capture_output=True)
        result = self.call('trust_browser_fixture_tls docker abcdef123456 "$2" 123-456')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.effects.read_text().splitlines(), ["export"])

    def test_installer_validates_its_own_namespace_before_privileged_effects(self):
        for arguments in ('', '"$CERT"', '"$CERT" ""', '"$CERT" ../escape',
                          '"$CERT" unknown', '"$CERT" 123-456 extra',
                          '"$CERT" ' + "1" * 10000 + "-2"):
            with self.subTest(argument_length=len(arguments)):
                try:
                    result = self.call('install_browser_fixture_ca ' + arguments)
                    self.assertEqual(result.returncode, 2, result.stderr)
                    self.assertFalse(self.effects.exists())
                finally:
                    self.effects.unlink(missing_ok=True)

    def test_inherited_matching_trust_path_is_not_owned_or_deleted(self):
        result = self.call('remove_browser_fixture_trust\n'
                           'test \"$NODE_EXTRA_CA_CERTS\" = /tmp/another-run.crt',
                           BROWSER_FIXTURE_CA_PATH='/usr/local/share/ca-certificates/kinosail-browser-fixture-999-888.crt',
                           BROWSER_FIXTURE_NODE_CA_PATH='/tmp/another-run.crt',
                           BROWSER_FIXTURE_NODE_CA_PREVIOUS='/tmp/unrelated.crt',
                           BROWSER_FIXTURE_NODE_CA_WAS_SET='x',
                           NODE_EXTRA_CA_CERTS='/tmp/another-run.crt')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(self.effects.exists())

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
        source = (ROOT / "apps/player/scripts/test-browser-journeys.sh").read_text()
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
            self.assertIn("browser_fixture_uses_tls", transport)
            self.assertIn("ignoreHTTPSErrors: false",
                          (ROOT / f"apps/{app}/e2e/playwright.config.ts").read_text())




if __name__ == "__main__":
    unittest.main()
