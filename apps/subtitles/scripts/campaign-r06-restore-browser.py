#!/usr/bin/env python3
"""Hosted bounded real Restore proof; detached source preparation only."""
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import time
import types

ROOT = Path(__file__).absolute().parents[3]
MODULES = {
    "campaign_r06_restore_runtime_sources": (13871, "a0c7da48c0d335a455da41e9b8ee799061788722e9024dfb0e3d19b579a00127", "b4a9b73435ea85e265b8296f87e11cece00d0ed9"),
    "campaign_r06_restore_runtime_process": (5682, "de8db722f69ae408b79959a4a152ea12fdd7f5cf4220cbbdb065a17c7ce7f158", "fb260e072eaae501ac9a85eb5d98d20eab5f801d"),
    "campaign_r06_restore_runtime_controls": (6376, '5f7d11b118f4fa6037dc01c10e03a1478210dec390580ebe06a9ab178953cf7b', '6c5366c48d60182a5db8f3980696ea1ae0ae4e01'),
    "campaign_r06_restore_runtime_projection": (11753, "7bb2c80874b6d81b6ae5db3585c8bdbc2508c7ef34d96c055dae74af43b34686", "5b61d4ff7daf5f563f57579f4319d61244812965"),
    "campaign_r06_restore_runtime_tools": (11816, 'f956c191675c7a17f114b5ef8f55e817d41f8db87cf7f850dd37878434fb4ce8', 'cb50230746654598c5f9433261dd6b0e3488e5f9'),
    "campaign_r06_restore_runtime_artifacts": (20951, "152fd639bef4c805981f57b94042eef451985a76fb0c21b5dcc1a036e6795cce", "d9a4cfcd554d72d395dc6e24218bc9a1e9ea6af7"),
}


def load_modules():
    # Verify exact reviewed helper bytes before executing any helper definitions.
    bodies = {}
    for name, expected in MODULES.items():
        path = ROOT / "apps/subtitles/scripts" / (name + ".py")
        for parent in reversed(path.parents):
            info = os.lstat(parent)
            if not stat.S_ISDIR(info.st_mode) or stat.S_ISLNK(info.st_mode): raise ValueError("bootstrap-parent")
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK)
        error, data = None, bytearray()
        try:
            before = os.fstat(fd)
            if not stat.S_ISREG(before.st_mode) or before.st_size != expected[0]: raise ValueError("bootstrap-shape")
            while True:
                block = os.read(fd, min(65536, expected[0] + 1 - len(data)))
                if not block: break
                data.extend(block)
                if len(data) > expected[0]: raise ValueError("bootstrap-overflow")
            after = os.fstat(fd)
            fields = ("st_dev", "st_ino", "st_mode", "st_size", "st_mtime_ns", "st_ctime_ns")
            if any(getattr(before, key) != getattr(after, key) for key in fields): raise ValueError("bootstrap-drift")
        except BaseException as failure: error = failure
        try: os.close(fd)
        except OSError: error = ValueError("bootstrap-close")
        if error is not None: raise error
        raw = bytes(data)
        if (len(raw), hashlib.sha256(raw).hexdigest(),
                hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest()) != expected:
            raise ValueError("bootstrap-pin")
        bodies[name] = raw
    loaded = {}
    for suffix in ("process", "sources", "controls", "projection", "tools", "artifacts"):
        name = "campaign_r06_restore_runtime_" + suffix
        module = types.ModuleType(name)
        module.__file__ = str(ROOT / "apps/subtitles/scripts" / (name + ".py"))
        sys.modules[name] = module
        exec(compile(bodies[name], module.__file__, "exec"), module.__dict__)
        loaded[suffix] = module
    return loaded


def main():
    suite = os.environ.get("CAMPAIGN_R06_SUITE")
    if (sys.argv[1:] or os.environ.get("CAMPAIGN_PROOF") != "R06" or
            os.environ.get("GITHUB_ACTIONS") != "true" or os.environ.get("RUNNER_OS") != "Linux" or
            not re.fullmatch("[a-f0-9]{40}", os.environ.get("GITHUB_SHA", "")) or
            suite not in ("restore-controls", "restore-headers", "restore-inspect-body")):
        return 2
    modules = load_modules()
    s, p, c, b, t, a = (modules[key] for key in ("sources", "process", "controls", "projection", "tools", "artifacts"))
    s.selection(os.environ, sys.argv[1:])
    os.umask(0o077)
    output = a.fresh_output(ROOT)
    work = output / "work"; work.mkdir(mode=0o700)
    private = work / "private"; private.mkdir(mode=0o700)
    binary = work / "restore-fixture.test"
    receipt = {"schemaVersion": 1, "id": "R06", "mode": "restore-browser", "suite": suite,
               "revision": os.environ["GITHUB_SHA"], "tree": None,
               "classification": "prerequisite-blocked", "sourceUnchanged": False,
               "dependenciesUnchanged": False, "compiledBinaryUnchanged": False,
               "startedUTC": datetime.now(timezone.utc).isoformat(), "phases": {},
               "metadataPhases": s.METADATA_RECEIPTS, "dependencyPhasesBefore": {}, "dependencyPhasesAfter": {},
               "dependenciesBefore": None, "dependenciesAfter": None,
               "compiledBinaryBefore": None, "compiledBinaryAfter": None,
               "sourceBefore": None, "sourceAfter": None, "inspectorSHA256": None,
               "limits": {"ownedSettlementReserveSeconds": 5, "captureBytes": 16777216, "lineBytes": 65536,
                          "privateResultBytes": 131072, "workers": 1, "retries": 0,
                          "unlockMilliseconds": 45000, "observationGraceMilliseconds": 10000},
               "pendingIsolation": "Published 2b816fa8 startup-corrected raw inputs; stock formatter qualification and hosted runtime isolation proof remain pending.",
               "coverageLimit": "Selected sources/executables/package roots only; no universal network, host, system-library or whole-environment absence claim."}
    result = {"schemaVersion": 1, "controls": None, "collection": None, "browser": None}
    source = {"schemaVersion": 1, "revision": receipt["revision"], "tree": None, "status": "unreached"}
    first, ledger, package, context = None, None, None, {}
    stage, exit_code, failure_reason = "checkout", 2, "none"
    names = ("R06_RESTORE_FIXTURE_BINARY", "R06_RESTORE_PRIVATE_ROOT", "R06_RESTORE_PRIVATE_OUTPUT",
             "R06_RESTORE_SAFE_RESULTS", "R06_RESTORE_REPORT_MODE")
    previous = {name: os.environ.get(name) for name in names}
    def launch(name, command, active, total, projection=None, cwd=s.APP):
        phase = {}; receipt["phases"][name] = phase
        p.execute(command, active, total, projection, phase, str(cwd))
        if not p.accepted(phase, (0, 1) if name == "browser" else (0,)):
            raise ValueError("owned-phase-incomplete")
        return phase
    try:
        first = s.identity()
        if first["revision"] != receipt["revision"]: raise ValueError("hosted-revision")
        receipt["tree"] = first["tree"]; source["tree"] = first["tree"]
        ledger = s.tracked_sources(deadline=time.monotonic() + 45)
        package = s.package_manifest()
        manifest_path = s.QA + "runtime-source-manifest.json"
        manifest_record = next(row for row in ledger["files"] if row["path"] == manifest_path)
        rows = [*package["sources"], *package["inputs"],
                {"path": manifest_path, "blob": manifest_record["gitBlob"],
                 "bytes": manifest_record["bytes"], "sha256": manifest_record["sha256"]}]
        if {row["path"] for row in rows} != s.REQUIRED_INPUTS: raise ValueError("input-scope")
        inputs = s.bind_inputs(ledger, rows)
        source = {"schemaVersion": 1, **first, "ledger": ledger, "inputs": inputs,
                  "runtimePackage": {"path": manifest_path, **inputs[manifest_path]}}
        receipt["sourceBefore"] = ledger["canonical"]
        receipt["inspectorSHA256"] = s.inspector_identity()["sha256"]
        stage = "dependencies"
        receipt["dependenciesBefore"] = t.capture(receipt["dependencyPhasesBefore"], suite, context)
        command = s.commands(suite, str(binary), str(work))
        go = str(context["paths"]["go"])
        command["compile"][0] = go; command["controls"][0] = go
        stage = "compile"
        launch("compile", command["compile"], 90, 95)
        receipt["compiledBinaryBefore"] = s.fingerprint(binary, executable=True, deadline=time.monotonic() + 30)
        os.environ["R06_RESTORE_PRIVATE_ROOT"] = str(private)
        stage = "public-controls"
        controls = c.ControlProjection()
        launch("publicControls", command["controls"], 60, 65, controls)
        result["controls"] = controls.result()
        if not result["controls"]["green"]: raise ValueError("public-controls-incomplete")
        if suite == "restore-controls":
            receipt["classification"], exit_code = "focused-restore-public-controls-green", 0
        else:
            os.environ["R06_RESTORE_FIXTURE_BINARY"] = str(binary)
            os.environ["R06_RESTORE_PRIVATE_OUTPUT"] = str(work / "playwright")
            os.environ["R06_RESTORE_SAFE_RESULTS"] = str(work / "collection-safe.json")
            os.environ["R06_RESTORE_REPORT_MODE"] = "collection"
            stage = "collection"
            collection_command = t.selected_cli(context["roots"], context["paths"]["node"], command["collection"][5:])
            launch("collection", collection_command, 15, 20, cwd=ROOT)
            result["collection"] = b.admit(s.strict_json(s.read_bytes(str(work / "collection-safe.json"), 131072), 131072),
                                           "collection", suite)
            os.environ["R06_RESTORE_SAFE_RESULTS"] = str(work / "runtime-safe.json")
            os.environ["R06_RESTORE_REPORT_MODE"] = "runtime"
            stage = "browser"
            browser_command = t.selected_cli(context["roots"], context["paths"]["node"], command["browser"][5:])
            phase = launch("browser", browser_command, 230, 235, cwd=ROOT)
            result["browser"] = b.admit(s.strict_json(s.read_bytes(str(work / "runtime-safe.json"), 131072), 131072),
                                        "runtime", suite)
            classification = b.classify(result["browser"], receipt["inspectorSHA256"])
            expected_exit = 0 if classification == "focused-restore-browser-green" else 1 if classification == "confirmed-restore-deadline-red" else 2
            if phase["exitCode"] != expected_exit: raise ValueError("runner-classification")
            receipt["classification"], exit_code = classification, expected_exit
        stage = "integrity"
    except Exception as error:
        receipt["failure"] = {"stage": stage, "class": a.safe_failure_class(error)}
        failure_reason = t.dependency_reason(error) if stage == "dependencies" else "unclassified"
    finally:
        for name, value in previous.items():
            if value is None: os.environ.pop(name, None)
            else: os.environ[name] = value
        # Every selected closure is independently checked, including when another check fails.
        try:
            final = s.identity()
            after = s.tracked_sources(deadline=time.monotonic() + 45)
            receipt["sourceAfter"] = after["canonical"]
            receipt["sourceUnchanged"] = first is not None and final == first and after == ledger
        except Exception: receipt["sourceUnchanged"] = False
        try:
            receipt["dependenciesAfter"] = t.capture(receipt["dependencyPhasesAfter"], suite)
            receipt["dependenciesUnchanged"] = t.same_state(receipt["dependenciesBefore"], receipt["dependenciesAfter"], suite)
        except Exception: receipt["dependenciesUnchanged"] = False
        try:
            receipt["compiledBinaryAfter"] = s.fingerprint(binary, executable=True, deadline=time.monotonic() + 30)
            receipt["compiledBinaryUnchanged"] = receipt["compiledBinaryBefore"] is not None and receipt["compiledBinaryAfter"] == receipt["compiledBinaryBefore"]
        except Exception: receipt["compiledBinaryUnchanged"] = False
        if not all(receipt[key] for key in ("sourceUnchanged", "dependenciesUnchanged", "compiledBinaryUnchanged")):
            receipt["classification"], exit_code = "prerequisite-blocked", 2
    receipt["endedUTC"] = datetime.now(timezone.utc).isoformat()
    bodies = a.seal({"receipt.json": receipt, "results.json": result, "source-manifest.json": source})
    try: a.admit_bundle(bodies, receipt["revision"], receipt["tree"])
    except Exception:
        receipt["classification"], exit_code = "prerequisite-blocked", 2
        receipt["failure"] = {"stage": "artifact-admission", "class": "ValueError"}
        bodies = a.seal({"receipt.json": receipt, "results.json": result, "source-manifest.json": source})
        a.admit_bundle(bodies, receipt["revision"], receipt["tree"])
    a.write_bundle(output, bodies)
    print(json.dumps({"id": "R06", "suite": suite, "classification": receipt["classification"], "failureReason": failure_reason}))
    return exit_code


if __name__ == "__main__":
    try: sys.exit(main())
    except Exception as error:
        print(json.dumps({"id": "R06", "classification": "prerequisite-blocked", "class": type(error).__name__}))
        sys.exit(2)
