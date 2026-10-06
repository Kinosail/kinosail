"""Actual selected executable and transitive Playwright importer closure."""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import time
from campaign_r06_restore_runtime_sources import ROOT, APP, read_bytes, pin, strict_json
from campaign_r06_restore_runtime_process import execute, accepted

LIMIT = 512 * 1024 * 1024
PACKAGES = ("@playwright/test", "playwright", "playwright-core")
RESOLVE = ("const fs=require('node:fs'),p=require('node:path'),m=require('node:module');"
           "const r=m.createRequire(process.cwd()+'/package.json');"
           "const testManifest=fs.realpathSync(r.resolve('@playwright/test/package.json'));"
           "const tr=m.createRequire(testManifest);"
           "const playwrightManifest=fs.realpathSync(tr.resolve('playwright/package.json'));"
           "const pr=m.createRequire(playwrightManifest);"
           "const coreManifest=fs.realpathSync(pr.resolve('playwright-core/package.json'));"
           "const manifests=[testManifest,playwrightManifest,coreManifest];"
           "process.stdout.write(JSON.stringify(Object.fromEntries(manifests.map(f=>{"
           "const v=JSON.parse(fs.readFileSync(f,'utf8'));return[v.name,{manifest:f,root:p.dirname(f),package:v}]})))+'\\n')")


def package_roots(value, repository):
    base = Path(repository)
    if not base.is_absolute() or type(value) is not dict or set(value) != set(PACKAGES):
        raise ValueError("package-resolution")
    roots = {}
    for name, row in value.items():
        if type(row) is not dict or set(row) != {"manifest", "root", "package"}: raise ValueError("package-candidate")
        if any(type(row[key]) is not str or "\0" in row[key] or not Path(row[key]).is_absolute()
               or ".." in Path(row[key]).parts for key in ("root", "manifest")): raise ValueError("package-path")
        root = Path(row["root"])
        if not root.is_relative_to(base) or Path(row["manifest"]) != root / "package.json": raise ValueError("package-path")
        package = row["package"]
        if type(package) is not dict or package.get("name") != name or package.get("version") != "1.63.0":
            raise ValueError("package-version")
        roots[name] = root
    return roots


def selected_cli(roots, node, arguments):
    if set(roots) != set(PACKAGES) or type(arguments) is not list or any(type(x) is not str for x in arguments):
        raise ValueError("selected-cli")
    return [str(node), str(roots["@playwright/test"] / "cli.js"), *arguments]


def registry_script(core):
    target = str(Path(core) / "lib/coreBundle.js")
    return ("const registry=require(" + json.dumps(target) + ").registry;"
            "process.stdout.write(JSON.stringify({chromium:registry.findExecutable('chromium').executablePath(),"
            "headless:registry.findExecutable('chromium-headless-shell').executablePath()})+'\\n')")


def browser_paths(registry, cache):
    if (type(registry) is not dict or type(registry.get("browsers")) is not list or
            type(cache) is not str or not Path(cache).is_absolute() or ".." in Path(cache).parts or "\0" in cache):
        raise ValueError("browser-registry")
    result = {}
    for name, directory, executable in (
            ("chromium", "chromium", "chrome-linux64/chrome"),
            ("chromium-headless-shell", "chromium_headless_shell", "chrome-headless-shell-linux64/chrome-headless-shell")):
        matches = [row for row in registry["browsers"] if type(row) is dict and row.get("name") == name]
        if len(matches) != 1: raise ValueError("browser-registry")
        revision = matches[0].get("revision")
        if type(revision) is not str or not re.fullmatch("[0-9]{1,12}", revision): raise ValueError("browser-revision")
        key = "chromium" if name == "chromium" else "headless"
        result[key] = str(Path(cache) / (directory + "-" + revision) / executable)
    return result


def valid_tools(value, suite="restore-headers"):
    if suite not in ("restore-controls", "restore-headers", "restore-inspect-body"): return False
    expected = {"go"} if suite == "restore-controls" else {"go", "node", "pnpm", "chromium", "headless"}
    if type(value) is not dict or set(value) != expected: return False
    for record in value.values():
        if type(record) is not dict or set(record) != {"bytes", "sha256", "gitBlob", "mode"}: return False
        if (type(record["bytes"]) is not int or not 0 < record["bytes"] <= LIMIT or
                type(record["mode"]) is not int or not 0 <= record["mode"] <= 0o777 or not record["mode"] & 0o111 or
                type(record["sha256"]) is not str or not re.fullmatch("[a-f0-9]{64}", record["sha256"]) or
                type(record["gitBlob"]) is not str or not re.fullmatch("[a-f0-9]{40}", record["gitBlob"])): return False
    return True


def same_state(before, after, suite="restore-headers"):
    if not (type(before) is dict and type(after) is dict and before == after and
            valid_tools(before.get("executables"), suite) and "installedClosure" in before): return False
    return before["installedClosure"] is None if suite == "restore-controls" else type(before["installedClosure"]) is dict


def capture_tool(path, reader=read_bytes):
    metadata = {}
    data = reader(str(path), LIMIT, executable=True, deadline=time.monotonic() + 30, metadata=metadata)
    if not metadata: raise ValueError("tool-metadata")
    return {**pin(data), "mode": metadata["mode"]}


class Resolution:
    def __init__(self): self.values = []
    def consume(self, line):
        value = strict_json(line, 262144)
        if type(value) is not dict or len(self.values) != 0: raise ValueError("tool-resolution")
        self.values.append(value)


def probe(node, script, phases, name):
    collector, phase = Resolution(), {}
    phases[name] = phase
    execute([str(node), "-e", script], 5, 10, collector, phase, str(APP / "e2e"))
    if not accepted(phase) or len(collector.values) != 1: raise ValueError("tool-resolution")
    return collector.values[0]


def installed_closure(roots):
    rows, total, examined, directories = [], 0, 0, 0
    end = time.monotonic() + 30
    flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK
    def directory_state(info):
        return (info.st_dev, info.st_ino, info.st_mode, info.st_mtime_ns, info.st_ctime_ns)
    def close(descriptor):
        try: os.close(descriptor)
        except OSError as error: raise ValueError("installed-close") from error
    def visit(descriptor, label, relative, depth):
        nonlocal total, examined, directories
        if depth > 32 or time.monotonic() >= end: raise ValueError("installed-depth-time")
        before = os.fstat(descriptor)
        if not stat.S_ISDIR(before.st_mode): raise ValueError("installed-directory")
        iterator, entries, pending = os.scandir(descriptor), [], 0
        try:
            for entry in iterator:
                examined += 1
                if examined > 5000 or time.monotonic() >= end: raise ValueError("installed-entry-bound")
                name = entry.name
                if type(name) is not str or not name or name in (".", "..") or "/" in name or "\0" in name:
                    raise ValueError("installed-name")
                if name == "node_modules": continue
                info = entry.stat(follow_symlinks=False)
                if stat.S_ISDIR(info.st_mode):
                    pending += 1; directories += 1
                    if pending > 128 or directories > 5000: raise ValueError("installed-directory-bound")
                elif not stat.S_ISREG(info.st_mode): raise ValueError("installed-entry")
                entries.append((name, info))
        finally:
            try: iterator.close()
            except OSError as error: raise ValueError("installed-scan-close") from error
        entries.sort(key=lambda row: row[0])
        for name, info in entries:
            path = relative + name
            if stat.S_ISDIR(info.st_mode):
                child = os.open(name, flags, dir_fd=descriptor)
                try:
                    actual = os.fstat(child)
                    if (actual.st_dev, actual.st_ino, actual.st_mode) != (info.st_dev, info.st_ino, info.st_mode):
                        raise ValueError("installed-child-swap")
                    visit(child, label, path + "/", depth + 1)
                finally: close(child)
            else:
                data = read_bytes(name, 4 * 1024 * 1024, deadline=end, dir_fd=descriptor)
                total += len(data)
                if total > 64 * 1024 * 1024 or len(rows) >= 5000: raise ValueError("installed-bound")
                rows.append({"package": label, "path": path, **pin(data)})
        if directory_state(os.fstat(descriptor)) != directory_state(before): raise ValueError("installed-drift")
    for label, root in sorted(roots.items()):
        descriptor = os.open(str(root), flags)
        try: visit(descriptor, label, "", 0)
        finally: close(descriptor)
    rows.sort(key=lambda row: (row["package"], row["path"]))
    if not rows: raise ValueError("installed-empty")
    canonical = json.dumps(rows, sort_keys=True, separators=(",", ":")).encode()
    return {"files": rows, "canonical": {"bytes": len(canonical), "sha256": hashlib.sha256(canonical).hexdigest()}}


def capture(phases, suite="restore-headers", context=None):
    if suite not in ("restore-controls", "restore-headers", "restore-inspect-body"): raise ValueError("dependency-mode")
    paths = {}
    for name in (("go",) if suite == "restore-controls" else ("go", "node", "pnpm")):
        found = shutil.which(name)
        if found is None: raise ValueError("tool-unavailable")
        paths[name] = Path(found).resolve(strict=True)
    records = {name: capture_tool(path) for name, path in paths.items()}
    if context is not None: context.update(paths=paths)
    if suite == "restore-controls":
        return {"executables": records, "installedClosure": None,
                "limits": "Selected Go executable only; no browser, Node, pnpm or installed browser dependency was used."}
    resolved = probe(paths["node"], RESOLVE, phases, "packageResolution")
    roots = package_roots(resolved, str(ROOT))
    for name, root in roots.items():
        value = strict_json(read_bytes(str(root / "package.json"), 131072), 131072)
        if value != resolved[name]["package"]: raise ValueError("package-drift")
    if context is not None: context.update(roots=roots)
    closure = installed_closure(roots)
    registry = strict_json(read_bytes(str(roots["playwright-core"] / "browsers.json"), 262144), 262144)
    cache = str(Path(os.environ.get("PLAYWRIGHT_BROWSERS_PATH", str(Path.home() / ".cache/ms-playwright"))).resolve(strict=True))
    expected = browser_paths(registry, cache)
    actual = probe(paths["node"], registry_script(roots["playwright-core"]), phases, "browserResolution")
    if actual != expected: raise ValueError("browser-resolution")
    for key, path in expected.items(): records[key] = capture_tool(path)
    if installed_closure(roots) != closure or not valid_tools(records, suite): raise ValueError("tool-drift")
    return {"executables": records, "installedClosure": closure,
            "limits": "Selected executable and three transitive package roots only; system libraries and whole runner not fingerprinted."}


def dependency_reason(error):
    known = {"source-path", "source-parent", "source-shape", "source-deadline", "source-overflow",
             "source-drift", "source-close", "tool-unavailable", "tool-metadata", "tool-resolution"}
    if type(error) is ValueError and len(error.args) == 1 and type(error.args[0]) is str and error.args[0] in known:
        return error.args[0]
    return "unclassified"
