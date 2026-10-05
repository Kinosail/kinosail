"""Pinned real-scanner controls for narrowly approved receipt digests."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SCANNER = os.environ.get("KINOSAIL_REAL_SCANNER")
APPROVED = (
    ("7ce5934078430675f83aef91681b0443a1ddba32", "q14-browse-return", "runtime-collection-prerequisite.json", 44),
    ("7ce5934078430675f83aef91681b0443a1ddba32", "q14-browse-return", "direct-cli-preparation-context.json", 34),
    ("7ce5934078430675f83aef91681b0443a1ddba32", "q14-browse-return", "collection-repair-context.json", 70),
    ("c5563c73971f81873639aed5a0e517c785f1096a", "q14-browse-return", "prepared-context.json", 29),
    ("d02c9534d2e4a67e34de979f6bfb443d7553b0e5", "r08-now-playing", "green-native-receipt.json", 75),
    ("2776f4ce39fc7084ff20cac18e895b7a373fe685", "r08-now-playing", "visibility-red-receipt.json", 72),
    ("51cb1a903ac3eb1f4c104a6a76a580a3bd2898bd", "r08-now-playing", "initial-native-receipt.json", 62),
    ("6da6ce46f9ab1589f42774b7713ed28cb2d4e4ff", "r08-now-playing", "red-receipt.json", 55),
)


def fingerprints():
    return {f"{commit}:engineering/qa/2026-10-04-{group}/{file}:generic-api-key:{line}"
            for commit, group, file, line in APPROVED}


class ReceiptFingerprintPolicy(unittest.TestCase):
    def test_eight_approved_digest_locations_are_exact(self):
        actual = set((ROOT / ".gitleaksignore").read_text().splitlines())
        self.assertTrue(fingerprints().issubset(actual))

    def test_hosted_secret_gate_runs_real_controls_before_scan(self):
        workflow = (ROOT / ".github/workflows/ci.yml").read_text()
        self.assertTrue("KINOSAIL_REAL_SCANNER: gitleaks" in workflow)
        self.assertLess(workflow.index("python3 -m unittest scripts.ci.test_receipt_fingerprints"),
                        workflow.index("run: python3 scripts/ci/secrets.py"))


@unittest.skipUnless(SCANNER, "real scanner is explicitly run by the hosted secret gate")
class RealReceiptScanner(unittest.TestCase):
    def setUp(self):
        self.assertEqual(subprocess.check_output([SCANNER, "version"], text=True).strip(), "8.30.1")
        self.temp = tempfile.TemporaryDirectory(prefix="kinosail-scanner-control-")
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name)
        self.env = {**os.environ, "GIT_CONFIG_NOSYSTEM": "1",
                    "GIT_CONFIG_GLOBAL": os.devnull}
        # An inherited diagnostic GIT_DIR must never redirect the disposable Git.
        for key in ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY",
                    "GIT_ALTERNATE_OBJECT_DIRECTORIES"):
            self.env.pop(key, None)
        self.git("init", "--quiet")
        self.git("config", "user.name", "Disposable scanner fixture")
        self.git("config", "user.email", "fixture@example.invalid")
        shutil.copyfile(ROOT / ".gitleaks.toml", self.repo / ".gitleaks.toml")
        self.path = Path("engineering/qa/2026-10-04-r08-now-playing/green-native-receipt.json")
        (self.repo / self.path).parent.mkdir(parents=True)
        digest = hashlib.sha256(b"disposable synthetic source bytes").hexdigest()
        self.write({"checksums": {"config/audit_key.json": digest}})
        self.commit("Record generated fixture digest")
        code, findings = self.scan()
        self.assertEqual(code, 1)
        self.assertEqual(len(findings), 1)
        self.original = findings[0]

    def git(self, *args):
        return subprocess.check_output(["git", *args], cwd=self.repo, env=self.env,
                                       stderr=subprocess.DEVNULL, text=True).strip()

    def write(self, body):
        (self.repo / self.path).write_text(json.dumps(body, indent=2) + "\n")

    def commit(self, title):
        self.git("add", ".")
        self.git("commit", "--quiet", "-m", title)
        return self.git("rev-parse", "HEAD")

    def scan(self):
        report = self.repo / "private-redacted.json"
        result = subprocess.run([SCANNER, "git", "--redact", "--no-banner",
                                 "--log-opts=--all", "--report-format=json",
                                 "--report-path=" + str(report)],
                                cwd=self.repo, env=self.env, timeout=30,
                                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        self.assertIn(result.returncode, (0, 1))
        # Never emit scanner match/secret/author fields, even on assertion failure.
        projected = [{key: row[key] for key in ("Commit", "File", "RuleID", "StartLine")}
                     for row in json.loads(report.read_text())]
        report.unlink()
        return result.returncode, projected

    @staticmethod
    def fingerprint(row, line=None):
        return f"{row['Commit']}:{row['File']}:{row['RuleID']}:{line or row['StartLine']}"

    def test_exact_digest_exception_keeps_new_credential_at_same_path_detectable(self):
        (self.repo / ".gitleaksignore").write_text(self.fingerprint(self.original) + "\n")
        self.assertEqual(self.scan(), (0, []))
        # This is a deliberately synthetic scanner payload; no service accepts it.
        self.write({"checksums": {"config/audit_key.json": "synthetic"},
                    "api_key": hashlib.sha256(b"synthetic credential control").hexdigest()})
        head = self.commit("Add synthetic credential near-miss control")
        code, findings = self.scan()
        self.assertEqual(code, 1)
        self.assertEqual(len(findings), 1)
        self.assertEqual(findings[0]["Commit"], head)
        self.assertEqual(findings[0]["File"], str(self.path))

    def test_structured_digest_receipts_pass_without_masking_credentials(self):
        (self.repo / ".gitleaksignore").write_text(self.fingerprint(self.original) + "\n")
        digest = hashlib.sha256(b"synthetic file bytes").hexdigest()
        self.write({"checksums": [{"path": "config/audit_key.json", "sha256": digest}],
                    "sources": [{"path": "packages/identitycore/library_access.go", "sha256": digest}]})
        self.commit("Separate receipt paths from verified file digests")
        self.assertEqual(self.scan(), (0, []))
        self.write({"checksums": [{"path": "config/audit_key.json", "sha256": digest}],
                    "api_key": hashlib.sha256(b"synthetic scanner positive control").hexdigest()})
        head = self.commit("Retain synthetic credential detection")
        code, findings = self.scan()
        self.assertEqual(code, 1)
        self.assertEqual(len(findings), 1)
        self.assertEqual(findings[0]["Commit"], head)

    def test_wrong_line_fingerprint_cannot_suppress_the_finding(self):
        (self.repo / ".gitleaksignore").write_text(
            self.fingerprint(self.original, self.original["StartLine"] + 1) + "\n")
        code, findings = self.scan()
        self.assertEqual(code, 1)
        self.assertEqual(findings, [self.original])


if __name__ == "__main__":
    unittest.main()
