"""Protect required CI evidence: a green browser exit must not mask skipped journeys.

The browser E2E suite cannot detect a helper that accepts an incomplete results
file. Load only the verifier so this isolated regression has no setup side effects.
"""
import ast
import hashlib
import http.server
import json
import os
from pathlib import Path
import ssl
import subprocess
import sys
import tempfile
import threading
import unittest


SOURCE = Path(__file__).with_name("run-populated-settings.py")
tree = ast.parse(SOURCE.read_text())
verifier = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == "verify_results")
namespace = {"json": json, "hashlib": hashlib}
exec(compile(ast.Module(body=[verifier], type_ignores=[]), str(SOURCE), "exec"), namespace)
verify_results = namespace["verify_results"]
DEFAULT_TITLES = ["settings search crosses levels and preserves unsaved preferences",
                  "Owner settings search finds a setting across task families"]


class PopulatedResultsTest(unittest.TestCase):
    def results(self, titles, status="expected", result="passed"):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        path = Path(temporary.name) / "results.json"
        specs = [{"title": title, "tests": [{"status": status, "results": [{"status": result}]}]} for title in titles]
        path.write_text(json.dumps({"suites": [{"suites": [{"specs": specs}]}]}))
        return path

    def test_defaults_still_require_both_settings_journeys(self):
        self.assertEqual(set(verify_results(self.results(DEFAULT_TITLES))["passed"]), set(DEFAULT_TITLES))
        with self.assertRaises(RuntimeError):
            verify_results(self.results(DEFAULT_TITLES[:1]))

    def test_explicit_progress_and_library_journeys_are_required(self):
        titles = ["real progress save", "complete show pagination"]
        self.assertEqual(set(verify_results(self.results(titles), titles)["passed"]), set(titles))
        with self.assertRaises(RuntimeError):
            verify_results(self.results(DEFAULT_TITLES), titles)

    def test_explicit_skip_failure_and_duplicate_results_are_rejected(self):
        titles = ["real progress save"]
        for status, result in [("skipped", "skipped"), ("unexpected", "failed"), ("expected", "failed")]:
            with self.subTest(status=status, result=result), self.assertRaises(RuntimeError):
                verify_results(self.results(titles, status, result), titles)
        with self.assertRaises(RuntimeError):
            verify_results(self.results(titles * 2), titles)


class PopulatedHTTPSFixture(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        subprocess.run(["git", "init", "-q", str(self.root)], check=True)
        subprocess.run(["git", "-C", str(self.root), "config", "user.name", "Disposable fixture"], check=True)
        subprocess.run(["git", "-C", str(self.root), "config", "user.email", "fixture@example.invalid"], check=True)
        subprocess.run(["git", "-C", str(self.root), "commit", "-q", "--allow-empty", "-m", "Fixture"], check=True)
        self.ca = self.root / "fixture.crt"
        key = self.root / "fixture.key"
        subprocess.run(["openssl", "req", "-x509", "-newkey", "ec",
                        "-pkeyopt", "ec_paramgen_curve:P-256", "-nodes",
                        "-keyout", str(key), "-out", str(self.ca), "-days", "1",
                        "-subj", "/CN=Disposable settings fixture",
                        "-addext", "basicConstraints=critical,CA:TRUE",
                        "-addext", "subjectAltName=DNS:localhost,IP:127.0.0.1"],
                       check=True, capture_output=True)
        self.requests = []
        requests = self.requests

        class Handler(http.server.BaseHTTPRequestHandler):
            def reply(self, code, body):
                requests.append((self.command, self.path))
                self.send_response(code)
                self.send_header("Content-Type", "application/json")
                self.send_header("Location", "/")
                self.end_headers()
                self.wfile.write(json.dumps(body).encode())
            def do_POST(self):
                self.rfile.read(int(self.headers.get("Content-Length", "0")))
                self.reply(201, {"token": "disposable-token",
                    "totp": {"secret": "JBSWY3DPEHPK3PXP"}})
            def do_PUT(self):
                self.rfile.read(int(self.headers.get("Content-Length", "0")))
                self.reply(200, {"enabled": True})
            def do_GET(self):
                self.reply(303, {})
            def log_message(self, *args):
                pass

        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.minimum_version = ssl.TLSVersion.TLSv1_2
        context.load_cert_chain(self.ca, key)
        self.server.socket = context.wrap_socket(self.server.socket, server_side=True)
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server.shutdown)
        tools = self.root / "bin"
        tools.mkdir()
        uname = tools / "uname"
        uname.write_text('#!/bin/sh\nprintf "Linux\\n"\n')
        uname.chmod(0o755)
        self.env = os.environ | {"PATH": str(tools) + ":" + os.environ["PATH"],
            "CI": "true", "GITHUB_ACTIONS": "true", "RUNNER_OS": "Linux",
            "KINOSAIL_BROWSER_TEST": "1", "KINOSAIL_BROWSER_PROJECT": "webkit",
            "NODE_EXTRA_CA_CERTS": str(self.ca), "PYTHONDONTWRITEBYTECODE": "1"}
        self.url = f"https://localhost:{self.server.server_port}"
        self.output = self.root / "results"
        self.marker = self.root / "browser-started"

    def run_helper(self, url=None, **env):
        browser = ("import json,os;from pathlib import Path;"
            "p=Path(os.environ['KINOSAIL_E2E_ARTIFACT_DIR']);"
            f"Path({str(self.marker)!r}).write_text('started');"
            "body={'suites':[{'specs':[{'title':'strict fixture journey',"
            "'tests':[{'status':'expected','results':[{'status':'passed'}]}]}]}]};"
            "(p/'results-webkit.json').write_text(json.dumps(body))")
        return subprocess.run([sys.executable, str(SOURCE), "--url", url or self.url,
            "--output", str(self.output), "--required-title", "strict fixture journey",
            "--", sys.executable, "-c", browser], cwd=self.root,
            env=self.env | env, text=True, capture_output=True, timeout=20)

    def test_strict_node_request_gets_validated_fixture_ca_and_restores_previous_env(self):
        tools = self.root / "bin"
        for name, content in {
            "docker": '#!/bin/sh\ncat "$PUBLIC_CA"\n',
            "sudo": '#!/bin/sh\nexit 0\n',
        }.items():
            path = tools / name
            path.write_text(content)
            path.chmod(0o755)
        helper = SOURCE.with_name("browser-fixture-tls.sh")
        script = "require('node:https').get(process.argv[1],r=>{console.log(r.statusCode);r.resume()}).on('error',()=>process.exit(3))"
        env = self.env | {"NODE_SCRIPT": script, "URL": self.url, "PUBLIC_CA": str(self.ca)}
        absent = subprocess.run(["node", "-e", script, self.url], env=env | {"NODE_EXTRA_CA_CERTS": ""},
                                capture_output=True, text=True)
        self.assertEqual(absent.returncode, 3)
        result = subprocess.run(["bash", "-c",
            'set -euo pipefail; source "$1"; '
            'trust_browser_fixture_tls docker abcdef123456 "$2" 123-456; '
            'test "$NODE_EXTRA_CA_CERTS" = "$2/browser-fixture-ca.crt"; '
            'node -e "$NODE_SCRIPT" "$URL"; '
            'remove_browser_fixture_trust; test "$NODE_EXTRA_CA_CERTS" = "$PUBLIC_CA"',
            "fixture", str(helper), str(self.root)], env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), "303")

    def test_strict_https_owner_preparation_and_browser_results(self):
        result = self.run_helper()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.requests, [("POST", "/api/v1/setup"),
            ("PUT", "/api/v1/me/mfa"), ("GET", "/onboarding/finish")])
        self.assertTrue(self.marker.exists())
        self.assertEqual(json.loads((self.output / "setup-and-run.json").read_text())["exitCode"], 0)

    def test_firefox_strict_browser_failure_precedes_owner_side_effects(self):
        import importlib.util
        import shutil
        from unittest.mock import patch
        self.root=self.root.resolve(); self.output=self.root/'results'
        project=self.root/'project'; helpers=project/'scripts/ci'; helpers.mkdir(parents=True)
        for name in ('run-populated-settings.py','browser-fixture-tls.sh','browser-native-ca.py','browser-firefox-policy.py'):
            shutil.copyfile(SOURCE.with_name(name),helpers/name)
        scripts=project/'apps/player/scripts'; scripts.mkdir(parents=True)
        for name in ('library_profile_admission.py','provider_profile_cases.py'):
            shutil.copyfile(SOURCE.parents[2]/'apps/player/scripts'/name,scripts/name)
        sys.path.insert(0,str(SOURCE.parent)); sys.path.insert(0,str(scripts))
        try:
            from test_playback_profile_admission import playback_report
            from provider_profile_cases import CASES, UI_FILES
            discovery=self.root/'discovery.json'; discovery.write_text(json.dumps(playback_report(CASES,'firefox',False)))
            ui=self.root/'ui'; ui.mkdir()
            for name in UI_FILES: (ui/name).write_text('<html>synthetic owner renderer peer</html>')
        finally:
            sys.path.remove(str(scripts)); sys.path.remove(str(SOURCE.parent))
        app=project/'apps/player/e2e'; dependency=app/'node_modules/@playwright/test'; dependency.mkdir(parents=True)
        shutil.copyfile(SOURCE.parents[2]/'apps/player/e2e/strict-firefox-probe.cjs',app/'strict-firefox-probe.cjs')
        home=self.root/'home'; executable=home/'.cache/ms-playwright/firefox-1543/firefox/firefox'
        executable.parent.mkdir(parents=True); executable.write_text('owned executable')
        certificate=self.root/'browser-fixture-ca.crt'; certificate.write_bytes(self.ca.read_bytes())
        (dependency/'package.json').write_text('{"main":"index.cjs"}')
        (dependency/'index.cjs').write_text("exports.firefox={executablePath:()=>process.env.EXECUTABLE,"
            "launch:async()=>({newContext:async()=>({newPage:async()=>({goto:async()=>{throw new Error('SEC_ERROR_UNKNOWN_ISSUER private-detail')}})}),"
            "close:async()=>require('fs').writeFileSync(process.env.CLOSED,'closed')})};")
        spec=importlib.util.spec_from_file_location('installed_firefox',helpers/'browser-native-ca.py')
        module=importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
        env=self.env|{'KINOSAIL_BROWSER_PROJECT':'firefox','EXECUTABLE':str(executable)}
        with patch.object(module.sys,'platform','linux'),patch.object(Path,'home',return_value=home),patch.dict(os.environ,env):
            module.install('firefox',certificate,self.root,'123-456')
        wrapper="import sys,runpy;from pathlib import Path;sys.platform='linux';home=sys.argv.pop(1);Path.home=classmethod(lambda cls:Path(home));sys.argv.pop(0);runpy.run_path(sys.argv[0],run_name='__main__')"
        closed=self.root/'closed'
        env.update(NODE_EXTRA_CA_CERTS=str(certificate),PLAYWRIGHT_FIREFOX_POLICIES_JSON=str(executable.parent/'distribution/policies.json'),
            KINOSAIL_FIREFOX_TRUST_NONCE='123-456',CLOSED=str(closed))
        result=subprocess.run([sys.executable,'-c',wrapper,str(home),str(helpers/'run-populated-settings.py'),
            '--url',self.url,'--output',str(self.output),'--profile','fake-provider','--project','firefox',
            '--state','fresh','--discovery',str(discovery),'--ui-fixtures',str(ui)],cwd=self.root,env=env,capture_output=True,text=True,timeout=10)
        self.assertEqual(result.returncode,1,result.stderr)
        self.assertEqual(self.requests,[])
        self.assertTrue(closed.exists(), result.stderr+result.stdout)
        receipt=json.loads((self.output/'setup-and-run.json').read_text())
        self.assertEqual(receipt['ownerSetup'],'pending')
        self.assertEqual(receipt['firefoxTrust']['category'],'certificate')
        self.assertEqual(receipt['firefoxTrust']['cleanup'],'closed')
        self.assertNotIn('private-detail',result.stdout+result.stderr+json.dumps(receipt))

    def test_unknown_remote_ambiguous_oversized_urls_have_no_effects(self):
        for value in (self.url + "?unknown=1", self.url + "?", self.url + "#",
                self.url + "/", " " + self.url, self.url + "\n",
                self.url.replace("://", "://\t"), "https://outside.invalid:1234", "ftp://localhost:1234",
                "https://user@localhost:1234", "http://localhost", "http://localhost:no",
                "http://localhost:0", "http://localhost:65536", "x" * 2049):
            with self.subTest(value=value[:64]):
                result = self.run_helper(value)
                self.assertEqual(result.returncode, 2)
                self.assertEqual(self.requests, [])
                self.assertFalse(self.output.exists())
                self.assertFalse(self.marker.exists())

    def test_missing_bad_private_oversized_or_foreign_ca_has_no_setup_effects(self):
        good = self.ca.read_text()
        for body in ("", "not a certificate", good + "PRIVATE KEY", good + "unknown\n", "x" * 262145):
            self.ca.write_text(body)
            result = self.run_helper()
            self.assertEqual(result.returncode, 2)
            self.assertEqual(self.requests, [])
            self.assertFalse(self.output.exists())
            self.assertFalse(self.marker.exists())
        self.ca.write_text(good)
        link = self.root / "link.crt"
        link.symlink_to(self.ca)
        for value in ("", str(self.root / "missing"), str(link), "x" * 4097):
            result = self.run_helper(NODE_EXTRA_CA_CERTS=value)
            self.assertEqual(result.returncode, 2)
            self.assertEqual(self.requests, [])
            self.assertFalse(self.output.exists())
            self.assertFalse(self.marker.exists())

    def test_https_requires_owned_webkit_linux_actions_context_before_effects(self):
        for env in ({"KINOSAIL_BROWSER_PROJECT": "chromium"}, {"KINOSAIL_BROWSER_TEST": ""},
                    {"CI": ""}, {"GITHUB_ACTIONS": ""}, {"RUNNER_OS": "macOS"}):
            result = self.run_helper(**env)
            self.assertEqual(result.returncode, 2)
            self.assertEqual(self.requests, [])
            self.assertFalse(self.output.exists())
            self.assertFalse(self.marker.exists())


if __name__ == "__main__":
    unittest.main()
