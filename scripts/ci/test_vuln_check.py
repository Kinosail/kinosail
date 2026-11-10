"""The vulnerability gate fails on every called finding except a dated, version-bound exception."""
import datetime
import json
from pathlib import Path
import tempfile
import unittest

import vuln_check

TODAY = datetime.date(2026, 11, 9)
EXCEPTION = {"id": "GO-2026-6909", "module": "github.com/crewjam/saml", "fixed": "v0.5.2", "expires": "2026-12-09",
             "reason": "The advisory text says through v0.5.1; the database record has no fixed version."}


def finding(osv="GO-2026-6909", module="github.com/crewjam/saml", version="v0.6.3", called=True):
    frame = {"module": module, "version": version, "package": module}
    if called:
        frame["function"] = "ParseResponse"
    return {"finding": {"osv": osv, "trace": [frame, {"module": "example.test/app", "function": "main"}]}}


def stream(*messages):
    return "\n".join(json.dumps(message, indent=1) for message in ({"config": {"scan_level": "symbol"}}, *messages))


class VulnerabilityGateTests(unittest.TestCase):
    def test_called_findings_fail_unless_a_current_exception_matches_the_module_and_fixed_version(self):
        cases = (
            ((finding(),), [EXCEPTION], []),
            ((finding(), finding()), [EXCEPTION], []),
            ((finding(called=False),), [], []),
            ((finding(osv="GO-2026-0001", called=False),), [], []),
            ((), [EXCEPTION], []),
            ((finding(),), [], ["GO-2026-6909"]),
            ((finding(version="v0.5.1"),), [EXCEPTION], ["GO-2026-6909"]),
            ((finding(version="v0.5.2-0.20260101000000-abcdefabcdef"),), [EXCEPTION], ["GO-2026-6909"]),
            ((finding(version=""),), [EXCEPTION], ["GO-2026-6909"]),
            ((finding(module="github.com/other/saml"),), [EXCEPTION], ["GO-2026-6909"]),
            ((finding(), finding(osv="GO-2026-7000", module="example.test/lib", version="v1.0.0")), [EXCEPTION], ["GO-2026-7000"]),
        )
        for messages, exceptions, expected in cases:
            with self.subTest(messages=messages, exceptions=exceptions):
                self.assertEqual(vuln_check.blocking(stream(*messages), exceptions, TODAY), expected)
        self.assertEqual(vuln_check.blocking(stream(finding()), [EXCEPTION], datetime.date(2026, 12, 9)), [])
        self.assertEqual(vuln_check.blocking(stream(finding()), [EXCEPTION], datetime.date(2026, 12, 10)), ["GO-2026-6909"])

    def test_output_without_a_scan_or_with_invalid_json_fails(self):
        for output in ("", "{}", json.dumps(finding()), "not json", stream() + "\n{", "[]"):
            with self.subTest(output=output), self.assertRaises(ValueError):
                vuln_check.blocking(output, [EXCEPTION], TODAY)

    def test_invalid_exception_files_fail_and_the_recorded_file_holds_one_dated_entry(self):
        invalid = ({}, [{}], [EXCEPTION | {"id": "CVE-2026-1"}], [EXCEPTION | {"expires": "soon"}], [EXCEPTION | {"fixed": "0.5.2"}],
                   [EXCEPTION | {"module": ""}], [EXCEPTION | {"reason": "short"}], [EXCEPTION | {"extra": 1}],
                   [{key: value for key, value in EXCEPTION.items() if key != "expires"}], [EXCEPTION, EXCEPTION])
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "exceptions.json"
            for content in invalid:
                path.write_text(json.dumps(content))
                with self.subTest(content=content), self.assertRaises(ValueError):
                    vuln_check.load_exceptions(path)
            path.write_text("not json")
            with self.assertRaises(ValueError):
                vuln_check.load_exceptions(path)
            path.write_text("[]")
            self.assertEqual(vuln_check.load_exceptions(path), [])
        recorded = vuln_check.load_exceptions()
        self.assertEqual([(entry["id"], entry["module"], entry["fixed"]) for entry in recorded],
                         [("GO-2026-6909", "github.com/crewjam/saml", "v0.5.2")])
        self.assertLessEqual(datetime.date.fromisoformat(recorded[0]["expires"]), datetime.date(2026, 12, 9))

    def test_the_command_pins_the_tool_and_scans_one_validated_module(self):
        self.assertEqual(vuln_check.command("apps/player", "golang.org/x/vuln/cmd/govulncheck@v1.9.0"),
                         ["go", "-C", str(vuln_check.ROOT / "apps/player"), "run", "golang.org/x/vuln/cmd/govulncheck@v1.9.0", "-format", "json", "./..."])
        for directory, tool in (("../outside", "golang.org/x/vuln/cmd/govulncheck@v1.9.0"), ("apps/missing", "golang.org/x/vuln/cmd/govulncheck@v1.9.0"),
                                ("apps/player", "example.test/tool@latest"), ("apps/player", "golang.org/x/vuln/cmd/govulncheck@latest")):
            with self.subTest(directory=directory, tool=tool), self.assertRaises(ValueError):
                vuln_check.command(directory, tool)


if __name__ == "__main__":
    unittest.main()
