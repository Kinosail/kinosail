#!/usr/bin/env python3
"""Pure dependency-stage privacy/fault controls; all graph operations are fictional."""
from contextlib import ExitStack
import json
from pathlib import PurePosixPath
from types import SimpleNamespace
import unittest
from unittest import mock
import campaign_q47_dependencies as diagnostics
import campaign_q47_sources as sources

STAGES = [
    "go-resolution", "node-resolution", "ruby-resolution", "bundle-resolution",
    "go-fingerprint", "node-fingerprint", "ruby-fingerprint", "bundle-fingerprint",
    "playwright-test-metadata", "playwright-test-tree", "playwright-metadata", "playwright-tree",
    "playwright-core-metadata", "playwright-core-tree", "cli-resolution", "cli-fingerprint",
    "registry-read", "cache-resolution", "cache-validation", "chromium-registry-entry",
    "chromium-executable", "chromium-fingerprint", "headless-registry-entry",
    "headless-executable", "headless-fingerprint", "inspection-complete",
]
PIN = {"bytes": 1, "sha256": "a" * 64, "gitBlob": "b" * 40}


class FictionalPath(PurePosixPath):
    def resolve(self, strict=True): return self
    def is_file(self): return True
    def is_dir(self): return True
    @classmethod
    def home(cls): return cls("/fictional/home")


def graph():
    context = ExitStack()
    app = FictionalPath("/fictional/e2e")
    packages = ["@playwright/test", "playwright", "playwright-core"]
    data = {str(app / "node_modules" / name / "package.json"):
            json.dumps({"name": name, "version": "1.64.0"}).encode() for name in packages}
    data[str(app / "node_modules/playwright-core/browsers.json")] = json.dumps({
        "browsers": [{"name": "chromium", "revision": "1243"},
                     {"name": "chromium-headless-shell", "revision": "1243"}]}).encode()
    for name, value in (("APP", app), ("Path", FictionalPath),
                        ("os", SimpleNamespace(environ={}, access=lambda *_: True, X_OK=1))):
        context.enter_context(mock.patch.object(sources, name, value))
    context.enter_context(mock.patch.object(sources, "tool_path", side_effect=lambda name: FictionalPath("/fictional/bin") / name))
    context.enter_context(mock.patch.object(sources, "fingerprint", return_value=PIN.copy()))
    context.enter_context(mock.patch.object(sources, "installed_tree", return_value={"files": 1, "bytes": 1, "sha256": "a" * 64}))
    context.enter_context(mock.patch.object(sources, "read_bounded", side_effect=lambda path, _limit: data[str(path)]))
    context.enter_context(mock.patch.object(sources, "resolve_package", side_effect=lambda app, importer, name: {
        "root": app / "node_modules" / name, "layout": "flat",
        "resolvedFrom": "app" if name == "@playwright/test" else "playwright-test" if name == "playwright" else "playwright"}, create=True))
    return context


class DependencyStageControls(unittest.TestCase):
    def test_unknown_private_or_non_string_stage_cannot_be_exported(self):
        diagnostics.record_stage("go-resolution")
        self.assertEqual(diagnostics.current_stage(), "go-resolution")
        self.assertEqual(diagnostics.receipt_stage("dependency-check"), "go-resolution")
        for phase in ("docs-build", "final-provenance", "/fictional/private", None):
            self.assertIsNone(diagnostics.receipt_stage(phase))
        for value in (None, True, 1, {}, [], "/fictional/private", "fictional-secret", "GO-resolution"):
            with self.subTest(value=type(value).__name__):
                with self.assertRaises(ValueError): diagnostics.record_stage(value)
                self.assertIsNone(diagnostics.current_stage())
        diagnostics._current = "fictional-secret"
        self.assertIsNone(diagnostics.current_stage())

    def test_each_actual_dependency_stage_fault_exports_only_fixed_enum(self):
        original = getattr(diagnostics, "record_stage", lambda _stage: None)
        for target in STAGES:
            with self.subTest(stage=target), graph():
                def note(stage):
                    original(stage)
                    if stage == target: raise ValueError("fictional-secret /fictional/private")
                with mock.patch.object(diagnostics, "record_stage", side_effect=note, create=True):
                    with self.assertRaises(ValueError): sources.dependencies()
                stage = diagnostics.current_stage()
                self.assertEqual(stage, target)
                encoded = json.dumps({"dependencyStage": stage})
                self.assertNotIn("fictional-secret", encoded)
                self.assertNotIn("/fictional/private", encoded)

    def test_successful_fake_graph_preserves_order_and_existing_return_shape(self):
        visited = []
        original = getattr(diagnostics, "record_stage", lambda _stage: None)
        def note(stage):
            visited.append(stage); original(stage)
        with graph(), mock.patch.object(diagnostics, "record_stage", side_effect=note, create=True):
            tools, cli, inventory = sources.dependencies()
        self.assertEqual(visited, STAGES)
        self.assertEqual(set(tools), {"go", "node", "ruby", "bundle"})
        self.assertEqual(str(cli), "/fictional/e2e/node_modules/@playwright/test/cli.js")
        self.assertEqual(set(inventory), {"tools", "packages"})
        self.assertEqual(len(inventory["tools"]), 7)
        self.assertEqual(set(inventory["packages"]), {"@playwright/test", "playwright", "playwright-core"})
        self.assertEqual(diagnostics.current_stage(), "inspection-complete")

    def test_native_operation_failure_keeps_stage_without_exception_details(self):
        with graph(), mock.patch.object(sources, "tool_path", side_effect=OSError("fictional-secret /fictional/private")):
            with self.assertRaises(OSError): sources.dependencies()
        self.assertEqual(diagnostics.current_stage(), "go-resolution")
        self.assertNotIn("fictional", json.dumps({"dependencyStage": diagnostics.current_stage()}))


if __name__ == "__main__":
    unittest.main()
