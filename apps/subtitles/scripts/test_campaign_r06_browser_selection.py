"""Pure fixed suite selection against Playwright's documented full-title contract."""
import ast
from pathlib import Path
import re
import unittest

from campaign_r06_browser_projection import CASES, FILE

# https://playwright.dev/docs/api/class-testconfig#test-config-grep
# grep receives project/file/describe/test/tags joined by spaces, not test name alone.
def suites_from_source(source):
    for node in ast.parse(source).body:
        if isinstance(node, ast.Assign) and any(isinstance(t, ast.Name) and t.id == "SUITES" for t in node.targets):
            return ast.literal_eval(node.value)
    raise ValueError("fixed-suite-source")

def driver_suites():
    return suites_from_source(Path(__file__).with_name("campaign-r06-browser.py").read_text(encoding="utf-8"))

class SuiteSelectionControls(unittest.TestCase):
    def setUp(self):
        self.suites = driver_suites()

    def selected(self, suite, prefix=""):
        pattern = self.suites[suite][2]
        return sorted(case for case, title in CASES.items()
                      if re.search(pattern, prefix + Path(FILE).name + " " + title))

    def test_headers_select_exact_two_documented_full_titles(self):
        self.assertEqual(self.selected("save-headers"),
                         ["save-headers-desktop", "save-headers-phone"])

    def test_body_select_exact_two_documented_full_titles(self):
        self.assertEqual(self.selected("save-body"),
                         ["save-body-desktop", "save-body-phone"])

    def test_project_and_describe_prefix_preserve_exact_selection(self):
        for suite in ("save-headers", "save-body"):
            for prefix in ("chromium ", "chromium owned suite ", "private fixture "):
                with self.subTest(suite=suite, prefix=prefix):
                    self.assertEqual(self.selected(suite, prefix), sorted(self.suites[suite][1]))

    def test_controls_select_no_browser_case(self):
        self.assertEqual(self.selected("save-controls"), [])

    def test_similar_titles_and_extra_suffix_never_match(self):
        for suite in ("save-headers", "save-body"):
            fault = "headers" if suite == "save-headers" else "body"
            for title in (f"R06 Save {fault} held after completed write - tablet",
                          f"XR06 Save {fault} held after completed write - phone",
                          f"R06 Save {fault} held after completed write - phone extra",
                          f"R06 Save {fault} held after completed write - phone @extra"):
                with self.subTest(suite=suite, title=title):
                    self.assertIsNone(re.search(self.suites[suite][2], Path(FILE).name + " " + title))

    def test_original_test_name_forms_and_suite_inventory_are_preserved(self):
        self.assertEqual(set(self.suites), {"save-controls", "save-headers", "save-body"})
        for suite in ("save-headers", "save-body"):
            matched = sorted(case for case, title in CASES.items() if re.search(self.suites[suite][2], title))
            self.assertEqual(matched, sorted(self.suites[suite][1]))

if __name__ == "__main__":
    unittest.main()
