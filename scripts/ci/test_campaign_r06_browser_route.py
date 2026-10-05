"""Guard fixed R06 browser dispatch without files or application execution."""
import io
import os
from pathlib import Path
import shlex
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[2]


def router_source():
    return (ROOT / "scripts/ci/run-campaign-proof.sh").read_text()


def probe_router(source, proof, suite=None, extra=()):
    # Override exec in this isolated shell: no selected driver is launched.
    shell = 'exec() { printf "%s\\n" "$@"; }\nset -- '
    shell += " ".join(shlex.quote(value) for value in (proof, *extra))
    shell += "\nsource /dev/stdin\n"
    environment = {"PATH": "/usr/bin:/bin", "LC_ALL": "C"}
    if suite is not None:
        environment["CAMPAIGN_R06_SUITE"] = suite
    return subprocess.run(["bash", "--noprofile", "--norc", "-c", shell],
                          input=source, env=environment, capture_output=True,
                          text=True, timeout=3)


class R06BrowserRouteTests(unittest.TestCase):
    def assert_dispatch(self, proof, suite, path):
        result = probe_router(router_source(), proof, suite)
        self.assertEqual(result.returncode, 0)
        self.assertEqual(result.stdout.splitlines(), ["python3", path])

    def assert_rejected(self, proof, suite, extra=()):
        result = probe_router(router_source(), proof, suite, extra)
        self.assertEqual(result.returncode, 2)
        self.assertEqual(result.stdout, "")

    def test_absent_and_explicit_protocol_preserve_incumbent(self):
        for suite in (None, "protocol"):
            with self.subTest(suite=suite):
                self.assert_dispatch("R06", suite,
                                     "apps/subtitles/scripts/campaign-r06-public.py")

    def test_only_fixed_browser_suites_select_browser_driver(self):
        for suite in ("save-headers", "save-body"):
            with self.subTest(suite=suite):
                self.assert_dispatch("R06", suite,
                                     "apps/subtitles/scripts/campaign-r06-browser.py")

    def test_source_format_selects_only_the_fixed_formatter(self):
        self.assert_dispatch("R06", "source-format",
                             "apps/subtitles/scripts/campaign_r06_format.py")

    def test_unknown_suite_never_executes_a_driver(self):
        for suite in ("", "all", "browser", "save-state", "../save-body", "save-body;touch bad",
                      "save-body\nprotocol", "SAVE-BODY"):
            with self.subTest(suite=suite):
                self.assert_rejected("R06", suite)

    def test_other_proofs_require_protocol_default(self):
        for proof in ("Q14", "Q09"):
            for suite in ("save-headers", "save-body", "source-format"):
                with self.subTest(proof=proof, suite=suite):
                    self.assert_rejected(proof, suite)

    def test_other_default_routes_are_unchanged(self):
        for proof in ("Q14", "Q09"):
            self.assert_dispatch(proof, None,
                                 f"apps/player/scripts/campaign-{proof.lower()}-public.py")

    def test_unknown_proof_or_extra_argument_never_executes(self):
        for proof in ("", "none", "Q47", "R06;touch bad", "../R06"):
            self.assert_rejected(proof, "save-headers")
        self.assert_rejected("R06", "save-headers", ("unexpected",))


if __name__ == "__main__":
    unittest.main()
