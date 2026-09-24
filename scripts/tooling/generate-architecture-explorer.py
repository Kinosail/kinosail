#!/usr/bin/env python3
"""Generate the static data snapshot used by the Kinosail Code Atlas."""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys
from pathlib import Path


REPO = Path(__file__).resolve().parents[2]
APP = sys.argv[1] if len(sys.argv) in (2, 3) else ""
CHECK = len(sys.argv) == 3 and sys.argv[2] == "--check"
MODULES = {
    "player": "github.com/MikeO7/kinosail-player",
    "subtitles": "github.com/MikeO7/kinosail-subtitles",
}
if APP not in MODULES or (len(sys.argv) == 3 and not CHECK):
    raise SystemExit(f"usage: {sys.argv[0]} {{player|subtitles}} [--check]")
APP_ROOT = REPO / "apps" / APP
PUBLISHED = APP_ROOT / "docs" / "architecture-explorer" / "index.html"
TEMPLATE = REPO / "scripts" / "tooling" / "architecture-explorer-template.html"
TEMPLATE_STYLES = TEMPLATE.with_suffix(".css")
TEMPLATE_SCRIPT = TEMPLATE.with_suffix(".js")
MODULE = MODULES[APP]
SHARED_MODULE = "github.com/MikeO7/kinosail/packages"
FIRST_PARTY_MODULES = (MODULE, SHARED_MODULE)

RESPONSIBILITIES = {
    "cmd/kinosail": "Executable, transport, health, and lifecycle wiring",
    "internal/backup": "App database and settings-validation adapter for recovery",
    "internal/configuration": "Validated server configuration and provider settings",
    "internal/database": "SQLite-backed private state and durable storage",
    "internal/server": "HTTP API, web UI, identity, playback, and integrations",
    "packages/auditjournal": "Shared validated, tamper-evident activity history and notifications",
    "packages/backup": "Shared validated, encrypted recovery archives and atomic restore",
    "packages/documentdb": "Shared Player document storage, validation, and migration policy",
    "packages/library": "Shared library indexing, media metadata, and organization",
    "packages/playback": "Shared Player-owned planning, HLS delivery, and Jellyfin playback protocol",
    "packages/privatefile": "Shared private file permissions across supported platforms",
    "packages/quickconnect": "Shared short-lived device authorization and grants",
    "packages/remoteaccess": "Shared managed remote-access transport and lifecycle",
    "packages/servertransport": "Shared hardened HTTP, health, local TLS, and trust export",
    "packages/supporter": "Shared Player-owned supporter certificates, activation, and badge policy",
    "packages/transcodepolicy": "Shared Player codec support and FFmpeg argument policy",
    "packages/trustedhttps": "Shared certificate issuance, renewal, and trust state",
    "packages/updatecontrol": "Shared release selection, update plans, and recovery state",
    "packages/watchrooms": "Shared expiring synchronized playback rooms",
    "packages/owneraccess": "Shared owner-paired private management connections",
}

FUNC_RE = re.compile(r"^func\s+(?:\([^)]*\)\s*)?([A-Za-z_]\w*)\s*\(")
TYPE_RE = re.compile(r"^type\s+([A-Za-z_]\w*)\b")


def run_go_list() -> list[dict]:
    output = ""
    for directory in (APP_ROOT, REPO / "packages"):
        result = subprocess.run(
            ["go", "list", "-json", "./..."],
            cwd=directory,
            # Match the canonical container target on every developer and CI host.
            env=os.environ | {"GOOS": "linux", "GOARCH": "amd64", "CGO_ENABLED": "0"},
            check=True,
            capture_output=True,
            text=True,
        )
        output += result.stdout
    decoder = json.JSONDecoder()
    packages: list[dict] = []
    index = 0
    while index < len(output):
        while index < len(output) and output[index].isspace():
            index += 1
        if index >= len(output):
            break
        package, consumed = decoder.raw_decode(output, index)
        packages.append(package)
        index = consumed
    return packages


def relative_file(path: str) -> str:
    return str(Path(path).relative_to(REPO))


def short_package_name(import_path: str) -> str:
    if import_path == MODULE:
        return "."
    if import_path.startswith(MODULE + "/"):
        return import_path.removeprefix(MODULE + "/")
    return "packages/" + import_path.removeprefix(SHARED_MODULE + "/")


def symbols_for(path: Path) -> tuple[list[str], list[str], list[dict], int]:
    types: list[str] = []
    functions: list[str] = []
    symbols: list[dict] = []
    loc = 0
    for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        if line.strip():
            loc += 1
        type_match = TYPE_RE.match(line)
        if type_match:
            name = type_match.group(1)
            types.append(name)
            symbols.append({"name": name, "kind": "type", "line": line_number})
        function_match = FUNC_RE.match(line)
        if function_match:
            name = function_match.group(1)
            functions.append(name)
            symbols.append({"name": name, "kind": "function", "line": line_number})
    return sorted(set(types)), sorted(set(functions)), symbols, loc


def build_snapshot() -> dict:
    raw_packages = run_go_list()
    packages: list[dict] = []
    by_import_path: dict[str, dict] = {}

    for raw in raw_packages:
        import_path = raw["ImportPath"]
        if not import_path.startswith(FIRST_PARTY_MODULES):
            continue
        short_name = short_package_name(import_path)
        package_dir = Path(raw["Dir"])
        source_files = [relative_file(str(package_dir / name)) for name in raw.get("GoFiles", [])]
        source_files += [relative_file(str(package_dir / name)) for name in raw.get("CgoFiles", [])]
        source_files = sorted(source_files)
        files: list[dict] = []
        total_loc = 0
        all_types: list[str] = []
        all_functions: list[str] = []
        for source_file in source_files:
            types, functions, symbols, loc = symbols_for(REPO / source_file)
            total_loc += loc
            all_types.extend(types)
            all_functions.extend(functions)
            files.append(
                {
                    "path": source_file,
                    "name": Path(source_file).name,
                    "loc": loc,
                    "types": types,
                    "functions": functions,
                    "symbols": symbols,
                }
            )
        internal_imports = sorted(
            value for value in raw.get("Imports", []) if value.startswith(FIRST_PARTY_MODULES)
        )
        external_imports = sorted(
            value for value in raw.get("Imports", []) if not value.startswith(FIRST_PARTY_MODULES)
        )
        package = {
            "id": import_path,
            "name": short_name,
            "label": short_name.rsplit("/", 1)[-1] if short_name != "." else "kinosail",
            "kind": "command" if short_name.startswith("cmd/") else "package",
            "responsibility": RESPONSIBILITIES.get(short_name, "First-party Kinosail package"),
            "files": files,
            "fileCount": len(files),
            "testFileCount": len(raw.get("TestGoFiles", [])) + len(raw.get("XTestGoFiles", [])),
            "loc": total_loc,
            "types": sorted(set(all_types)),
            "functions": sorted(set(all_functions)),
            "imports": internal_imports,
            "externalImports": external_imports,
        }
        packages.append(package)
        by_import_path[import_path] = package

    incoming = {package["id"]: 0 for package in packages}
    edges: list[dict] = []
    for package in packages:
        for dependency in package["imports"]:
            if dependency not in by_import_path:
                continue
            incoming[dependency] += 1
            edges.append({"source": package["id"], "target": dependency})
    for package in packages:
        package["fanIn"] = incoming[package["id"]]
        package["fanOut"] = len(package["imports"])

    total_files = sum(package["fileCount"] for package in packages)
    total_loc = sum(package["loc"] for package in packages)
    total_symbols = sum(len(package["types"]) + len(package["functions"]) for package in packages)
    return {
        "commit": "main",
        "commitShort": "main",
        "module": MODULE,
        "sharedModule": SHARED_MODULE,
        "packages": sorted(packages, key=lambda package: (package["kind"], package["name"])),
        "edges": edges,
        "summary": {
            "packageCount": len(packages),
            "fileCount": total_files,
            "loc": total_loc,
            "symbolCount": total_symbols,
            "dependencyCount": len(edges),
        },
    }


def main() -> None:
    template = TEMPLATE.read_text(encoding="utf-8")
    data = json.dumps(build_snapshot(), separators=(",", ":"))
    social = ""
    if APP == "player":
        url = "https://kinosail.com/architecture-explorer/"
        image = "https://kinosail.com/assets/images/kinosail-docs-share.png"
        description = "Explore Kinosail Player package dependencies, files, source symbols, and guided architecture journeys."
        social = f"""
  <meta name="description" content="{description}">
  <link rel="canonical" href="{url}">
  <meta property="og:title" content="Kinosail Code Atlas · Kinosail Player Docs">
  <meta property="og:description" content="{description}">
  <meta property="og:type" content="website">
  <meta property="og:url" content="{url}">
  <meta property="og:image" content="{image}">
  <meta property="og:image:type" content="image/png">
  <meta property="og:image:width" content="1200">
  <meta property="og:image:height" content="630">
  <meta property="og:image:alt" content="Kinosail Player Docs sail mark and the words Install. Use. Connect.">
  <meta name="twitter:card" content="summary_large_image">
  <meta name="twitter:image" content="{image}">"""
    output = (
        template.replace("__KINOSAIL_ARCHITECTURE_STYLES__", TEMPLATE_STYLES.read_text(encoding="utf-8").rstrip("\n"))
        .replace("__KINOSAIL_ARCHITECTURE_SOCIAL__", social)
        .replace("__KINOSAIL_ARCHITECTURE_DATA__", data)
        .replace("__KINOSAIL_ARCHITECTURE_SCRIPT__", TEMPLATE_SCRIPT.read_text(encoding="utf-8").rstrip("\n"))
    )
    if CHECK:
        if not PUBLISHED.is_file() or PUBLISHED.read_text(encoding="utf-8") != output:
            raise SystemExit(f"stale architecture snapshot: {PUBLISHED}")
        print(f"checked {PUBLISHED}")
        return
    PUBLISHED.parent.mkdir(parents=True, exist_ok=True)
    PUBLISHED.write_text(output, encoding="utf-8")
    print(f"generated {PUBLISHED}")


if __name__ == "__main__":
    main()
