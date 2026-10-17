"""Security coverage and policy must survive workflow maintenance."""
import json
from pathlib import Path
import subprocess
import unittest
from affected import LANGUAGES, affected

ROOT = Path(__file__).resolve().parents[2]


class SecurityContracts(unittest.TestCase):
    def test_all_supported_sources_have_a_maintained_scan(self):
        self.assertEqual(set(LANGUAGES), {"go", "java-kotlin", "javascript-typescript", "python", "actions"})
        self.assertTrue(affected(["apps/player/apps/native/Sources/App.swift"])["client"])
        self.assertTrue(affected(["apps/player/apps/android/app/src/main/java/App.kt"])["java-kotlin"])
        self.assertTrue(affected([".github/workflows/ci.yml"])["actions"])
        ci = (ROOT / ".github/workflows/ci.yml").read_text()
        workflow = (ROOT / ".github/workflows/deep.yml").read_text()
        self.assertIn("  swift-analysis:\n    name: Swift code scanning\n    runs-on: xcode-27", workflow)
        self.assertIn("DEVELOPER_DIR=/Applications/Xcode_27.1.app/Contents/Developer", workflow)
        toolchains = [line.split("DEVELOPER_DIR:", 1)[1].strip()
                      for line in (ci + workflow).splitlines() if "DEVELOPER_DIR:" in line]
        self.assertEqual(toolchains, ["/Applications/Xcode_27.1.app/Contents/Developer"] * 5)
        self.assertIn("languages: javascript-typescript,python,actions", workflow)
        self.assertIn("languages: go,java-kotlin", workflow)
        self.assertIn("languages: ${{ needs.plan.outputs.codeql }}", ci)
        self.assertIn("languages: ${{ needs.plan.outputs.compiled }}", ci)
        for source in (ci, workflow):
            interpreted = source.split("build-mode: none", 1)[0].rsplit("languages:", 1)[1]
            compiled = source.split("build-mode: manual", 1)[0].rsplit("languages:", 1)[1]
            self.assertNotIn("go", interpreted)
            self.assertNotIn("javascript", compiled)
        self.assertIn("queries: security-extended", ci)
        self.assertIn("config-file: .github/codeql-config.yml", ci)
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
        browser_step = ci.split("- if: contains(needs.plan.outputs.codeql, 'javascript-typescript')", 1)[1].split("- if:", 1)[0]
        self.assertNotIn("prepare-codeql-js", ci + workflow)
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
