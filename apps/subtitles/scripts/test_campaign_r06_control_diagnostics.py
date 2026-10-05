"""Pure typed Go control diagnostics; no fixture, HTTP, files or process launch."""
import json
import unittest

from campaign_r06_browser_sources import GO_CASES, PACKAGE, GoProjection


class ControlDiagnostics(unittest.TestCase):
    def event(self, projection, name, action, output=None):
        value = {"Package":PACKAGE, "Action":action}
        if name is not None: value["Test"] = name
        if output is not None: value["Output"] = output
        projection.consume(json.dumps(value))

    def output(self, projection, name, payload, file="owner_test.go", line=44):
        self.event(projection, name, "output", f"    {file}:{line}: {payload}\n")

    def inventory(self, projection, name, csrf=True):
        for stage, status in (("setup",200), ("mfa",303), ("current",200)):
            self.output(projection, name, f"R06_OWNER_REQUEST {stage} {status} true")
        self.output(projection, name, f"R06_OWNER_SETUP 200 true true true {str(csrf).lower()}")
        self.output(projection, name, "R06_OWNER_CURRENT 200 true true")
        routes = [("catalog",200), ("inspection",200), ("preview",200)]
        if "Receipt" in name: routes.append(("prepare",201))
        for stage, status in routes:
            self.output(projection, name, f"R06_ROUTE {stage} {status} {status} true true true true")

    def completed(self, inventory=True, csrf=True):
        projection = GoProjection()
        for name in GO_CASES:
            self.event(projection, name, "run")
            if inventory: self.inventory(projection, name, csrf)
            self.event(projection, name, "pass")
        self.event(projection, None, "pass")
        return projection.result()

    def test_owned_setup_failure_retains_only_safe_status_and_stage(self):
        projection, name = GoProjection(), GO_CASES[0]
        self.event(projection, name, "run")
        self.output(projection, name, "R06_OWNER_REQUEST setup 200 true")
        self.output(projection, name, "R06_OWNER_SETUP 200 true true false true")
        self.output(projection, name, "actual TLS Owner/MFA/CSRF prerequisite failed", "fixture_test.go",25)
        self.event(projection, name, "fail")
        self.event(projection, None, "fail")
        result = projection.result()
        self.assertFalse(result["green"])
        self.assertEqual(result["invalidEvents"], 0)
        self.assertEqual(result["diagnostics"][0]["failures"], ["owner-auth"])
        self.assertEqual(result["diagnostics"][0]["ownerSetup"],
                         {"status":200,"read":True,"secure":True,"totp":False,"csrf":True})

    def test_four_passes_without_route_inventory_are_incomplete(self):
        self.assertFalse(self.completed(inventory=False)["green"])

    def test_four_actual_terminals_with_complete_inventory_are_green(self):
        self.assertTrue(self.completed()["green"])

    def test_false_presence_flag_cannot_be_green(self):
        self.assertFalse(self.completed(csrf=False)["green"])

    def test_unknown_or_malformed_markers_are_rejected(self):
        vectors = [("R06_OWNER_REQUEST setup 600 true","owner_test.go",44),
                   ("R06_OWNER_REQUEST setup 200 yes","owner_test.go",44),
                   ("R06_ROUTE unknown 200 200 true true true true","owner_test.go",44),
                   ("R06_OWNER_CURRENT 200 true true private","owner_test.go",44),
                   ("R06_OWNER_CURRENT 200 true true","fixture_test.go",25),
                   ("R06_OWNER_CURRENT 200 true true","owner_test.go",301)]
        for payload, file, line in vectors:
            with self.subTest(payload=payload, file=file, line=line):
                projection, name = GoProjection(), GO_CASES[0]
                self.event(projection, name, "run")
                self.output(projection, name, payload, file, line)
                self.assertEqual(projection.result()["invalidEvents"], 1)
                self.assertTrue(projection.prerequisite)

    def test_duplicate_marker_is_a_prerequisite_failure(self):
        projection, name = GoProjection(), GO_CASES[0]
        self.event(projection, name, "run")
        for _ in range(2):
            self.output(projection, name, "R06_OWNER_REQUEST setup 200 true")
        self.assertEqual(projection.result()["invalidEvents"], 1)

    def test_unknown_error_text_is_never_exported(self):
        projection, name = GoProjection(), GO_CASES[0]
        self.event(projection, name, "run")
        self.output(projection, name, "private-response-cookie-secret", "fixture_test.go",25)
        self.event(projection, name, "fail")
        self.event(projection, None, "fail")
        self.assertNotIn("private-response-cookie-secret", json.dumps(projection.result()))
        self.assertFalse(projection.result()["green"])

    def test_failure_location_must_match_owned_wrapper(self):
        projection, name = GoProjection(), GO_CASES[0]
        self.event(projection, name, "run")
        self.output(projection, name, "actual TLS Owner/MFA/CSRF prerequisite failed","fixture_test.go",24)
        self.assertEqual(projection.result()["invalidEvents"], 1)


if __name__ == "__main__":
    unittest.main()
