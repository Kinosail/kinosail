"""Fixed Restore selection and stable full-source capture; no import-time IO."""
import hashlib
import json
import math
import os
from pathlib import Path
import re
import stat
import subprocess
import time

ROOT = Path(__file__).absolute().parents[3]
APP = ROOT / "apps/subtitles"
FIXTURE = APP / "engineering/qa/2026-10-05-restore-recovery/fixture"
PACKAGE = "github.com/MikeO7/kinosail-subtitles/engineering/qa/2026-10-05-restore-recovery/fixture"
NAMES = ("TestRestoreLegacySwapPublicControl", "TestRestorePreparedReceiptPublicControl",
         "TestRestoreHeldHeadersPublicControl", "TestRestoreHeldInspectionBodyPublicControl")
SUITES = {
    "restore-controls": ((), "^$"),
    "restore-headers": (("r06-restore-headers-desktop", "r06-restore-headers-phone"),
                        "(?:^| )R06 Restore held headers releases (desktop|phone) editor$"),
    "restore-inspect-body": (("r06-restore-inspect-body-desktop", "r06-restore-inspect-body-phone"),
                             "(?:^| )R06 Restore held inspection body releases (desktop|phone) editor$"),
}

QA = "apps/subtitles/engineering/qa/2026-10-05-restore-recovery/"
SCRIPT = "apps/subtitles/scripts/"
REQUIRED_INPUTS = {
    *(QA + "fixture/" + name for name in ["restore_assertions_test.go","restore_controls_test.go","restore_exchange_test.go","restore_filesystem_test.go","restore_http_test.go","restore_main_test.go","restore_owner_enrollment_test.go","restore_owner_test.go","restore_requests_test.go","restore_routes_test.go","restore_routing_test.go","restore_target_test.go","restore_transport_test.go","restore_witness_test.go"]),
    *("apps/subtitles/e2e/" + name for name in ["subtitle-restore-proof-reporter.ts","subtitle-restore-recovery-fixture.ts","subtitle-restore-recovery-helpers.ts","subtitle-restore-recovery-network.ts","subtitle-restore-recovery.config.ts","subtitle-restore-recovery.journey.ts","subtitle-save-recovery-auth.ts","subtitle-save-attachment-reader.cjs"]),
    SCRIPT + "campaign-r06-restore-browser.py",
    *(SCRIPT + "campaign_r06_restore_runtime_" + name + ".py"
      for name in ("sources", "process", "controls", "projection", "tools", "artifacts")),
    *(SCRIPT + "test_campaign_r06_restore_runtime_" + name + ".py"
      for name in ("selection", "sources", "process", "controls", "projection", "tools")),
    *(QA + name for name in ("runtime-failure-analysis.md", "runtime-test-contract.json", "runtime-source-manifest.json",
                            "fixture-source-manifest.json", "test-contract.json", "canonical-adoption-source-manifest.json")),
}

STARTUP_BASELINE = {"commit":"2b816fa8484279aa906116446f305d8f49ceb66d","tree":"d511ed8f7634e6c82c8ec847eadcb9a8ab39bf65","kind":"published-startup-corrected-raw-inputs","formatterQualification":"pending","canonicalOutputs":None}
STARTUP_RAW_PINS = {
    "apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_main_test.go": (9238, "09e1ca5c0277ba72237cfdbb9ad880cca048d13d01ec31ea55ae1b77e946a9d4", "8b53d7a674a4099d5b318ae35d4fd0bff41d549e"),
    "apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_filesystem_test.go": (7649, "37b2c37f8534cc220871f6892c2b0e76be9e77ad7e7683be561d24b322000685", "e330d8bb78d411b32565ccb6c98d626eba9c6230"),
    "apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture-source-manifest.json": (19363, "b627c147c2a3f569ce6df2f9122a15a9fcefdb6a348216640fc8b5cf15e5a9a7", "62bf1292673af484aa170bb0534d66677e8f250d"),
}


def selection(environment, argv):
    if (argv or environment.get("CAMPAIGN_PROOF") != "R06" or
            environment.get("GITHUB_ACTIONS") != "true" or environment.get("RUNNER_OS") != "Linux" or
            type(environment.get("GITHUB_SHA")) is not str or
            not re.fullmatch("[a-f0-9]{40}", environment["GITHUB_SHA"]) or
            environment.get("CAMPAIGN_R06_SUITE") not in SUITES):
        raise ValueError("selection")
    return environment["CAMPAIGN_R06_SUITE"]


def commands(suite, binary, work):
    if (suite not in SUITES or type(binary) is not str or type(work) is not str or
            not Path(binary).is_absolute() or not Path(work).is_absolute() or
            Path(binary) != Path(work) / "restore-fixture.test" or "\0" in binary + work):
        raise ValueError("command-boundary")
    base = ["pnpm", "--dir", str(APP / "e2e"), "exec", "playwright", "test",
            "--config", str(APP / "e2e/subtitle-restore-recovery.config.ts")]
    return {
        "compile": ["go", "test", "-c", "-p", "1", "-o", binary,
                    "./engineering/qa/2026-10-05-restore-recovery/fixture"],
        "controls": ["go", "tool", "test2json", "-t", "-p", PACKAGE, binary,
                     "-test.v=test2json", "-test.run=^(" + "|".join(NAMES) + ")$",
                     "-test.parallel=1", "-test.count=1", "-test.timeout=60s"],
        "collection": base + ["--list", "--global-timeout=15000"],
        "browser": base + ["--global-timeout=230000", "--grep", SUITES[suite][1]],
    }


def pin(data):
    return {"bytes": len(data), "sha256": hashlib.sha256(data).hexdigest(),
            "gitBlob": hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()}


def strict_json(raw, limit=4194304):
    if type(raw) is not bytes or not raw or len(raw) > limit or b"\0" in raw:
        raise ValueError("json-bound")
    depth, quoted, escape = 0, False, False
    for byte in raw:
        if quoted:
            if escape: escape = False
            elif byte == 92: escape = True
            elif byte == 34: quoted = False
        elif byte == 34: quoted = True
        elif byte in (91, 123):
            depth += 1
            if depth > 32: raise ValueError("json-depth")
        elif byte in (93, 125): depth -= 1
    def unique(pairs):
        value = {}
        for key, item in pairs:
            if key in value: raise ValueError("json-duplicate")
            value[key] = item
        return value
    def finite_float(value):
        number = float(value)
        if not math.isfinite(number): raise ValueError("json-nonfinite")
        return number
    try:
        return json.loads(raw.decode("utf-8"), object_pairs_hook=unique, parse_float=finite_float,
                          parse_constant=lambda _: (_ for _ in ()).throw(ValueError("json-nonfinite")))
    except (UnicodeError, json.JSONDecodeError, RecursionError) as error:
        raise ValueError("json-shape") from error


def checked_parents(path):
    target = Path(path)
    if not target.is_absolute() or ".." in target.parts or "\0" in str(target):
        raise ValueError("source-path")
    for parent in reversed(target.parents):
        info = os.lstat(parent)
        if not stat.S_ISDIR(info.st_mode) or stat.S_ISLNK(info.st_mode):
            raise ValueError("source-parent")


def state(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def read_bytes(path, limit, executable=False, deadline=None, metadata=None, dir_fd=None):
    if dir_fd is None: checked_parents(path)
    elif type(dir_fd) is not int or dir_fd < 0 or type(path) is not str or Path(path).name != path or path in (".", ".."): raise ValueError("source-dir-fd")
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK, **({} if dir_fd is None else {"dir_fd": dir_fd}))
    data, error = bytearray(), None
    try:
        before = os.fstat(descriptor)
        if (not stat.S_ISREG(before.st_mode) or not 0 <= before.st_size <= limit or
                executable and (before.st_size == 0 or not before.st_mode & 0o111)):
            raise ValueError("source-shape")
        while True:
            if deadline is not None and time.monotonic() >= deadline: raise ValueError("source-deadline")
            block = os.read(descriptor, min(65536, limit + 1 - len(data)))
            if not block: break
            data.extend(block)
            if len(data) > limit: raise ValueError("source-overflow")
        if len(data) != before.st_size or state(os.fstat(descriptor)) != state(before):
            raise ValueError("source-drift")
        if metadata is not None:
            metadata.update(mode=before.st_mode & 0o777, identity=state(before))
    except BaseException as failure:
        error = failure
    try: os.close(descriptor)
    except OSError as failure: error = ValueError("source-close")
    if error is not None: raise error
    return bytes(data)


def fingerprint(path, limit=134217728, executable=False, deadline=None):
    metadata = {}
    data = read_bytes(str(path), limit, executable, deadline, metadata)
    return {**pin(data), "mode": metadata["mode"]}


METADATA_RECEIPTS = []


class MetadataCapture:
    binary = True
    def __init__(self): self.data = bytearray()
    def consume(self, block):
        if type(block) is not bytes or len(self.data) + len(block) > 8 * 1024 * 1024:
            raise ValueError("git-bound")
        self.data.extend(block)


def execute_metadata(command, active, total, projection, receipt, cwd):
    from campaign_r06_restore_runtime_process import execute
    return execute(command, active, total, projection, receipt, cwd)


def git(*arguments):
    from campaign_r06_restore_runtime_process import accepted
    collector, receipt = MetadataCapture(), {}
    METADATA_RECEIPTS.append(receipt)
    execute_metadata(["git", *arguments], 5, 10, collector, receipt, str(ROOT))
    if not accepted(receipt): raise ValueError("git-unsettled")
    return bytes(collector.data)


def identity():
    return {"revision": git("rev-parse", "HEAD").decode().strip(),
            "tree": git("rev-parse", "HEAD^{tree}").decode().strip()}


def tracked_sources(deadline=None):
    if git("diff", "HEAD", "--name-only", "-z") or git("status", "--porcelain", "--untracked-files=no"):
        raise ValueError("source-dirty")
    rows = []
    for entry in git("ls-files", "--stage", "-z").split(b"\0"):
        if not entry: continue
        metadata, name = entry.split(b"\t", 1)
        mode, expected, stage = metadata.decode().split()
        name = name.decode()
        if stage != "0" or mode not in ("100644", "100755", "120000") or not re.fullmatch("[a-f0-9]{40}", expected):
            raise ValueError("source-index")
        path = ROOT / name
        if not name or Path(name).is_absolute() or ".." in Path(name).parts: raise ValueError("source-path")
        if mode == "120000":
            checked_parents(path)
            before = os.lstat(path)
            if not stat.S_ISLNK(before.st_mode): raise ValueError("source-symlink")
            data = os.fsencode(os.readlink(path))
            if state(os.lstat(path)) != state(before): raise ValueError("source-symlink-drift")
        else:
            metadata = {}
            data = read_bytes(str(path), 32 * 1024 * 1024, deadline=deadline, metadata=metadata)
            if bool(metadata["mode"] & 0o111) != (mode == "100755"): raise ValueError("source-mode")
        value = pin(data)
        if value["gitBlob"] != expected: raise ValueError("source-blob")
        rows.append({"path": name, "gitMode": mode, **value})
    rows.sort(key=lambda row: row["path"])
    if not 1 <= len(rows) <= 10000 or len({row["path"] for row in rows}) != len(rows):
        raise ValueError("source-count")
    canonical = json.dumps(rows, sort_keys=True, separators=(",", ":")).encode()
    return {"schemaVersion": 1, "scope": "entire-tracked-source", "files": rows,
            "canonical": {key: value for key, value in pin(canonical).items() if key != "gitBlob"}}


def bind_inputs(ledger, rows):
    indexed = {row["path"]: row for row in ledger["files"]}
    bound = {}
    for row in rows:
        actual = indexed.get(row["path"])
        if actual is None or actual["gitMode"] not in ("100644", "100755"):
            raise ValueError("input-path")
        expected = {"bytes": row["bytes"], "gitBlob": row["blob"], "sha256": row["sha256"]}
        if {key: actual[key] for key in expected} != expected: raise ValueError("input-pin")
        bound[row["path"]] = expected
    if len(bound) != len(rows): raise ValueError("input-duplicate")
    return bound


def inspector_identity():
    paths = ("subtitle-source-cues.js", "subtitle-save-operation.js", "subtitle-inspector.js")
    return pin(b"".join(read_bytes(str(APP / "internal/server/static" / name), 262144) for name in paths))


def package_manifest():
    path = APP / "engineering/qa/2026-10-05-restore-recovery/runtime-source-manifest.json"
    value = strict_json(read_bytes(str(path), 262144), 262144)
    if type(value) is not dict or value.get("schema") != "r06-restore-runtime-source-v1": raise ValueError("package-manifest")
    if value.get("currentInputBaseline") != STARTUP_BASELINE: raise ValueError("startup-input-baseline")
    rows = value.get("inputs")
    expected = {path for path in REQUIRED_INPUTS if not path.startswith(SCRIPT) and not path.startswith(QA + "runtime-")}
    if type(rows) is not list or len(rows) != 25: raise ValueError("startup-input-count")
    indexed = {}
    for row in rows:
        if (type(row) is not dict or type(row.get("path")) is not str or row["path"] not in expected or row["path"] in indexed or
                type(row.get("bytes")) is not int or not 0 <= row["bytes"] <= 33554432 or
                type(row.get("sha256")) is not str or not re.fullmatch("[a-f0-9]{64}", row["sha256"]) or
                type(row.get("blob")) is not str or not re.fullmatch("[a-f0-9]{40}", row["blob"])): raise ValueError("startup-input-shape")
        indexed[row["path"]] = row
    if set(indexed) != expected: raise ValueError("startup-input-scope")
    for path, expected_pin in STARTUP_RAW_PINS.items():
        row = indexed[path]
        if (row["bytes"], row["sha256"], row["blob"]) != expected_pin: raise ValueError("startup-raw-pin")
    return value
