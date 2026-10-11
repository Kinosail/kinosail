#!/usr/bin/env python3
"""Fixed Playwright importer lookup with an intentional APP-only security cutoff."""
import json
import stat
from campaign_q47_execution import check_budget
from campaign_q47_io import read_bounded

VERSION = "1.63.0"
METADATA_LIMIT = 512 * 1024
MAX_ANCESTORS = 32
MAX_CANDIDATES = 16
IMPORTERS = {"@playwright/test": "app", "playwright": "@playwright/test",
             "playwright-core": "playwright"}


def metadata(root, name):
    check_budget()
    value = json.loads(read_bounded(root / "package.json", METADATA_LIMIT).decode("utf-8"))
    if (type(value) is not dict or value.get("name") != name
            or value.get("version") != VERSION):
        raise ValueError("dependency_version")


def container(path, boundary):
    check_budget()
    try:
        details = path.lstat()
    except FileNotFoundError:
        return None
    if not (stat.S_ISDIR(details.st_mode) or stat.S_ISLNK(details.st_mode)):
        raise ValueError("dependency_kind")
    # Existing broken/escaped links are failures; only missing lstat may continue.
    check_budget()
    selected = path.resolve(strict=True)
    if not selected.is_relative_to(boundary) or not selected.is_dir():
        raise ValueError("dependency_root")
    return selected


def select(modules, name, boundary):
    modules = container(modules, boundary)
    if modules is None:
        return None
    if name.startswith("@"):
        modules = container(modules / "@playwright", boundary)
        if modules is None:
            return None
    root = container(modules / name.split("/")[-1], boundary)
    if root is None:
        return None
    if root == boundary:
        raise ValueError("dependency_package_root")
    metadata(root, name)
    return root


def candidates(app, importer):
    # Mirrors Node 26 POSIX _nodeModulePaths, stopping at the declared APP.
    # No global folders, NODE_PATH, package exports or arbitrary package names.
    ancestor, steps, emitted = importer, 0, 0
    while True:
        check_budget()
        steps += 1
        if steps > MAX_ANCESTORS or not ancestor.is_relative_to(app):
            raise ValueError("dependency_ancestor_bound")
        if ancestor.name != "node_modules":
            emitted += 1
            if emitted > MAX_CANDIDATES:
                raise ValueError("dependency_candidate_bound")
            yield ancestor / "node_modules"
        if ancestor == app:
            return
        ancestor = ancestor.parent


def resolve_package(app, importer, name):
    check_budget()
    if type(name) is not str or name not in IMPORTERS:
        raise ValueError("dependency_name")
    if (not app.is_absolute() or ".." in app.parts or app.resolve(strict=True) != app
            or not app.is_dir()):
        raise ValueError("dependency_app")
    boundary = app / "node_modules"
    declared = container(boundary, boundary)
    if declared != boundary:
        raise ValueError("dependency_modules")
    expected = IMPORTERS[name]
    if expected == "app":
        if importer != app:
            raise ValueError("dependency_importer")
        search = (boundary,)
    else:
        if not importer.is_relative_to(boundary) or ".." in importer.parts:
            raise ValueError("dependency_importer")
        importer = container(importer, boundary)
        if importer is None:
            raise ValueError("dependency_importer")
        metadata(importer, expected)
        search = candidates(app, importer)
    for modules in search:
        check_budget()
        root = select(modules, name, boundary)
        if root is None:
            continue
        relative = root.relative_to(boundary)
        layout = ("flat" if relative.as_posix() == name else
                  "pnpm-isolated" if relative.parts[0] == ".pnpm"
                  and relative.parts[2:] == ("node_modules", *name.split("/")) else "nested")
        return {"root": root, "resolvedFrom": "playwright-test" if expected == "@playwright/test" else expected,
                "layout": layout}
    raise ValueError("dependency_missing")
