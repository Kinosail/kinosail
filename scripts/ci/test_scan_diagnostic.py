"""Protect private findings and the unchanged full-history diagnostic coverage."""
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch


SCRIPT = Path(__file__).with_name('scan-diagnostic.py')


def module():
    spec = importlib.util.spec_from_file_location('scan_diagnostic', SCRIPT)
    loaded = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(loaded)
    return loaded


class ScanDiagnosticTests(unittest.TestCase):
    def test_shared_projection_has_only_authorized_metadata(self):
        finding = {'RuleID': 'generic-api-key', 'File': 'packages/example.go',
                   'Commit': 'a' * 40, 'StartLine': 12,
                   'Secret': 'private-fixture-value', 'Match': 'private-context',
                   'Fingerprint': 'private-fingerprint', 'Author': 'private-author'}
        actual = module().project([finding])
        self.assertEqual(actual, [{'rule': 'generic-api-key', 'file': 'packages/example.go',
                                  'commit': 'a' * 40, 'line': 12}])
        self.assertNotIn('private', json.dumps(actual))

    def test_invalid_metadata_fails_without_echoing_private_input(self):
        valid = {'RuleID': 'generic-api-key', 'File': 'packages/example.go',
                 'Commit': 'a' * 40, 'StartLine': 12}
        for field, value in [('File', '../private-fixture-value'),
                             ('File', 'private-fixture-value\nunsafe'),
                             ('Commit', 'private-fixture-value'),
                             ('StartLine', True), ('StartLine', 0),
                             ('RuleID', 'private-fixture-value\nunsafe'),
                             ('File', '/private-fixture-value'),
                             ('File', 'private-fixture-value\\unsafe'),
                             ('StartLine', 10_000_001)]:
            with self.subTest(field=field, value=value):
                with self.assertRaisesRegex(ValueError, '^Invalid finding metadata$'):
                    module().project([{**valid, field: value}])

    def test_invalid_report_shape_is_rejected(self):
        for value in (None, {}, ['private-fixture-value'], [None]):
            with self.subTest(value=value):
                with self.assertRaisesRegex(ValueError, '^Invalid finding metadata$'):
                    module().project(value)

    def test_scanner_keeps_all_history_redaction_and_failure_exit(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            scanner = root / 'scanner'
            args = root / 'args.json'
            scanner.write_text('#!/usr/bin/env python3\nimport json,sys\nfrom pathlib import Path\n'
                               f'Path({str(args)!r}).write_text(json.dumps(sys.argv[1:]))\n'
                               'report=next(a.split("=",1)[1] for a in sys.argv if a.startswith("--report-path="))\n'
                               'Path(report).write_text(json.dumps([{\n'
                               '"RuleID":"generic-api-key","File":"packages/example.go",\n'
                               '"Commit":"' + 'a' * 40 + '","StartLine":12,\n'
                               '"Secret":"private-fixture-value","Match":"private-context"}]))\n'
                               'raise SystemExit(1)\n')
            scanner.chmod(0o700)
            output = root / 'safe.json'
            result = subprocess.run(['python3', str(SCRIPT), '--scanner', str(scanner),
                                     '--output', str(output)], cwd=root, capture_output=True, text=True)
            self.assertEqual(result.returncode, 1, result.stderr)
            actual_args = json.loads(args.read_text())
            self.assertEqual(actual_args[:5], ['git', '--redact', '--no-banner', '--log-opts=--all',
                                              '--report-format=json'])
            self.assertEqual(len(actual_args), 6)
            actual = json.loads(output.read_text())
            self.assertEqual(actual['scanner_exit_code'], 1)
            self.assertEqual(actual['coverage'], '--all')
            self.assertEqual(len(actual['findings']), 1)
            self.assertNotIn('private', output.read_text() + result.stdout + result.stderr)

    def test_tool_error_does_not_publish_a_successful_projection(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            scanner = root / 'scanner'
            scanner.write_text('#!/usr/bin/env python3\nraise SystemExit(2)\n')
            scanner.chmod(0o700)
            output = root / 'safe.json'
            result = subprocess.run(['python3', str(SCRIPT), '--scanner', str(scanner),
                                     '--output', str(output)], cwd=root, capture_output=True, text=True)
            self.assertEqual(result.returncode, 2)
            self.assertFalse(output.exists())

    def test_timeout_and_invalid_report_publish_nothing_private(self):
        loaded = module()
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'safe.json'

            def malformed(command, **kwargs):
                path = next(arg.split('=', 1)[1] for arg in command
                            if arg.startswith('--report-path='))
                Path(path).write_text('private-fixture-value')
                return subprocess.CompletedProcess(command, 1)

            for effect in (subprocess.TimeoutExpired(['private-fixture-value'], 180), malformed):
                with self.subTest(effect=type(effect).__name__):
                    with patch.object(loaded.subprocess, 'run', side_effect=effect):
                        self.assertEqual(loaded.scan('scanner', output), 2)
                    self.assertFalse(output.exists())

    def test_exit_code_and_report_must_agree(self):
        loaded = module()
        finding = {'RuleID': 'generic-api-key', 'File': 'packages/example.go',
                   'Commit': 'a' * 40, 'StartLine': 12}
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'safe.json'
            for code, findings, expected in ((0, [], 0), (0, [finding], 2), (1, [], 2)):
                def result(command, **kwargs):
                    report = next(arg.split('=', 1)[1] for arg in command
                                  if arg.startswith('--report-path='))
                    Path(report).write_text(json.dumps(findings))
                    return subprocess.CompletedProcess(command, code)
                with self.subTest(code=code, findings=len(findings)):
                    output.unlink(missing_ok=True)
                    with patch.object(loaded.subprocess, 'run', side_effect=result):
                        self.assertEqual(loaded.scan('scanner', output), expected)
                    self.assertEqual(output.exists(), expected == 0)

    def test_manual_projection_does_not_replace_the_existing_gate(self):
        workflow = SCRIPT.parents[2] / '.github/workflows/ci.yml'
        source = workflow.read_text()
        gate = source.split('  secrets:\n', 1)[1].split('\n  codeql:', 1)[0]
        self.assertEqual(gate.count('run: python3 scripts/ci/secrets.py'), 1)
        self.assertEqual(gate.count('go install -ldflags="-X github.com/zricethezav/gitleaks/v8/version.Version=8.30.1" github.com/zricethezav/gitleaks/v8@v8.30.1'), 1)
        self.assertIn("always() && github.event_name == 'workflow_dispatch' && inputs.scan_diagnostic", gate)
        self.assertNotIn('continue-on-error', gate)
        self.assertIn('path: ${{ runner.temp }}/scan-safe.json', gate)
        self.assertNotIn('path: ${{ runner.temp }}\n', gate)


if __name__ == '__main__':
    unittest.main()
