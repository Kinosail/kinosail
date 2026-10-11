#!/usr/bin/env python3
"""Q47 exact tracked source, dependency, generated-docs and binary fingerprints."""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
from campaign_q47_execution import execute, complete, environment, check_budget
from campaign_q47_admission import format_binding_valid
from campaign_q47_io import fingerprint, read_bounded, bounded_paths
import campaign_q47_dependencies as diagnostics
from campaign_q47_package_resolution import resolve_package
from campaign_q47_product_binding import PRODUCT_CHANGE, APPROVED_PRODUCT_BLOBS
from campaign_q47_supplementary_pins import output_pins

ROOT = Path(__file__).resolve().parents[3]
APP = ROOT / "apps/player/e2e"
FIXTURE_FILES = ("apps/player/e2e/compose-template-fixture.go", "apps/player/e2e/compose-template-peer.go")
FORMAT_INPUTS = (*FIXTURE_FILES, "apps/player/scripts/campaign_q47_format.py", "apps/player/scripts/test_campaign_q47_format.py",
                 ".github/workflows/layout-stability.yml", "scripts/ci/run-campaign-proof.sh", "go.work")
# Historical canonical pair binding admitted and independently reviewed by root.
FORMAT_BINDING = {"schemaVersion":1,"mode":"source-format","revision":"3a2322a68a6c8580b915cb53fe7b43f70479c868","tree":"20f5734476261834800074221846891c1b1489cc","runId":"37309852573","artifactId":"11344634805","sourceManifestSHA256":"c6c95e6956617e379c29dc78a4c315bae6c20c5373eb4308bf9f7d1ab7fb51e1","formatter":{"bytes":3144728,"sha256":"86dd91f69254432a37ca365f964720677040fd6f049a412c23e25f8b3812f342"},"inputs":[{"path":"apps/player/e2e/compose-template-fixture.go","bytes":2869,"sha256":"1078f61a5cff0926d0a4a592d337ce64751e232b6162adde1548da2c5c064a80"},{"path":"apps/player/e2e/compose-template-peer.go","bytes":5716,"sha256":"b9d6e06e0251bf0c743e2d614de2c38129695db1b18abdd3003703b95cd7fdac"},{"path":"apps/player/scripts/campaign_q47_format.py","bytes":12799,"sha256":"b95de5e642b334f48fc9c60f6c35c662316992333b3a5cd44df8e85371950741"},{"path":"apps/player/scripts/test_campaign_q47_format.py","bytes":17683,"sha256":"f64854acb41536a4c1b22f0e066089d51c46e7b42ed398481d78ac9d90e7d088"},{"path":".github/workflows/layout-stability.yml","bytes":10545,"sha256":"de61497ec3b6d2efe642b84650b5dd1efe9bee00047368226acd251d05cb4066"},{"path":"scripts/ci/run-campaign-proof.sh","bytes":922,"sha256":"32ae097e0512fe4bebecd7eb5cb8228228d969deb9fcaaab7acae6dc1e328b50"},{"path":"go.work","bytes":62,"sha256":"765a0af6b5ad93fa49fb0bae0fdc444df2d775393a2aa7e4d4aacfd4da567390"}],"outputs":[{"path":"apps/player/e2e/compose-template-fixture.go","bytes":2869,"lines":103,"sha256":"1078f61a5cff0926d0a4a592d337ce64751e232b6162adde1548da2c5c064a80"},{"path":"apps/player/e2e/compose-template-peer.go","bytes":5718,"lines":210,"sha256":"245ec58619efa6e17a230c39dfaacad796691412418586880e63f7dab7caab45"}]}
BASE = "bd1ea2e148787ae8d4a0a46640bc2a965e10fe6a"
FROZEN = {
    "apps/player/e2e/compose-template-recovery.journey.ts": "2f0836813c25e0c6dc0312712b7048e016c12181373b0327022b721f4a97e8a6",
    "apps/player/e2e/compose-template-recovery.config.ts": "62a269660ba877acdc3336fc17d60e44d3a94881be86cf0b57a5d0e4820b63fe",
    "apps/player/e2e/compose-template-proof-reporter.ts": "52ce05e0d5c1d88c5e8b5413aebd60228f4bd0a12809df9e25017b2c6e4a05f5",
    "apps/player/e2e/compose-template-proof-attachments.ts": "6365b41e61e489cfc6bb0830d5d4b504b15047d1314486b8cb38b2a1be2bcb03",
    "apps/player/e2e/compose-template-proof-attachments.controls.cjs": "0ed12672d83ccd97a97ad9d895116d052f37ad635ee829f7bdea4b00ce064c00",
    "apps/player/e2e/compose-template-recovery.observation.ts": "2882b641dff938b0737a6d861069e2da6a6d12fa2f8c75e21fcee62d9411f4c1",
    "engineering/documentation/test-install-builder-recovery.cjs": "53d63a79a207a9f16dc1c67c5047907667ce814c9fa407381ca93574b827a277",
    "engineering/qa/2026-10-05-q47-compose-template/failure-analysis.md": "da520e8f06d296e8972d6471ebee4f845508250243eccb60247ddc3a1644aaf7"
}
BASE_BLOBS = {
    "apps/player/docs/assets/js/platform-install.js": "e778290caacb567cc67384fda07a4f2b40272833",
    "apps/player/docs/getting-started/platforms.md": "80695914c8ccf3993a4a2b9df8fdd7af5e154877",
    "apps/player/packaging/platform-compose.yaml": "4a5fbc2f894130790943da9f003cbf583c119c19",
    "apps/subtitles/packaging/platform-compose.yaml": "8680c29d2e00a9eab3f0040191edb088ce30f66c",
    "apps/player/packaging/platform-compose-both.yaml": "90937e3dc3320943b9d08642d78303ee13c952db",
    "engineering/documentation/build.py": "c082c9cad2fd630c4c381c15878e8f7217d905bd",
    "engineering/documentation/test-install-builder.cjs": "9c0664f9b6ac2648cb162b591ef7824b682e38af",
    "scripts/tooling/test-platform-install-kits.py": "6a9adaa557b2964e0bb08630399e2679ffe4bdb7",
    "apps/player/e2e/package.json": "c4b2a5258bbc83cc96bdd6feb01ad55e70ca8cf2",
    "apps/player/e2e/pnpm-lock.yaml": "d5d8c670227b0a8d3d71b95027e4afe5c34477ed",
    "engineering/documentation/package.json": "33d89eb9243f49686a20570803658bfd8f5d1de8",
    "engineering/documentation/package-lock.json": "59a9ebe3ee5d9ea96e656015e3e5b2374207f2d9",
    "engineering/documentation/Gemfile": "322b9c581f4414877b559941d937e0904abbff47",
    "engineering/documentation/Gemfile.lock": "c7237722aaef72bc0cbeb4d82642244a6d87a76d"
}
TEMPLATES = {"player": "apps/player/packaging/platform-compose.yaml",
             "subtitles": "apps/subtitles/packaging/platform-compose.yaml",
             "both": "apps/player/packaging/platform-compose-both.yaml"}


def canonical(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=True, allow_nan=False) + "\n").encode()


def git(*args):
    result, output = execute(["git", *args], 10, ROOT, environment(), label="source-git")
    if not complete(result, 0) or len(output) > 4 * 1024 * 1024:
        raise ValueError("git_prerequisite")
    return output


def identity():
    revision = git("rev-parse", "HEAD").decode().strip()
    tree = git("rev-parse", "HEAD^{tree}").decode().strip()
    if not all(re.fullmatch("[0-9a-f]{40}", item) for item in (revision, tree)):
        raise ValueError("git_identity")
    git("diff", "--quiet", "HEAD", "--")
    if (ROOT / ".gates-disabled").exists() or (ROOT / ".gates-disabled").is_symlink():
        raise ValueError("gates_disabled")
    if (os.environ.get("GITHUB_ACTIONS") != "true" or os.environ.get("RUNNER_OS") != "Linux"
            or os.environ.get("GITHUB_SHA") != revision):
        raise ValueError("hosted_revision")
    return {"revision": revision, "tree": tree, "trackedClean": True}


def tracked_sources():
    records = []
    for entry in git("ls-files", "--stage", "-z").split(b"\0"):
        if not entry:
            continue
        header, raw = entry.split(b"\t", 1)
        mode, blob, stage = header.decode("ascii").split()
        name = raw.decode("utf-8")
        if (stage != "0" or mode not in ("100644", "100755", "120000")
                or len(name) > 512 or any(char in name for char in "\n\r\t\0")
                or Path(name).is_absolute() or ".." in Path(name).parts):
            raise ValueError("tracked_entry")
        path = ROOT / name
        if mode == "120000":
            content = os.readlink(path).encode()
            record = {"bytes": len(content), "sha256": hashlib.sha256(content).hexdigest(),
                      "gitBlob": hashlib.sha1(b"blob " + str(len(content)).encode() + b"\0" + content).hexdigest()}
        else:
            record = fingerprint(path)
            if bool(path.stat().st_mode & 0o111) != (mode == "100755"):
                raise ValueError("tracked_mode")
        if record["gitBlob"] != blob:
            raise ValueError("tracked_bytes")
        records.append({"path": name, "mode": mode, **record})
    if not 100 <= len(records) <= 10_000:
        raise ValueError("tracked_count")
    records.sort(key=lambda item: item["path"])
    return {"count": len(records), "sha256": hashlib.sha256(canonical(records)).hexdigest(), "files": records}


def source_snapshot():
    state = identity()
    tracked = tracked_sources()
    by_name = {item["path"]: item for item in tracked["files"]}
    for path, expected in FROZEN.items():
        if by_name.get(path, {}).get("sha256") != expected:
            raise ValueError("frozen_source")
    for path, expected in BASE_BLOBS.items():
        expected = APPROVED_PRODUCT_BLOBS.get(path, expected)
        if by_name.get(path, {}).get("gitBlob") != expected:
            raise ValueError("baseline_product_changed")
    if not format_binding_valid(FORMAT_BINDING, FIXTURE_FILES, FORMAT_INPUTS):
        raise ValueError("canonical_format_pending")
    fixtures = []
    for expected in FORMAT_BINDING["outputs"]:
        path = ROOT / expected["path"]
        record = fingerprint(path)
        lines = len(read_bounded(path, 512 * 1024).splitlines())
        if any(record[key] != expected[key] for key in ("bytes", "sha256")) or lines != expected["lines"]:
            raise ValueError("canonical_format_bytes")
        fixtures.append({"path": expected["path"], "lines": lines, **record})
    return {"base": BASE, "identity": state, "tracked": tracked, "fixtures": fixtures,
            "productChange": PRODUCT_CHANGE, "formatBinding": FORMAT_BINDING, "formatBoundary": "Historical formatter artifacts separately admitted by root."}


def installed_tree(root):
    root = root.resolve(strict=True)
    records, total = [], 0
    for count, path in enumerate(bounded_paths(root, 20_000), 1):
        check_budget()
        if count > 20_000:
            raise ValueError("dependency_entry_bound")
        relative = path.relative_to(root).as_posix()
        if len(relative) > 512 or any(char in relative for char in "\n\r\t\0"):
            raise ValueError("dependency_path")
        if path.is_symlink():
            target = path.resolve(strict=True)
            if not target.is_relative_to(APP / "node_modules"):
                raise ValueError("dependency_link")
            content = os.readlink(path).encode()
            record = {"bytes": len(content), "sha256": hashlib.sha256(content).hexdigest()}
        elif path.is_file():
            record = fingerprint(path, 64 * 1024 * 1024)
        elif path.is_dir():
            continue
        else:
            raise ValueError("dependency_kind")
        total += record["bytes"]
        if total > 128 * 1024 * 1024:
            raise ValueError("dependency_byte_bound")
        records.append({"path": relative, **record})
    if not records:
        raise ValueError("dependency_bound")
    records.sort(key=lambda item: item["path"])
    return {"files": len(records), "bytes": total, "sha256": hashlib.sha256(canonical(records)).hexdigest()}


def tool_path(name):
    value = shutil.which(name)
    if not value:
        raise ValueError("tool_missing")
    path = Path(value).resolve(strict=True)
    if not path.is_file() or not os.access(path, os.X_OK):
        raise ValueError("tool_executable")
    return path


def dependencies():
    diagnostics.reset_stage()
    tools = {}
    for name in ("go", "node", "ruby", "bundle"):
        diagnostics.record_stage(name + "-resolution")
        tools[name] = tool_path(name)
    limits = {"go": 64, "node": 192, "ruby": 64, "bundle": 64}
    pins = {}
    for name, path in tools.items():
        diagnostics.record_stage(name + "-fingerprint")
        pins[name] = fingerprint(path, limits[name] * 1024 * 1024)
    packages, roots = {}, {}
    for name in ("@playwright/test", "playwright", "playwright-core"):
        label = "playwright-test" if name == "@playwright/test" else name
        diagnostics.record_stage(label + "-metadata")
        importer = APP if name == "@playwright/test" else roots["@playwright/test" if name == "playwright" else "playwright"]
        selected = resolve_package(APP, importer, name)
        root = selected["root"]
        roots[name] = root
        if not root.is_relative_to(APP / "node_modules"):
            raise ValueError("dependency_root")
        metadata = json.loads(read_bounded(root / "package.json", 512 * 1024).decode("utf-8"))
        if metadata["name"] != name or metadata["version"] != "1.64.0":
            raise ValueError("dependency_version")
        diagnostics.record_stage(label + "-tree")
        packages[name] = {**installed_tree(root), "resolvedFrom": selected["resolvedFrom"], "layout": selected["layout"]}
    diagnostics.record_stage("cli-resolution")
    cli = (roots["@playwright/test"] / "cli.js").resolve(strict=True)
    if not cli.is_relative_to(roots["@playwright/test"]):
        raise ValueError("dependency_cli_root")
    diagnostics.record_stage("cli-fingerprint")
    pins["playwright-cli"] = fingerprint(cli)
    diagnostics.record_stage("registry-read")
    registry = json.loads(read_bounded(roots["playwright-core"] / "browsers.json", 262_144).decode("utf-8"))
    diagnostics.record_stage("cache-resolution")
    cache = Path(os.environ.get("PLAYWRIGHT_BROWSERS_PATH", str(Path.home() / ".cache/ms-playwright"))).resolve(strict=True)
    diagnostics.record_stage("cache-validation")
    if str(cache) == "/" or not cache.is_dir():
        raise ValueError("browser_cache")
    # Pinned Playwright 1.64.0 registry, Linux x64; no guessed legacy executable names.
    for name, suffix, executable in (("chromium", "chrome-linux64", "chrome"),
                                     ("chromium-headless-shell", "chrome-headless-shell-linux64", "chrome-headless-shell")):
        label = "chromium" if name == "chromium" else "headless"
        diagnostics.record_stage(label + "-registry-entry")
        entry = next(item for item in registry["browsers"] if item["name"] == name)
        revision = entry["revision"]
        if type(revision) is not str or not re.fullmatch("[0-9]{1,6}", revision):
            raise ValueError("browser_revision")
        diagnostics.record_stage(label + "-executable")
        directory = cache / (name.replace("-", "_") + "-" + revision)
        selected = (directory / suffix / executable).resolve(strict=True)
        if not selected.is_relative_to(cache) or not selected.is_file() or not os.access(selected, os.X_OK):
            raise ValueError("browser_binary")
        diagnostics.record_stage(label + "-fingerprint")
        pins[name] = fingerprint(selected, 512 * 1024 * 1024)
    diagnostics.record_stage("inspection-complete")
    return tools, cli, {"tools": pins, "packages": packages}


def generated(site):
    records = []
    total = 0
    for count, path in enumerate(bounded_paths(site, 10_000), 1):
        check_budget()
        if count > 10_000:
            raise ValueError("generated_entry_bound")
        if path.is_symlink():
            raise ValueError("generated_link")
        if path.is_dir():
            continue
        name = path.relative_to(site).as_posix()
        if len(name) > 512 or any(char in name for char in "\n\r\t\0"):
            raise ValueError("generated_name")
        record = fingerprint(path, 64 * 1024 * 1024)
        total += record["bytes"]
        if total > 256 * 1024 * 1024:
            raise ValueError("generated_byte_bound")
        records.append({"path": name, **record})
    if not records:
        raise ValueError("generated_bound")
    records.sort(key=lambda item: item["path"])
    selected = {item["path"]: item for item in records}
    required = ["getting-started/platforms/index.html", "assets/js/platform-install.js", "assets/js/docs.js",
                "assets/css/docs.css", "assets/fonts/manrope.woff2"] + ["assets/install/" + key + ".yaml" for key in TEMPLATES]
    if not all(name in selected for name in required):
        raise ValueError("generated_required")
    pairs = {"assets/js/platform-install.js": "apps/player/docs/assets/js/platform-install.js",
             "assets/js/docs.js": "apps/player/docs/assets/js/docs.js", "assets/css/docs.css": "apps/player/docs/assets/css/docs.css",
             "assets/fonts/manrope.woff2": "packages/webassets/static/fonts/manrope.woff2"}
    pairs.update({"assets/install/" + key + ".yaml": value for key, value in TEMPLATES.items()})
    if any(selected[target]["sha256"] != fingerprint(ROOT / source)["sha256"] for target, source in pairs.items()):
        raise ValueError("generated_copy_mismatch")
    outputs = []
    marker = 'source: "$' + '{KINOSAIL_MEDIA_PATH:?Set an existing absolute media path}"'
    for app in ("player", "both"):
        text = read_bounded(site / ("assets/install/" + app + ".yaml"), 16_384).decode("utf-8")
        text = text.replace(marker, 'source: "/fictional/q47/media"').replace('- "38127:38127"', '- "49127:38127"')
        if app == "both":
            text = text.replace('- "38128:38128"', '- "49128:38128"')
        data = text.encode()
        outputs.append({"bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()})
    pins = {"helper": {key: selected["assets/js/platform-install.js"][key] for key in ("bytes", "sha256")},
            "templates": {app: {key: selected["assets/install/" + app + ".yaml"][key] for key in ("bytes", "sha256")}
                          for app in TEMPLATES}, "outputs": outputs}
    pins["supplementaryOutputs"] = output_pins({app: read_bounded(site / ("assets/install/" + app + ".yaml"), 16_384).decode("utf-8")
                                                for app in TEMPLATES})
    return {"files": records, "sha256": hashlib.sha256(canonical(records)).hexdigest()}, pins


def private_report(path):
    data = read_bounded(path, 262_144, mode=0o600)
    if not data or b"\0" in data:
        raise ValueError("private_report")
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError("duplicate_json_key")
            result[key] = value
        return result
    return json.loads(data.decode("utf-8"), object_pairs_hook=pairs,
                      parse_constant=lambda _: (_ for _ in ()).throw(ValueError("nonfinite_json")))
