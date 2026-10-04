"""Protect required CI evidence: a green browser exit must not mask skipped journeys.

The browser E2E suite cannot detect a helper that accepts an incomplete results
file. Load only the verifier so this isolated regression has no setup side effects.
"""
import ast
import hashlib
import json
from pathlib import Path
import tempfile
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


if __name__ == "__main__":
    unittest.main()
