#!/usr/bin/env python3
"""Pure importer-lookup controls; all paths, metadata and operations are fictional."""
from contextlib import ExitStack
import json
from pathlib import PurePosixPath
import stat
from types import SimpleNamespace
import unittest
from unittest import mock
import campaign_q47_dependencies as diagnostics
import campaign_q47_package_resolution as resolver
import campaign_q47_sources as sources

NAMES = ("@playwright/test", "playwright", "playwright-core")
PIN = {"bytes": 1, "sha256": "a" * 64, "gitBlob": "b" * 40}


class FictionalPath(PurePosixPath):
    graph = None
    def resolve(self, strict=True): return self.graph.resolve(self, strict)
    def lstat(self): return self.graph.details(self)
    def is_dir(self): return self.graph.kind(self) == stat.S_IFDIR
    def is_file(self): return self.graph.kind(self) == stat.S_IFREG
    @classmethod
    def home(cls): return cls("/fictional/home")


class Graph:
    def __init__(self, isolated=False):
        self.app = FictionalPath("/fictional/e2e")
        self.entries, self.data, self.reads, self.lookups = {}, {}, [], []
        self.roots = {}
        for name in NAMES:
            label = name.replace("/", "+")
            root = self.app / "node_modules" / name
            if isolated:
                root = self.app / "node_modules/.pnpm" / (label + "@1.64.0") / "node_modules" / name
            self.package(root, name); self.roots[name] = root
        if isolated:
            self.link(self.app / "node_modules/@playwright/test", self.roots[NAMES[0]])
            self.link(self.roots[NAMES[0]].parent.parent / "playwright", self.roots[NAMES[1]])
            self.link(self.roots[NAMES[1]].parent / "playwright-core", self.roots[NAMES[2]])
        self.file(self.roots[NAMES[0]] / "cli.js", b"fictional")
        self.file(self.roots[NAMES[2]] / "browsers.json", json.dumps({"browsers": [
            {"name": "chromium", "revision": "1243"},
            {"name": "chromium-headless-shell", "revision": "1243"}]}).encode())
        for name, folder, exe in (("chromium", "chrome-linux64", "chrome"),
                                 ("chromium_headless_shell", "chrome-headless-shell-linux64", "chrome-headless-shell")):
            self.file(FictionalPath.home() / ".cache/ms-playwright" / (name + "-1243") / folder / exe, b"x")
    def directory(self, path):
        for parent in [path, *path.parents]:
            self.entries.setdefault(str(parent), (stat.S_IFDIR, None))
    def file(self, path, data):
        self.directory(path.parent)
        self.entries[str(path)] = (stat.S_IFREG, None); self.data[str(path)] = data
    def package(self, path, name, version="1.64.0"):
        self.directory(path)
        self.file(path / "package.json", json.dumps({"name": name, "version": version}).encode())
    def link(self, path, target):
        self.directory(path.parent); self.entries[str(path)] = (stat.S_IFLNK, target)
    def resolve(self, path, strict=True):
        path = FictionalPath(path)
        for _ in range(32):
            changed = False
            for prefix in reversed([path, *path.parents]):
                kind, target = self.entries.get(str(prefix), (None, None))
                if kind == stat.S_IFLNK:
                    path = FictionalPath(target) / path.relative_to(prefix); changed = True; break
            if not changed:
                if strict and str(path) not in self.entries: raise FileNotFoundError("fictional missing")
                return path
        raise ValueError("fictional link bound")
    def details(self, path):
        self.lookups.append(str(path))
        parent = self.resolve(path.parent)
        key = str(parent / path.name)
        if key not in self.entries: raise FileNotFoundError("fictional missing")
        return SimpleNamespace(st_mode=self.entries[key][0])
    def kind(self, path):
        try: return self.entries[str(self.resolve(path))][0]
        except FileNotFoundError: return None
    def read(self, path, limit):
        data = self.data[str(self.resolve(path))]
        self.reads.append((str(path), limit))
        if len(data) > limit: raise ValueError("fictional read bound")
        return data


def graph_context(graph):
    context = ExitStack()
    context.enter_context(mock.patch.object(FictionalPath, "graph", graph))
    context.enter_context(mock.patch.object(resolver, "read_bounded", side_effect=graph.read, create=True))
    context.enter_context(mock.patch.object(resolver, "check_budget", create=True))
    for name, value in (("APP", graph.app), ("Path", FictionalPath),
                        ("os", SimpleNamespace(environ={}, access=lambda *_: True, X_OK=1))):
        context.enter_context(mock.patch.object(sources, name, value))
    context.enter_context(mock.patch.object(sources, "tool_path", side_effect=lambda name: FictionalPath("/fictional/bin") / name))
    context.enter_context(mock.patch.object(sources, "fingerprint", return_value=PIN.copy()))
    context.enter_context(mock.patch.object(sources, "installed_tree", return_value={"files": 1, "bytes": 1, "sha256": "a" * 64}))
    context.enter_context(mock.patch.object(sources, "read_bounded", side_effect=graph.read))
    return context


class PackageResolutionControls(unittest.TestCase):
    def test_flat_inventory_preserves_all_tools_trees_and_caps(self):
        graph = Graph()
        with graph_context(graph):
            tools, cli, inventory = sources.dependencies()
        self.assertEqual(set(tools), {"go", "node", "ruby", "bundle"})
        self.assertEqual(cli, graph.roots[NAMES[0]] / "cli.js")
        self.assertEqual(len(inventory["tools"]), 7)
        for name, role in zip(NAMES, ("app", "playwright-test", "playwright")):
            self.assertEqual(inventory["packages"][name]["resolvedFrom"], role)
            self.assertEqual(inventory["packages"][name]["layout"], "flat")
            self.assertEqual(inventory["packages"][name]["files"], 1)
        self.assertEqual(diagnostics.current_stage(), "inspection-complete")
        self.assertTrue(all(cap == (262144 if path.endswith("browsers.json") else 524288)
                            for path, cap in graph.reads))

    def test_isolated_full_chain_binds_cli_registry_and_inventory(self):
        graph = Graph(True)
        self.assertNotIn(str(graph.app / "node_modules/playwright"), graph.entries)
        self.assertNotIn(str(graph.app / "node_modules/playwright-core"), graph.entries)
        with graph_context(graph):
            _, cli, inventory = sources.dependencies()
        self.assertEqual(cli, graph.roots[NAMES[0]] / "cli.js")
        self.assertIn((str(graph.roots[NAMES[2]] / "browsers.json"), 262144), graph.reads)
        self.assertEqual([inventory["packages"][name]["layout"] for name in NAMES], ["pnpm-isolated"] * 3)
        self.assertEqual([inventory["packages"][name]["resolvedFrom"] for name in NAMES],
                         ["app", "playwright-test", "playwright"])
        self.assertNotIn("/fictional", json.dumps(inventory))

    def test_nearest_nested_and_scoped_search_order_skips_node_modules(self):
        graph = Graph(True)
        root = graph.roots[NAMES[0]]
        nested = root / "node_modules/playwright"
        graph.package(nested, "playwright")
        with graph_context(graph):
            selected = resolver.resolve_package(graph.app, root, "playwright")
        self.assertEqual(selected["root"], nested)
        self.assertEqual(selected["layout"], "nested")
        graph.entries.pop(str(nested)); graph.entries.pop(str(nested / "package.json"))
        graph.lookups.clear()
        with graph_context(graph):
            selected = resolver.resolve_package(graph.app, root, "playwright")
        self.assertEqual(selected["root"], graph.roots[NAMES[1]])
        modules = [value for value in graph.lookups if value.endswith("/node_modules")]
        self.assertLess(modules.index(str(root / "node_modules")),
                        modules.index(str(root.parent / "node_modules")))
        self.assertNotIn("/node_modules/node_modules/", " ".join(graph.lookups))

    def test_existing_nearer_bad_package_is_never_bypassed(self):
        for defect in ("version", "name", "malformed", "shape", "oversize", "broken", "escape", "file", "fifo"):
            with self.subTest(defect=defect):
                graph = Graph()
                candidate = graph.roots[NAMES[0]] / "node_modules/playwright"
                graph.package(candidate, "playwright")
                if defect in ("version", "name"):
                    graph.package(candidate, "wrong" if defect == "name" else "playwright",
                                  "1.65.0" if defect == "version" else "1.64.0")
                elif defect == "malformed": graph.data[str(candidate / "package.json")] = b"{"
                elif defect == "shape": graph.data[str(candidate / "package.json")] = b"[]"
                elif defect == "oversize": graph.data[str(candidate / "package.json")] = b" " * 524289
                elif defect in ("broken", "escape"):
                    target = FictionalPath("/fictional/outside/playwright")
                    if defect == "escape": graph.package(target, "playwright")
                    graph.link(candidate, target)
                else: graph.entries[str(candidate)] = (stat.S_IFREG if defect == "file" else stat.S_IFIFO, None)
                with graph_context(graph), self.assertRaises((ValueError, FileNotFoundError)):
                    resolver.resolve_package(graph.app, graph.roots[NAMES[0]], "playwright")
                self.assertNotIn(str(graph.roots[NAMES[1]] / "package.json"),
                                 [path for path, _ in graph.reads])

    def test_existing_escaped_or_broken_container_is_not_bypassed(self):
        for component in ("node_modules", "node_modules/@playwright"):
            for broken in (False, True):
                graph = Graph()
                root = graph.app if component.endswith("@playwright") else graph.roots[NAMES[0]]
                name = "@playwright/test" if root == graph.app else "playwright"
                target = FictionalPath("/fictional/outside")
                if not broken: graph.directory(target)
                graph.link(root / component, target)
                with graph_context(graph), self.assertRaises((ValueError, FileNotFoundError)):
                    resolver.resolve_package(graph.app, root, name)

    def test_missing_never_uses_outside_global_package(self):
        graph = Graph(True)
        del graph.entries[str(graph.roots[NAMES[0]].parent.parent / "playwright")]
        graph.package(FictionalPath("/fictional/node_modules/playwright"), "playwright")
        with graph_context(graph), self.assertRaises(ValueError):
            resolver.resolve_package(graph.app, graph.roots[NAMES[0]], "playwright")
        self.assertFalse(any(path.startswith("/fictional/node_modules") for path in graph.lookups))

    def test_names_importers_and_declared_root_are_strict(self):
        for name in ("../playwright", "playwright/../../outside", "PLAYWRIGHT", None):
            graph = Graph()
            with graph_context(graph), self.assertRaises(ValueError):
                resolver.resolve_package(graph.app, graph.app, name)
            self.assertEqual(graph.reads, [])
        graph = Graph()
        with graph_context(graph):
            for importer, name in ((graph.app, "playwright"), (graph.roots[NAMES[1]], "playwright"),
                                   (FictionalPath("/fictional/outside"), "playwright"),
                                   (graph.roots[NAMES[0]], "@playwright/test")):
                with self.assertRaises(ValueError): resolver.resolve_package(graph.app, importer, name)

    def test_candidate_and_ancestor_bounds_budget_before_discovery(self):
        graph = Graph()
        deep = graph.app / "node_modules" / "/".join(["nested"] * 40) / "@playwright/test"
        graph.package(deep, "@playwright/test")
        with graph_context(graph), self.assertRaises(ValueError):
            resolver.resolve_package(graph.app, deep, "playwright")
        self.assertLessEqual(len([path for path in graph.lookups if path.endswith("/playwright")]), 16)
        graph = Graph()
        deep = graph.app / "node_modules" / "/".join(["node_modules"] * 40) / "@playwright/test"
        graph.package(deep, "@playwright/test")
        with graph_context(graph), self.assertRaises(ValueError):
            resolver.resolve_package(graph.app, deep, "playwright")
        self.assertLessEqual(len([path for path in graph.lookups if path.endswith("/playwright")]), 16)
        graph = Graph()
        with graph_context(graph), mock.patch.object(resolver, "check_budget", side_effect=ValueError("fictional budget")):
            with self.assertRaises(ValueError):
                resolver.resolve_package(graph.app, graph.app, "@playwright/test")
        self.assertEqual(graph.lookups, [])
        self.assertEqual(graph.reads, [])

    def test_importer_metadata_and_root_modules_escape_fail_closed(self):
        graph = Graph()
        graph.package(graph.roots[NAMES[0]], "@playwright/test", "1.62.0")
        with graph_context(graph), self.assertRaises(ValueError):
            resolver.resolve_package(graph.app, graph.roots[NAMES[0]], "playwright")
        graph = Graph()
        outside = FictionalPath("/fictional/outside")
        graph.directory(outside); graph.link(graph.app / "node_modules", outside)
        with graph_context(graph), self.assertRaises(ValueError):
            resolver.resolve_package(graph.app, graph.app, "@playwright/test")

    def test_cli_cannot_escape_its_validated_test_package(self):
        graph = Graph(True)
        target = graph.roots[NAMES[2]] / "foreign-cli.js"
        graph.file(target, b"fictional"); graph.link(graph.roots[NAMES[0]] / "cli.js", target)
        with graph_context(graph), self.assertRaises(ValueError):
            sources.dependencies()

    def test_package_link_to_modules_boundary_is_rejected_before_metadata(self):
        for name in ("@playwright/test", "playwright"):
            with self.subTest(name=name):
                graph = Graph()
                boundary = graph.app / "node_modules"
                graph.package(boundary, name)
                importer = graph.app if name == "@playwright/test" else graph.roots[NAMES[0]]
                candidate = boundary / name if name == "@playwright/test" else importer / "node_modules/playwright"
                graph.link(candidate, boundary)
                with graph_context(graph), self.assertRaises(ValueError):
                    resolver.resolve_package(graph.app, importer, name)
                self.assertNotIn(str(boundary / "package.json"), [path for path, _ in graph.reads])


if __name__ == "__main__":
    unittest.main()
