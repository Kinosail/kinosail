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
        swift = workflow.split("  swift:\n", 1)[1].split("  supply-chain:", 1)[0]
        self.assertIn("platform: [iOS, tvOS]", swift)
        self.assertIn("build-mode: manual", swift)
        self.assertIn("category: /language:swift/platform:${{ matrix.platform }}", swift)
        self.assertIn("timeout-minutes: 30", swift)
        self.assertIn("runs-on: macos-26-intel", swift)
        build = (ROOT / "scripts/ci/build-codeql-swift.sh").read_text()
        for option in ("-jobs 1", "ARCHS=arm64", "COMPILATION_CACHE_ENABLE_CACHING=NO",
                       "SWIFT_ENABLE_COMPILE_CACHE=NO", "SWIFT_USE_INTEGRATED_DRIVER=NO",
                       "SWIFT_COMPILATION_MODE=wholemodule", "SWIFT_USE_PARALLEL_WHOLE_MODULE_OPTIMIZATION=NO",
                       "SWIFT_USE_PARALLEL_WMO_TARGETS=NO", "COMPILER_INDEX_STORE_ENABLE=NO",
                       "OTHER_SWIFT_FLAGS=$(inherited) -num-threads 1"):
            self.assertIn(option, build)
        self.assertIn("needs: [codeql, swift]", workflow)
        self.assertIn("always() && (inputs.languages != '[]' || fromJSON(inputs.plan).swift)", workflow)
        self.assertIn("queries: security-extended", workflow)
        self.assertIn("config-file: .github/codeql-config.yml", workflow)
        self.assertLess(workflow.index("node scripts/ci/prepare-codeql-js.mjs"),
                        workflow.index("uses: github/codeql-action/init@"))
        browser_step = workflow.split("- name: Assemble complete browser source for CodeQL", 1)[1].split("- uses:", 1)[0]
        self.assertIn("if: matrix.language == 'javascript-typescript'", browser_step)
        for setting in ("CODEQL_ACTION_DIFF_INFORMED_QUERIES=false", "CODEQL_OVERLAY_DATABASE_MODE=none"):
            self.assertIn(setting, browser_step)

    def test_invalid_swift_target_cannot_run_build_or_monitor(self):
        import os
        import tempfile
        with tempfile.TemporaryDirectory() as directory:
            marker = Path(directory) / "ran"
            for tool in ("xcodebuild", "python3", "tee"):
                binary = Path(directory) / tool
                binary.write_text(f'#!/bin/sh\ntouch "{marker}"\n')
                binary.chmod(0o755)
            env = os.environ | {"PATH": directory + ":" + os.environ["PATH"]}
            for args in ([], [""], ["watchOS"], ["iOS", "tvOS"], ["x" * 10000]):
                result = subprocess.run(["bash", str(ROOT / "scripts/ci/build-codeql-swift.sh"), *args],
                                        capture_output=True, env=env)
                self.assertEqual(result.returncode, 2)
                self.assertFalse(marker.exists())

    def test_swift_build_preserves_driver_flags_and_compiler_failure(self):
        import os
        import sys
        import tempfile
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            compiler = root / "xcodebuild"
            compiler.write_text(f'#!{sys.executable}\nimport json, os, sys\n'
                                'print(json.dumps(sys.argv[1:]))\n'
                                'sys.exit(int(os.environ["BUILD_RESULT"]))\n')
            compiler.chmod(0o755)
            monitor = root / "python3"
            monitor.write_text('#!/bin/sh\nexit 0\n')
            monitor.chmod(0o755)
            env = os.environ | {"PATH": directory + ":" + os.environ["PATH"],
                                "RUNNER_TEMP": directory, "GITHUB_SHA": "a" * 40}
            for platform, status in (("iOS", 0), ("tvOS", 17)):
                result = subprocess.run(["bash", str(ROOT / "scripts/ci/build-codeql-swift.sh"), platform],
                                        env=env | {"BUILD_RESULT": str(status)}, capture_output=True, text=True)
                self.assertEqual(result.returncode, status)
                args = json.loads(result.stdout)
                self.assertEqual(args[args.index("-scheme") + 1], "Kinosail-" + platform)
                self.assertIn("OTHER_SWIFT_FLAGS=$(inherited) -num-threads 1", args)
                self.assertIn("COMPILER_INDEX_STORE_ENABLE=NO", args)
                self.assertEqual((root / "codeql-swift-build.log").read_text(), result.stdout)

    def test_open_medium_and_low_security_findings_also_block_merge(self):
        workflow = (ROOT / ".github/workflows/security.yml").read_text()
        expression = next(line.split("jq -e '", 1)[1].split("' alerts.json", 1)[0]
                          for line in workflow.splitlines() if "jq -e '" in line)
        for severity in ("low", "medium", "high", "critical", None):
            alerts = [[{"rule": {"security_severity_level": severity}}]]
            result = subprocess.run(["jq", "-e", expression], input=json.dumps(alerts),
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, 0 if severity is None else 1)
