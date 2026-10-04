"""Fictional suite admission controls; no Go, Node or browser is invoked."""
import copy
import unittest

import campaign_q14_admission as admission

SPECS = ["browse-return.spec.ts", "browse-return-cold.spec.ts", "browse-return-bfcache.spec.ts",
         "browse-return-safety.spec.ts", "browse-return-home.spec.ts"]
PAIRS = {
    "cold": [(SPECS[1], f"cold native Back restores later Movie cards at {width}px") for width in (390, 1440)],
    "bfcache": [(SPECS[2], "native BFCache preserves loaded Movie DOM without repeated continuation")],
    "htmx": [(SPECS[0], "HTMX title-letter Back fetches current browse data and restores extent without a native pageshow")],
    "shows": [(SPECS[0], f"visible Player Back restores original Shows action via {action}") for action in ("direct Play", "details and episode")],
    "search": [(SPECS[1], "live query uses current URL rather than the document's initial browse key")],
    "safety": [(SPECS[3], f"saved return rejects {name} before navigation or continuation") for name in (
        "external origin", "protocol-relative origin", "non-browse route", "duplicate query", "unknown query",
        "oversized query", "excessive extent", "different profile", "different destination")]
        + [(SPECS[3], "a direct Player opened in another tab does not inherit browse return state")],
    "home": [(SPECS[4], title) for title in (
        "visible Player Back restores the Home Continue watching action without a Library grid",
        "cold native Back restores the Home Continue watching action without a Library grid",
        "cold native Back restores the scrolled Home Movies destination without a Library grid")],
}


def collection(suite):
    return {"schemaVersion": 2, "suite": suite, "status": "passed", "errors": [], "cases": [],
            "collected": [{"file": file, "title": title} for file, title in PAIRS[suite]]}


class FixedSuiteTests(unittest.TestCase):
    def test_every_fixed_suite_collects_exact_names(self):
        for suite in PAIRS:
            with self.subTest(suite=suite):
                value = collection(suite)
                self.assertEqual(admission.admit(value, True, suite), value)

    def test_wrong_suite_names_and_arbitrary_extra_data_fail_closed(self):
        for suite in PAIRS:
            for mutate in (lambda value: value.update(suite="all"),
                           lambda value: value["collected"].pop(),
                           lambda value: value["collected"][0].update(title="fictional-private-value"),
                           lambda value: value.update(rawURL="https://fictional.invalid/secret")):
                candidate = collection(suite)
                mutate(candidate)
                with self.subTest(suite=suite, candidate=candidate):
                    self.assertIsNone(admission.admit(candidate, True, suite))

    def test_legacy_primary_cannot_be_used_to_certify_new_suite(self):
        legacy = {"schemaVersion": 1, "status": "passed", "errors": [], "cases": [],
                  "collected": [{"file": file, "title": title} for file, title in admission.COLLECTION]}
        for suite in PAIRS:
            with self.subTest(suite=suite):
                self.assertIsNone(admission.admit(copy.deepcopy(legacy), True, suite))


if __name__ == "__main__":
    unittest.main()
