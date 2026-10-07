"""Guard fixed R06 browser dispatch without files or application execution."""
import io
import os
from pathlib import Path
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[2]


def router_source():
    return (ROOT / "scripts/ci/run-campaign-proof.sh").read_text()


def probe_router(source, proof, suite=None, extra=()):
    # Override exec in this isolated shell: no selected driver is launched.
    shell = 'exec() { printf "%s\\n" "$@"; }\n'
    environment = {"PATH": "/usr/bin:/bin", "LC_ALL": "C"}
    if suite is not None:
        environment["CAMPAIGN_R06_SUITE"] = suite
    return subprocess.run(["bash", "--noprofile", "--norc", "-s", "--", proof, *extra],
                          input=shell + source, env=environment, capture_output=True,
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

    def test_delayed_stdin_rejection_consumes_the_router(self):
        # Keep the writer open before router delivery: an empty source must
        # neither complete the probe nor leave an owned shell behind.
        import select
        import time
        from unittest.mock import patch

        def delayed_run(arguments, *, input, env, capture_output, text, timeout):
            deadline = time.monotonic() + timeout
            process = subprocess.Popen(arguments, env=env, stdin=subprocess.PIPE,
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=text)
            try:
                try:
                    process.stdin.write('printf "input-ready\\n" >&2\n')
                    process.stdin.flush()
                except BrokenPipeError:
                    self.fail("router probe exited before readiness input")
                ready, _, _ = select.select([process.stderr], [], [],
                                             max(0, deadline - time.monotonic()))
                self.assertTrue(ready, "router probe did not acknowledge readiness input")
                self.assertEqual(os.read(process.stderr.fileno(), 4096), b"input-ready\n")
                remaining = deadline - time.monotonic()
                self.assertGreater(remaining, 0, "readiness exhausted the probe timeout")
                with self.assertRaises(subprocess.TimeoutExpired,
                                       msg="router probe exited while its stdin writer remained open"):
                    process.wait(timeout=min(0.1, remaining))
                stdout, stderr = process.communicate(input, timeout=max(0, deadline - time.monotonic()))
                return subprocess.CompletedProcess(arguments, process.returncode, stdout, stderr)
            finally:
                try:
                    if process.poll() is None:
                        process.kill()
                    process.wait(timeout=timeout)
                finally:
                    for pipe in (process.stdin, process.stdout, process.stderr):
                        try:
                            pipe.close()
                        except BrokenPipeError:
                            pass

        with patch.object(subprocess, "run", side_effect=delayed_run):
            self.assert_rejected("Q09", "source-format")

    def test_absent_and_explicit_protocol_preserve_incumbent(self):
        for suite in (None, "protocol"):
            with self.subTest(suite=suite):
                self.assert_dispatch("R06", suite,
                                     "apps/subtitles/scripts/campaign-r06-public.py")

    def test_only_fixed_browser_suites_select_browser_driver(self):
        for suite in ("save-controls", "save-headers", "save-body"):
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
            for suite in ("save-controls", "save-headers", "save-body", "source-format"):
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
