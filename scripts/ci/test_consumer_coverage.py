"""Consumer-only tests must cover executed production code across the module.

App E2E cannot inspect the CI collector's instrumentation. This isolated command
test runs real Go, HTTP and file I/O in a disposable module, including a negative
control that must retain uncovered production statements and fail the gate.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SOURCES = {
    'go.mod': 'module qa.invalid/consumer\n\ngo 1.22\n',
    'state/state.go': '''package state
import "os"
func Save(path string) error { return os.WriteFile(path, []byte("qa"), 0600) }
''',
    'consumer/handler.go': '''package consumer
import ("fmt"; "net/http"; "qa.invalid/consumer/state")
func Handler(path string) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  fmt.Fprint(w, state.Save(path) == nil)
 })
}
''',
    'consumer/handler_test.go': '''package consumer_test
import ("io"; "net/http"; "net/http/httptest"; "os"; "path/filepath"; "testing"; "qa.invalid/consumer/consumer")
func TestHTTPPersistsState(t *testing.T) {
 path := filepath.Join(t.TempDir(), "state")
 server := httptest.NewServer(consumer.Handler(path)); defer server.Close()
 response, err := http.Get(server.URL)
 if err != nil { t.Fatal(err) }; defer response.Body.Close()
 body, err := io.ReadAll(response.Body)
 if err != nil || response.StatusCode != 200 || string(body) != "true" { t.Fatal("HTTP result") }
 data, err := os.ReadFile(path)
 if err != nil || string(data) != "qa" { t.Fatal("state side effect") }
}
''',
}
UNEXECUTED = '''
func Unexecuted() int {
 value := 1
 value += 2
 value *= 3
 value -= 4
 return value
}
'''


class ConsumerCoverageTests(unittest.TestCase):
    def test_consumer_execution_counts_without_hiding_unexecuted_code(self):
        for uncovered in (False, True):
            with self.subTest(uncovered=uncovered), tempfile.TemporaryDirectory() as directory:
                repo = Path(directory)
                app = repo / 'apps/player'
                for name, content in SOURCES.items():
                    target = app / name
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_text(content + (UNEXECUTED if uncovered and name == 'state/state.go' else ''))
                script = repo / 'scripts/ci/test-go.sh'
                script.parent.mkdir(parents=True)
                shutil.copy2(ROOT / 'scripts/ci/test-go.sh', script)
                env = os.environ | {'GOWORK': 'off', 'GOTOOLCHAIN': 'local',
                                    'CGO_ENABLED': '1', 'KINOSAIL_GO_TEST_MODE': 'deep'}
                result = subprocess.run(['bash', str(script), 'player'], env=env, capture_output=True)
                log = result.stdout + result.stderr
                profile = app / '.verification/coverage.out'
                self.assertTrue(profile.is_file())
                coverage = subprocess.run(['go', 'tool', 'cover', '-func=' + str(profile)],
                                          cwd=app, env=env, capture_output=True, check=True).stdout
                output = os.environ.get('KINOSAIL_QA_CONSUMER_COVERAGE_EVIDENCE')
                if output:
                    destination = Path(output) / ('unexecuted' if uncovered else 'executed')
                    destination.mkdir(mode=0o700, parents=True, exist_ok=False)
                    files = {'command-private.log': log, 'coverage.out': profile.read_bytes(),
                             'coverage-functions.txt': coverage, 'test-go.sh': script.read_bytes()}
                    for name, content in files.items():
                        (destination / name).write_bytes(content)
                    receipt = {'exit': result.returncode, 'unexecutedControl': uncovered,
                               'sha256': {name: hashlib.sha256(content).hexdigest() for name, content in files.items()}}
                    (destination / 'safe-receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
                self.assertRegex(log, rb'(?m)^ok\s+qa.invalid/consumer/consumer\s')
                self.assertNotIn(b'WARNING: DATA RACE', log)
                if uncovered:
                    self.assertEqual(result.returncode, 1)
                    self.assertIn(b'is below 89%', log)
                    self.assertRegex(coverage, rb'Unexecuted\s+0\.0%')
                else:
                    self.assertEqual(result.returncode, 0)
                    self.assertRegex(coverage, rb'Save\s+100\.0%')
                    self.assertRegex(coverage, rb'total:\s+\(statements\)\s+100\.0%')


if __name__ == '__main__':
    unittest.main()
