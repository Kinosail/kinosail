"""Fixed BrowseReturn proof/caller controls; no real Go, app or browser runs."""
import copy
import importlib.util
import io
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[2]
SCRIPTS = ROOT / "apps/player/scripts"
sys.path.insert(0, str(SCRIPTS))
import campaign_q14_admission as admission
import campaign_q14_suites as suites

NAVIGATION = [("watch-navigation.spec.ts", f"Player has one accessible return link with Movies context and a direct-entry fallback at {width}px") for width in (390, 1440, 1920)] + [("watch-navigation.spec.ts", title) for title in ("Home keeps its exact return after Mark watched", "Plain root keeps its exact return after Mark watched")]


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace("-", "_"), SCRIPTS / (name + ".py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def report(mode, project="chromium"):
    pairs = NAVIGATION if mode == "navigation" else suites.SUITES[mode]
    href = "/watch/0123456789abcdef"
    state = {"path": "/", "values": {"view": "all"} if mode == "home" else {"view": "movies"}, "profile": "local-owner", "titles": ["Return Movie 25"], "hrefs": [href], "focused": href, "focusedBrowse": None, "scroll": {"x": 0, "y": 0}, "selected": None, "document": {"idSHA256": "a" * 64, "shows": [{"persisted": False}], "histories": 0}, "navigation": ["navigate"]}
    value = {"schemaVersion": 2, "suite": mode, "status": "passed", "errors": [], "collected": [{"file": file, "title": title} for file, title in pairs], "cases": []}
    for file, title in pairs:
        names = ({"movies-player-state", "direct-player-state"} if "accessible" in title else {"home-watched-return-state" if title.startswith("Home") else "root-watched-return-state"}) if mode == "navigation" else suites.required(mode, title)
        attachments = []
        for name in sorted(names):
            observation = {"state": copy.deepcopy(state), "peer": []}
            if mode == "navigation":
                observation["state"].update(path=href, values={}, titles=[], hrefs=[], focused=None)
                if name == "direct-player-state":
                    observation["state"]["document"]["idSHA256"] = "b" * 64
            if name == "served-browse-asset":
                observation = {"src": "/static/main.kinosail.bundle.js?v=35", "bytes": 1000, "sha256": "c" * 64}
            if name == "safe-rejection":
                observation = {"name": title.removeprefix("saved return rejects ").removesuffix(" before navigation or continuation"), "href": href, "noBrowseRequests": True}
            if name == "home-cold-boundary-state":
                observation["state"]["document"]["idSHA256"] = "b" * 64
                observation["state"]["navigation"] = ["back_forward"]
                observation["peer"] = [{"values": {}, "continuation": False, "history": False, "htmx": False}]
            attachments.append({"name": name, "bytes": 128, "sha256": "c" * 64, "observation": observation})
        value["cases"].append({"file": file, "title": title, "status": "passed", "retry": 0, "durationMs": 1, "expectedStatus": "passed", "failures": [], "attachments": attachments})
    value.update(schemaVersion=3, project=project)
    for row in [*value["collected"], *value["cases"]]:
        row["fullTitle"] = row["title"]
    return value


class NavigationProofTests(unittest.TestCase):
    def test_five_actual_navigation_identities_and_four_state_names_are_admitted(self):
        value = report("navigation")
        self.assertEqual(admission.admit(value, suite="navigation"), value)
        self.assertTrue(admission.complete(value, suite="navigation"))

    def test_missing_or_contradictory_navigation_states_cannot_certify(self):
        for mutate in (lambda v: v["cases"][0].update(attachments=[]), lambda v: v["cases"][0]["attachments"][0]["observation"]["state"].update(path="/")):
            value = report("navigation")
            mutate(value)
            self.assertFalse(admission.complete(value, suite="navigation"))

    def test_direct_page_observer_is_installed_before_its_original_navigation(self):
        source = (ROOT / "apps/player/e2e/watch-navigation.spec.ts").read_text()
        self.assertIn("await observe(direct);\n    await direct.goto", source)


class ProofCaptureTests(unittest.TestCase):
    def setUp(self):
        self.driver = load("campaign-q14-public")

    def capture(self, raw):
        process = mock.Mock(pid=123456789, stdout=io.BytesIO(raw))
        process.poll.return_value = process.wait.return_value = 0
        with mock.patch.object(self.driver.subprocess, "Popen", return_value=process), mock.patch.object(self.driver, "settle_group", return_value=True), mock.patch.object(self.driver, "group_signal"):
            return self.driver.run(["inert"], ROOT, {}, 3, "journeys", 2, "primary")

    def test_escaped_duplicate_json_fields_and_nonfinite_json_are_rejected(self):
        text = json.dumps(report("primary"))
        for raw in (text.replace('"suite": "primary"', '"suite": "wrong", "\\u0073uite": "primary"'), text.replace('"durationMs": 1', '"durationMs": NaN')):
            phase, accepted = self.capture(b"Q14_PROOF_RESULT " + raw.encode() + b"\n")
            self.assertIsNone(accepted)
            self.assertFalse(phase["reportAdmitted"])


    def test_missing_duplicate_unknown_malformed_and_oversized_markers_never_certify(self):
        marker = b"Q14_PROOF_RESULT " + json.dumps(report("primary")).encode() + b"\n"
        for raw in (b"", marker + marker, b"Q14_OTHER {}\n", b"Q14_PROOF_RESULT {\n",
                    b"Q14_PROOF_RESULT " + b"x" * (2 * 1024 * 1024 + 1) + b"\n"):
            phase, value = self.capture(raw)
            self.assertIsNone(value)
            self.assertFalse(phase["reportAdmitted"])


class DeclaredContextAdmissionTests(unittest.TestCase):
    def collection(self):
        contexts = {
            'cold native Back restores later Movie cards at 390px': 'native Back without browser cache',
            'cold native Back restores later Movie cards at 1440px': 'native Back without browser cache',
            "live query uses current URL rather than the document's initial browse key": 'live HTMX search then cold playback Back',
            'native BFCache preserves loaded Movie DOM without repeated continuation': 'observed native browser cache',
        }
        rows = [{"file": file, "title": title, "fullTitle": contexts[title] + " > " + title if title in contexts else title}
                for file, title in suites.SUITES['all']]
        return {"schemaVersion": 3, "project": "chromium", "suite": "all", "status": "passed", "collected": rows, "cases": [], "errors": []}

    def test_exact_declared_nine_case_collection_is_admitted(self):
        value = self.collection()
        self.assertEqual(admission.admit(value, True), value)
        self.assertTrue(admission.complete(value, True))
        self.assertEqual({row['file'] for row in value['collected']}, {'browse-return.spec.ts', 'browse-return-cold.spec.ts', 'browse-return-bfcache.spec.ts'})

    def test_context_and_collection_ambiguity_reject_before_completion(self):
        mutations = [lambda v: v['collected'][-1].update(fullTitle=v['collected'][-1]['title']),
                     lambda v: v['collected'][-1].update(fullTitle='unknown > '+v['collected'][-1]['title']),
                     lambda v: v['collected'][-1].update(fullTitle='observed native browser cache > extra > '+v['collected'][-1]['title']),
                     lambda v: v['collected'][-1].update(file='browse-return-cold.spec.ts'),
                     lambda v: v['collected'].__setitem__(0,v['collected'][-1]),
                     lambda v: v['collected'].append(v['collected'][-1]),
                     lambda v: v['collected'].pop(), lambda v: v.update(project='unknown')]
        for mutate in mutations:
            value=self.collection(); mutate(value)
            self.assertIsNone(admission.admit(value, True))
            self.assertFalse(admission.complete(value, True))


class BrowseReturnCallerTests(unittest.TestCase):
    def setUp(self):
        self.owner = load("run-browse-return")
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name).resolve()
        self.media = self.directory / "Checkpoint Example.mp4"
        self.media.write_bytes(b"inert-fixture-control")
        self.output = self.directory / "proof"

    def invoke(self, args=None, phase=None, value=None):
        phase = phase if phase is not None else {"exitCode": 0, "ownedGroupStopped": True, "captureSettled": True, "outputOverflow": False, "goBoundary": "completed-pass", "reportAdmitted": True}
        value = report("primary") if value is None else value
        with mock.patch.object(self.owner.driver, "run", return_value=(phase, value)) as run:
            status = self.owner.main(args if args is not None else ["chromium", "primary", str(self.media), str(self.output)])
        return status, run

    def test_each_mode_uses_exact_go_owner_and_separate_receipt(self):
        for project in ("chromium", "firefox", "webkit"):
            for mode, count in (("primary", 2), ("navigation", 5), ("safety", 10), ("home", 3)):
                with self.subTest(project=project, mode=mode):
                    output = self.directory / (project + "-" + mode)
                    status, run = self.invoke([project, mode, str(self.media), str(output)], value=report(mode, project))
                    self.assertEqual(status, 0)
                    call = run.call_args.args
                    self.assertIn("^TestBrowseReturnBrowserJourney$", call[0])
                    self.assertIn("-count=1", call[0])
                    self.assertEqual(call[2]["KINOSAIL_BROWSE_RETURN_CASES"], mode)
                    self.assertEqual(call[2]["KINOSAIL_BROWSER_PROJECT"], project)
                    self.assertEqual(call[2]["KINOSAIL_BROWSE_RETURN_PROOF"], "1")
                    receipt = json.loads((output / "receipt.json").read_text())
                    self.assertEqual(receipt["passedCases"], count)
                    self.assertEqual(receipt["project"], project)
                    self.assertEqual(receipt["media"]["bytes"], self.media.stat().st_size)

    def test_missing_project_and_full_title_cannot_certify_another_engine(self):
        status, _ = self.invoke(["webkit", "primary", str(self.media), str(self.output)], value=report("primary"))
        self.assertEqual(status, 1)
        self.assertFalse((self.output / "proof.json").exists())

    def test_invalid_arguments_reject_before_output_or_child_effects(self):
        arguments = ["chromium", "primary", str(self.media), str(self.output)]
        bad = [[], arguments + ["extra"], ["chromium"], ["unknown", *arguments[1:]], [arguments[0], "all", *arguments[2:]], [arguments[0], "navigation\nprimary", *arguments[2:]], ["x" * 4097, *arguments[1:]], [*arguments[:2], str(self.directory / "missing.mp4"), arguments[3]], [*arguments[:3], str(self.directory / ".." / "escaping")], [*arguments[:3], str(self.directory / "same/../escaping")]]
        for args in bad:
            with self.subTest(args=args):
                status, run = self.invoke(args)
                self.assertEqual(status, 2)
                run.assert_not_called()
                self.assertFalse(self.output.exists())

    def test_linked_empty_oversized_media_and_linked_or_existing_output_reject(self):
        for kind in ("linked media", "empty", "oversized", "linked output", "existing output", "linked parent"):
            with self.subTest(kind=kind):
                media, output = self.directory / (kind + ".mp4"), self.directory / (kind + "-proof")
                media.write_bytes(b"inert")
                if kind == "linked media":
                    media.unlink(); media.symlink_to(self.media)
                elif kind == "empty":
                    media.write_bytes(b"")
                elif kind == "oversized":
                    with media.open("wb") as stream: stream.truncate(8 * 1024 * 1024 + 1)
                elif kind == "linked output":
                    output.symlink_to(self.directory)
                elif kind == "existing output":
                    output.mkdir()
                elif kind == "linked parent":
                    linked = self.directory / "parent-link"; linked.symlink_to(self.directory)
                    output = linked / "child"
                before = sorted(item.name for item in self.directory.iterdir())
                status, run = self.invoke(["chromium", "primary", str(media), str(output)])
                self.assertEqual(status, 2); run.assert_not_called()
                self.assertEqual(sorted(item.name for item in self.directory.iterdir()), before)

    def test_missing_duplicate_extra_skipped_retried_unknown_and_malformed_proof_fail(self):
        mutations = [lambda v: v.update(cases=[]), lambda v: v["cases"].append(v["cases"][0]), lambda v: v["collected"].append(v["collected"][0]), lambda v: v["cases"][0].update(status="skipped"), lambda v: v["cases"][0].update(retry=1), lambda v: v["cases"][0].update(title="unknown"), lambda v: v.update(token="private"), lambda v: v["cases"][0].update(attachments=[]), lambda v: v.update(errors=[{"phase": "unclassified", "label": None, "location": None}])]
        for index, mutate in enumerate(mutations):
            value = report("primary"); mutate(value)
            output = self.directory / str(index)
            status, _ = self.invoke(["chromium", "primary", str(self.media), str(output)], value=value)
            self.assertEqual(status, 1)
            receipt = json.loads((output / "receipt.json").read_text())
            self.assertNotIn("private", json.dumps(receipt))
            self.assertFalse((output / "proof.json").exists())

    def test_exact_file_full_title_project_and_first_attempt_result_are_required(self):
        mutations = [lambda v: v.pop("project"), lambda v: v.update(project="unknown"),
                     lambda v: v.update(project="x" * 4097), lambda v: v.update(project="firefox"),
                     lambda v: v["collected"][0].pop("fullTitle"),
                     lambda v: v["collected"][0].update(fullTitle="other suite > " + v["collected"][0]["title"]),
                     lambda v: v["cases"][0].update(fullTitle="nested > " + v["cases"][0]["title"]),
                     lambda v: v["cases"][0].update(file="watch-navigation.spec.ts"),
                     lambda v: v["cases"][0].update(status="failed"),
                     lambda v: v["cases"][0].update(expectedStatus="failed")]
        for index, mutate in enumerate(mutations):
            value = report("primary"); mutate(value)
            output = self.directory / ("identity-" + str(index))
            status, _ = self.invoke(["chromium", "primary", str(self.media), str(output)], value=value)
            self.assertEqual(status, 1)
            self.assertFalse((output / "proof.json").exists())

    def test_failed_child_or_unjoined_overflow_or_incomplete_go_never_certifies(self):
        for index, mutation in enumerate(({"exitCode": 1}, {"ownedGroupStopped": False}, {"captureSettled": False}, {"outputOverflow": True}, {"goBoundary": "incomplete"}, {"reportAdmitted": False})):
            phase = {"exitCode": 0, "ownedGroupStopped": True, "captureSettled": True, "outputOverflow": False, "goBoundary": "completed-pass", "reportAdmitted": True}; phase.update(mutation)
            status, _ = self.invoke(["chromium", "primary", str(self.media), str(self.directory / str(index))], phase=phase)
            self.assertEqual(status, 1)

    def test_non_regular_fifo_media_does_not_block_or_start_child(self):
        fifo = self.directory / "media.fifo"
        os.mkfifo(fifo)
        # A real isolated Python CLI, not Go: opening a FIFO must not hang before
        # input admission. A 2s control bound preserves the failure distinctly.
        import subprocess
        result = subprocess.run([sys.executable, "-B", str(SCRIPTS / "run-browse-return.py"),
                                 "chromium", "primary", str(fifo), str(self.output)],
                                capture_output=True, timeout=2)
        self.assertEqual(result.returncode, 2)
        self.assertFalse(self.output.exists())

    def test_container_owner_wires_all_modes_beside_existing_lanes(self):
        wrapper = (ROOT / "apps/player/scripts/test-browser-journeys.sh").read_text()
        container = (ROOT / "apps/player/scripts/test-container.sh").read_text()
        self.assertIn("for mode in primary navigation safety home", wrapper)
        self.assertIn('run_browse_return_journeys "$project"', container)
        self.assertIn('run_native_intent_regression "$project"', container)
        self.assertIn('run_populated_player_journeys "$project"', container)


if __name__ == "__main__":
    unittest.main()
