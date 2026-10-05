#!/usr/bin/env python3
"""Hosted real Save fault proof, retaining four safe JSON artifacts only."""
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import re
import shutil
import stat
import sys
import subprocess

from campaign_r06_browser_process import execute, complete_process
from campaign_r06_browser_projection import admit_projection, classify
from campaign_r06_browser_sources import (APP, FIXTURE, GO_CASES, OUTPUT, PACKAGE, ROOT,
                                         GoProjection, fingerprint, identity, inspector_identity, pin, private_result, tracked_sources)

SAFE_NAMES = ("receipt.json","results.json","source-manifest.json","artifact-manifest.json")
TOOL_BYTE_LIMITS = {"go":64*1024*1024,"node":192*1024*1024,"pnpm":64*1024*1024}
SUITES = {
    "save-headers":("headers",["save-headers-desktop","save-headers-phone"],"^R06 Save headers held after completed write - (phone|desktop)$"),
    "save-body":("body",["save-body-desktop","save-body-phone"],"^R06 Save body held after completed write - (phone|desktop)$"),
}


def settled(phase, exits):
    return (phase.get("exitCode") in exits and not phase.get("stopReason") and
            phase.get("ownedProcessExited") and phase.get("ownedGroupSettled") and phase.get("captureSettled"))


def tools_state(preflight=None):
    state = {}
    for name in ("go","node","pnpm"):
        found = shutil.which(name)
        if preflight is not None:
            preflight[name] = {"available":found is not None}
        if found is None: raise ValueError("tool-unavailable")
        path = Path(found).resolve(strict=True)
        if preflight is not None:
            info = path.lstat()
            preflight[name].update(bytes=info.st_size,mode=info.st_mode & 0o777,
                                   regular=stat.S_ISREG(info.st_mode))
        state[name] = fingerprint(path,TOOL_BYTE_LIMITS[name],True)
    return state


def launch(command, seconds, receipt, name, projection=None, cwd=APP):
    phase = {"boundSeconds":seconds,"processLaunched":False}
    receipt["phases"][name] = phase
    execute(command,seconds,projection,phase,cwd=cwd)
    return phase


def sources_equal(initial, pins):
    return identity() == initial and tracked_sources() == pins


def playwright_command(*args):
    return ["pnpm","--dir",str(APP/"e2e"),"exec","playwright","test",
            "--config","playwright.r06-save.config.ts",*args]


def main():
    suite = os.environ.get("CAMPAIGN_R06_SUITE")
    if (sys.argv[1:] or suite not in SUITES or os.environ.get("GITHUB_ACTIONS") != "true"
            or os.environ.get("RUNNER_OS") != "Linux"):
        return 2
    os.umask(0o077)
    if OUTPUT.exists() and any(OUTPUT.iterdir()): return 2
    OUTPUT.mkdir(parents=True,exist_ok=True)
    receipt = {"schemaVersion":1,"id":"R06","mode":"save-browser","suite":suite,
               "classification":"prerequisite-blocked","sourceUnchanged":False,
               "startedUTC":datetime.now(timezone.utc).isoformat(),"phases":{},
               "limits":{"compileSeconds":95,"controlGoSeconds":40,"controlExternalSeconds":45,
                         "collectionSeconds":20,"browserGlobalSeconds":150,"browserExternalSeconds":155,
                         "caseSeconds":70,"saveUnlockMilliseconds":45000,"observationGraceMilliseconds":10000,
                         "workers":1,"retries":0,"leaderReapSeconds":3,"groupSettlementSeconds":4,
                         "captureBytes":16777216,"lineBytes":65536,"privateResultBytes":131072},
               "boundary":"Save-only real fixture; no decoded media, device, Audio/Restore/Generate or ready-draft acceptance.",
               "browserExecutableBytesPinned":False,
               "browserBinaryLimit":"Go fixture and go/node/pnpm are pinned; Chromium executable bytes are not independently pinned."}
    results = {"schemaVersion":1,"controls":None,"collection":None,"browser":None}
    sources = {"schemaVersion":1,"status":"unreached"}
    initial, binary_before, tools_before, stage = None, None, None, "checkout"
    exit_code = 2
    environment_names = ("R06_SAVE_FIXTURE_BINARY","R06_SAVE_PRIVATE_ROOT","R06_SAVE_PRIVATE_OUTPUT",
                         "R06_SAVE_SAFE_RESULTS","R06_SAVE_REPORT_MODE")
    previous = {name:os.environ.get(name) for name in environment_names}
    try:
        initial = identity()
        if os.environ.get("GITHUB_SHA") != initial["revision"]: raise ValueError("hosted-revision")
        sources = tracked_sources()
        receipt.update(checkout=initial,selectedSource=sources["canonical"])
        stage = "toolchain"
        receipt["toolchainPreflight"] = {}
        receipt["toolchain"] = tools_state(receipt["toolchainPreflight"])
        tools_before = receipt["toolchain"]
        work = OUTPUT/"work"
        work.mkdir(mode=0o700)
        private_root = work/"private"
        private_root.mkdir(mode=0o700)
        binary = work/"save-fixture.test"
        os.environ["R06_SAVE_PRIVATE_ROOT"] = str(private_root)
        stage = "compile"
        compilation = launch(["go","test","-c","-p","1","-o",str(binary),"./"+str(FIXTURE.relative_to(APP))],
                             95,receipt,"compile")
        if not complete_process(compilation): raise ValueError("compile-incomplete")
        binary.chmod(0o700)
        binary_before = fingerprint(binary,128*1024*1024,True)
        receipt["compiledBinaryBefore"] = binary_before
        if not sources_equal(initial,sources): raise ValueError("compile-source-drift")
        stage = "public-controls"
        controls = GoProjection()
        command = ["go","tool","test2json","-t","-p",PACKAGE,str(binary),"-test.v=test2json",
                   "-test.run=^("+ "|".join(GO_CASES)+")$","-test.parallel=1","-test.count=1","-test.timeout=40s"]
        control_phase = launch(command,45,receipt,"publicControls",controls,cwd=FIXTURE)
        results["controls"] = controls.result()
        if not complete_process(control_phase) or not results["controls"]["green"]:
            raise ValueError("public-controls-incomplete")
        if not sources_equal(initial,sources) or fingerprint(binary,128*1024*1024,True) != binary_before:
            raise ValueError("control-source-drift")
        os.environ["R06_SAVE_FIXTURE_BINARY"] = str(binary)
        os.environ["R06_SAVE_PRIVATE_OUTPUT"] = str(work/"playwright")
        os.environ["R06_SAVE_SAFE_RESULTS"] = str(work/"collection-safe.json")
        os.environ["R06_SAVE_REPORT_MODE"] = "collection"
        stage = "collection"
        collection_phase = launch(playwright_command("--list","--global-timeout=15000"),20,receipt,"collection",cwd=ROOT)
        if not complete_process(collection_phase): raise ValueError("collection-incomplete")
        results["collection"] = admit_projection(private_result(work/"collection-safe.json"),"collection",SUITES[suite][1])
        os.environ["R06_SAVE_SAFE_RESULTS"] = str(work/"runtime-safe.json")
        os.environ["R06_SAVE_REPORT_MODE"] = "runtime"
        stage = "browser"
        runtime_phase = launch(playwright_command("--grep",SUITES[suite][2],"--global-timeout=150000"),
                               155,receipt,"browser",cwd=ROOT)
        if not settled(runtime_phase,(0,1)): raise ValueError("browser-incomplete")
        results["browser"] = admit_projection(private_result(work/"runtime-safe.json"),"runtime",SUITES[suite][1])
        receipt["servedAssetExpected"] = {"inspector":inspector_identity()}
        expected_inspector = receipt["servedAssetExpected"]["inspector"]["sha256"]
        classified = classify(results["browser"],expected_inspector)
        if (classified == "focused-save-browser-green" and runtime_phase["exitCode"] != 0 or
                classified == "confirmed-save-deadline-red" and runtime_phase["exitCode"] != 1):
            raise ValueError("runner-classification")
        receipt["classification"] = classified
        exit_code = 0 if classified == "focused-save-browser-green" else 1 if classified == "confirmed-save-deadline-red" else 2
        stage = "integrity"
    except (OSError,ValueError,RuntimeError,KeyError,TypeError,subprocess.SubprocessError) as error:
        receipt["failure"] = {"stage":stage,"class":type(error).__name__}
    finally:
        for name, value in previous.items():
            if value is None: os.environ.pop(name,None)
            else: os.environ[name] = value
        try:
            receipt["sourceUnchanged"] = initial is not None and sources_equal(initial,sources)
            receipt["toolchainUnchanged"] = tools_before is not None and tools_state() == tools_before
            receipt["compiledBinaryUnchanged"] = binary_before is not None and fingerprint(binary,128*1024*1024,True) == binary_before
        except (OSError,ValueError,RuntimeError,KeyError,subprocess.SubprocessError):
            receipt["sourceUnchanged"] = False
        if not all(receipt.get(key) for key in ("sourceUnchanged","toolchainUnchanged","compiledBinaryUnchanged")):
            receipt["classification"], exit_code = "prerequisite-blocked", 2
    receipt["endedUTC"] = datetime.now(timezone.utc).isoformat()
    bodies = {name:(json.dumps(value,sort_keys=True,indent=2)+"\n").encode() for name,value in
              (("receipt.json",receipt),("results.json",results),("source-manifest.json",sources))}
    bodies["artifact-manifest.json"] = (json.dumps({name:pin(body) for name,body in bodies.items()},
                                                  sort_keys=True,indent=2)+"\n").encode()
    if any(len(body)>4*1024*1024 for body in bodies.values()): return 2
    for name in SAFE_NAMES:
        with (OUTPUT/name).open("xb") as stream: stream.write(bodies[name])
    print(json.dumps({"id":"R06","suite":suite,"classification":receipt["classification"]}))
    return exit_code


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError,ValueError,RuntimeError,subprocess.SubprocessError) as error:
        print(json.dumps({"id":"R06","classification":"prerequisite-blocked","class":type(error).__name__}))
        sys.exit(2)
