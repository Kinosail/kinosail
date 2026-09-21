"""Security coverage and policy must survive workflow maintenance."""
import json
from pathlib import Path
import subprocess
import unittest
from affected import LANGUAGES, affected

ROOT = Path(__file__).resolve().parents[2]


class SecurityContracts(unittest.TestCase):
    def test_all_supported_sources_have_a_maintained_scan(self):
        self.assertEqual(set(LANGUAGES), {"go", "javascript-typescript", "python", "swift", "actions"})
        self.assertTrue(affected(["apps/player/apps/native/Sources/App.swift"])["swift"])
        self.assertTrue(affected([".github/workflows/security.yml"])["actions"])
        workflow = (ROOT / ".github/workflows/security.yml").read_text()
        self.assertIn("matrix.language == 'swift' && 'macos-latest'", workflow)
        self.assertIn("matrix.language == 'swift') && 'manual'", workflow)
        self.assertIn("make -C apps/player client-check", workflow)
        self.assertIn("ARCHS = arm64", workflow)
        self.assertIn('XCODE_XCCONFIG_FILE="$RUNNER_TEMP/codeql.xcconfig"', workflow)
        self.assertIn("matrix.language == 'swift' && 30 || 15", workflow)
        self.assertIn("queries: security-extended", workflow)

    def test_open_medium_and_low_security_findings_also_block_merge(self):
        workflow = (ROOT / ".github/workflows/security.yml").read_text()
        expression = next(line.split("jq -e '", 1)[1].split("' alerts.json", 1)[0]
                          for line in workflow.splitlines() if "jq -e '" in line)
        for severity in ("low", "medium", "high", "critical", None):
            alerts = [[{"rule": {"security_severity_level": severity}}]]
            result = subprocess.run(["jq", "-e", expression], input=json.dumps(alerts),
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, 0 if severity is None else 1)
