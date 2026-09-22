"""Selection invariants and real Git history regression tests."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

from affected import APPS, FLAGS, affected, changed_paths, main


class SelectionTests(unittest.TestCase):
    def test_app_isolation_and_complete_suite_selection(self):
        for app in APPS:
            with self.subTest(app=app):
                plan = affected([f"apps/{app}/internal/configuration/settings.go"])
                self.assertEqual([name for name in APPS if plan[name]], [app])
                self.assertTrue(plan["go"])
                self.assertFalse(plan["client"])
                self.assertFalse(plan["tooling"])
                self.assertFalse(plan["packages"])
                self.assertFalse(plan[f"{app}_browsers"])

    def test_linter_dependencies_select_tooling_security_and_web_without_building_products(self):
        for path in ("scripts/quality/package.json", "scripts/quality/pnpm-lock.yaml"):
            plan = affected([path])
            self.assertEqual({key for key, value in plan.items() if value},
                             {"tooling", "web", "supply", "javascript-typescript"})
        self.assertTrue(affected(["scripts/quality/new-build-input"])["swift"])

    def test_ci_test_and_swift_build_changes_select_their_actual_consumers(self):
        self.assertEqual({key for key, value in affected(["scripts/ci/test_delivery.py"]).items() if value},
                         {"tooling", "python"})
        self.assertEqual({key for key, value in affected(["scripts/ci/build-codeql-swift.sh"]).items() if value},
                         {"tooling", "client", "swift"})
        self.assertTrue(all(affected(["scripts/ci/new-build.sh"]).values()))
        self.assertTrue(all(affected(["scripts/ci/test_affected.py", "scripts/ci/affected.py"]).values()))

    def test_shared_changes_select_every_consumer(self):
        for path in ("packages/catalog/item.go", "go.work", "go.work.sum", "apps/player/go.mod"):
            with self.subTest(path=path):
                plan = affected([path])
                self.assertTrue(all(plan[app] for app in APPS))
                self.assertTrue(plan["packages"])

    def test_ui_and_embedded_assets_select_browser_matrix(self):
        for path in ("apps/player/internal/server/home.go", "apps/player/e2e/test.spec.ts",
                     "apps/player/internal/server/static/icon.svg", "packages/webassets/assets.go"):
            with self.subTest(path=path):
                plan = affected([path])
                self.assertTrue(plan["player_browsers"])
                self.assertTrue(plan["web"])
                self.assertTrue(plan["javascript-typescript"])

    def test_native_only_does_not_build_server_containers(self):
        plan = affected(["apps/player/apps/native/Sources/App.swift"])
        self.assertTrue(plan["client"])
        self.assertTrue(plan["swift"])
        self.assertFalse(any(plan[app] for app in APPS))
        self.assertTrue(affected(["apps/player/Makefile"])["client"])

    def test_prose_exemptions_do_not_include_shipped_licenses_or_embedded_markdown(self):
        paths = ["README.md", "AGENTS.md", "engineering/research/ci.md", "apps/player/docs/install.md"]
        self.assertFalse(any(affected(paths).values()))
        for path in ("apps/player/LICENSING.md", "apps/player/THIRD_PARTY_NOTICES.md",
                     "apps/player/internal/server/static/help.md"):
            with self.subTest(path=path):
                self.assertTrue(affected([path])["player"])

    def test_build_inputs_and_unknown_paths_are_conservative(self):
        for path in (".github/workflows/quality.yml", "scripts/ci/affected.py", "new-service/main.go", ".dockerignore"):
            self.assertTrue(all(affected([path]).values()))
        for path in ("apps/subtitles/Containerfile", "apps/subtitles/scripts/install.sh"):
            plan = affected([path])
            self.assertTrue(plan["subtitles_arm"])
            self.assertTrue(plan["subtitles_tools"])
            self.assertTrue(plan["supply"])

    def test_invalid_paths_are_rejected_even_after_unknown_paths(self):
        for paths in ("foo", [""], [None], ["/etc/passwd"], ["a/../b"], ["a//b"], ["a/./b"],
                      ["x" * 4097], ["a\0b"], ["README.md"] * 100001, ["unknown", "../bad"]):
            with self.subTest(paths=str(paths)[:60]), self.assertRaises(ValueError):
                affected(paths)

    def test_full_runs_and_zero_base(self):
        for name in ("schedule", "workflow_dispatch", "workflow_call"):
            self.assertIsNone(changed_paths(name, {}, "a" * 40))
        self.assertIsNone(changed_paths("push", {"before": "0" * 40}, "a" * 40))
        self.assertTrue(all(affected(None).values()))

    def test_invalid_event_never_invokes_git(self):
        for name, event, head in (("unknown", {}, "a" * 40), ("push", {"before": "-x"}, "a" * 40),
                                  ("push", {"before": "a" * 40}, "bad"), ("push", [], "a" * 40)):
            with self.subTest(name=name, event=event), patch("affected.subprocess.run") as run:
                with self.assertRaises(ValueError):
                    changed_paths(name, event, head)
                run.assert_not_called()

    def test_missing_git_base_fails_instead_of_emitting_empty_plan(self):
        with patch("affected.subprocess.run", side_effect=subprocess.CalledProcessError(128, "git")):
            with self.assertRaises(subprocess.CalledProcessError):
                changed_paths("push", {"before": "a" * 40}, "b" * 40)

    def test_selection_emits_swift_separately_from_linux_language_matrix(self):
        for paths, expected in ((["apps/player/apps/native/Sources/App.swift"], []),
                                (["scripts/quality/pnpm-lock.yaml"], ["javascript-typescript"]),
                                (None, ["go", "javascript-typescript", "python", "actions"])):
            with self.subTest(paths=paths), tempfile.TemporaryDirectory() as directory:
                event = Path(directory) / "event.json"
                output = Path(directory) / "output"
                event.write_text("{}")
                env = {"GITHUB_EVENT_PATH": str(event), "GITHUB_OUTPUT": str(output),
                       "GITHUB_STEP_SUMMARY": "", "GITHUB_EVENT_NAME": "pull_request",
                       "GITHUB_SHA": "a" * 40, "CI_FULL": "false"}
                with patch.dict(os.environ, env), patch("affected.changed_paths", return_value=paths):
                    main()
                values = dict(line.split("=", 1) for line in output.read_text().splitlines())
                self.assertEqual(json.loads(values["languages"]), expected)
                self.assertEqual(json.loads(values["plan"])["swift"], paths is None or "native" in paths[0])

    def test_rejected_cli_event_has_no_output_side_effects(self):
        with tempfile.TemporaryDirectory() as directory:
            event = Path(directory) / "event.json"
            output = Path(directory) / "output"
            summary = Path(directory) / "summary"
            event.write_text('{"before":"invalid"}')
            env = {"GITHUB_EVENT_PATH": str(event), "GITHUB_OUTPUT": str(output),
                   "GITHUB_STEP_SUMMARY": str(summary), "GITHUB_EVENT_NAME": "push", "GITHUB_SHA": "a" * 40}
            with patch.dict(os.environ, env):
                for full in ("false", "bad"):
                    with patch.dict(os.environ, {"CI_FULL": full}), self.assertRaises(ValueError):
                        main()
                    self.assertFalse(output.exists())
                    self.assertFalse(summary.exists())
                event.write_text(" " * (16 * 1024 * 1024 + 1))
                with self.assertRaises(ValueError):
                    main()
                self.assertFalse(output.exists())


class GitDiffTests(unittest.TestCase):
    def test_push_pr_merge_group_renames_deletions_and_large_changes(self):
        with tempfile.TemporaryDirectory() as directory:
            repo = Path(directory)
            def git(*args):
                return subprocess.check_output(["git", "-C", directory, *args], stderr=subprocess.DEVNULL).decode().strip()
            git("init", "-b", "main")
            git("config", "user.email", "ci@example.invalid")
            git("config", "user.name", "CI Fixture")
            (repo / "apps/player").mkdir(parents=True)
            (repo / "apps/player/old.go").write_text("old")
            git("add", ".")
            git("commit", "-m", "base")
            base = git("rev-parse", "HEAD")
            (repo / "apps/dashboard").mkdir(parents=True)
            git("mv", "apps/player/old.go", "apps/dashboard/new.go")
            (repo / "docs").mkdir()
            for index in range(350):
                (repo / f"docs/{index}.md").write_text("prose")
            (repo / "apps/subtitles").mkdir(parents=True)
            (repo / "apps/subtitles/with\nnewline.go").write_text("changed")
            git("add", ".")
            git("commit", "-m", "rename and many changes")
            head = git("rev-parse", "HEAD")
            actual_run = subprocess.run
            def in_repo(*args, **kwargs):
                return actual_run(*args, cwd=directory, **kwargs)
            with patch("affected.subprocess.run", side_effect=in_repo):
                for event_name, event in (("push", {"before": base}),
                    ("pull_request", {"pull_request": {"base": {"sha": base}, "head": {"sha": head}}}),
                    ("merge_group", {"merge_group": {"base_sha": base, "head_sha": head}})):
                    paths = changed_paths(event_name, event, head)
                    self.assertEqual(len(paths), 353)
                    self.assertIn("apps/player/old.go", paths)
                    self.assertIn("apps/dashboard/new.go", paths)
                    self.assertIn("apps/subtitles/with\nnewline.go", paths)
                    self.assertTrue(all(affected(paths)[app] for app in APPS))
            git("rm", "apps/dashboard/new.go")
            git("commit", "-m", "delete")
            deleted_head = git("rev-parse", "HEAD")
            with patch("affected.subprocess.run", side_effect=in_repo):
                self.assertEqual(changed_paths("push", {"before": head}, deleted_head), ["apps/dashboard/new.go"])

    def test_pr_uses_merge_base_so_unrelated_base_changes_do_not_select_apps(self):
        with tempfile.TemporaryDirectory() as directory:
            def git(*args):
                return subprocess.check_output(["git", "-C", directory, *args], stderr=subprocess.DEVNULL).decode().strip()
            git("init", "-b", "main")
            git("config", "user.email", "ci@example.invalid")
            git("config", "user.name", "CI")
            git("commit", "--allow-empty", "-m", "base")
            git("checkout", "-b", "feature")
            Path(directory, "README.md").write_text("docs")
            git("add", "."); git("commit", "-m", "docs")
            head = git("rev-parse", "HEAD")
            git("checkout", "main")
            Path(directory, "global.go").write_text("code")
            git("add", "."); git("commit", "-m", "base advanced")
            base = git("rev-parse", "HEAD")
            real_run = subprocess.run
            with patch("affected.subprocess.run", side_effect=lambda *a, **kw: real_run(*a, cwd=directory, **kw)):
                event = {"pull_request": {"base": {"sha": base}, "head": {"sha": head}}}
                self.assertFalse(any(affected(changed_paths("pull_request", event, head)).values()))


if __name__ == "__main__":
    unittest.main()
