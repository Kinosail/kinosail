"""Security coverage and policy must survive workflow maintenance."""
import json
from pathlib import Path
import subprocess
import unittest
from affected import LANGUAGES, affected

ROOT = Path(__file__).resolve().parents[2]


class SecurityContracts(unittest.TestCase):
    def test_all_supported_sources_have_a_maintained_scan(self):
        self.assertEqual(set(LANGUAGES), {"go", "javascript-typescript", "python", "actions"})
        self.assertTrue(affected(["apps/player/apps/native/Sources/App.swift"])["client"])
        self.assertTrue(affected([".github/workflows/ci.yml"])["actions"])
        workflow = (ROOT / ".github/workflows/ci.yml").read_text()
        self.assertIn("  swift-analysis:\n    if: github.event_name == 'schedule'", workflow)
        self.assertIn("    runs-on: macos-latest", workflow)
        self.assertIn("languages: swift", workflow)
        self.assertIn("build-mode: manual", workflow)
        self.assertIn("make -C apps/player client-check", workflow)
        self.assertIn("ARCHS = arm64", workflow)
        for setting in ("COMPILATION_CACHE_ENABLE_CACHING", "SWIFT_ENABLE_COMPILE_CACHE", "SWIFT_USE_INTEGRATED_DRIVER"):
            self.assertIn(f"{setting} = NO", workflow)
        self.assertIn('XCODE_XCCONFIG_FILE="$RUNNER_TEMP/codeql.xcconfig"', workflow)
        self.assertIn("timeout-minutes: 90", workflow)
        self.assertIn("queries: security-extended", workflow)
        self.assertIn("config-file: .github/codeql-config.yml", workflow)
        self.assertLess(workflow.index("node scripts/ci/prepare-codeql-js.mjs"),
                        workflow.index("uses: github/codeql-action/init@"))
        browser_step = workflow.split("- if: matrix.language == 'javascript-typescript'", 1)[1].split("- uses:", 1)[0]
        self.assertIn("node scripts/ci/prepare-codeql-js.mjs", browser_step)
        for setting in ("CODEQL_ACTION_DIFF_INFORMED_QUERIES=false", "CODEQL_OVERLAY_DATABASE_MODE=none"):
            self.assertIn(setting, browser_step)

    def test_open_medium_and_low_security_findings_also_block_merge(self):
        workflow = (ROOT / ".github/workflows/ci.yml").read_text()
        expression = next(line.split("jq -e '", 1)[1].split("' alerts.json", 1)[0]
                          for line in workflow.splitlines() if "jq -e '" in line)
        for severity in ("low", "medium", "high", "critical", None):
            alerts = [[{"rule": {"security_severity_level": severity}}]]
            result = subprocess.run(["jq", "-e", expression], input=json.dumps(alerts),
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, 0 if severity is None else 1)
