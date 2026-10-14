"""Bounded, private Home readiness checks for disposable Wear emulators."""
import re
import subprocess
import time

LIMIT = 1 << 20


def component(value):
    if not re.fullmatch(r"[A-Za-z0-9_.$]{1,200}/[A-Za-z0-9_.$]{1,200}", value):
        return None
    package, name = value.split("/")
    return package + "/" + (package + name if name.startswith(".") else name)


def home_component(output):
    match = re.search(rb"(?m)^Activity: ([A-Za-z0-9_.$/]{1,401})\r?$", output[:4096])
    return component(match[1].decode()) if match else None


def home_state(data, home_output):
    entry = home_component(home_output)
    state = {"entrypointKnown": entry is not None, "topResumedAvailable": False,
             "homePackageMatches": False, "followOnResumed": False,
             "truncated": len(data) > LIMIT, "ambiguous": False}
    if state["truncated"]:
        return state
    lines = [line for line in data.decode("utf-8", errors="replace").splitlines()
             if line.lstrip().startswith("ResumedActivity:")]
    state["ambiguous"] = len(lines) > 1
    if len(lines) != 1:
        return state
    match = re.fullmatch(r"\s*ResumedActivity: ActivityRecord\{[0-9a-f]+ u[0-9]+ "
                         r"([A-Za-z0-9_.$/]{1,401}) t-?[0-9]+(?: f)?\}", lines[0])
    current = component(match[1]) if match else None
    state["topResumedAvailable"] = current is not None
    if current is not None and entry is not None:
        state["homePackageMatches"] = current.split("/")[0] == entry.split("/")[0]
        state["followOnResumed"] = state["homePackageMatches"] and current != entry
    return state


def wait_for_home(adb, directory, home, case):
    observation = case["homeReadiness"] = {"attempts": 0}
    started = time.monotonic()
    while (remaining := 30 - (time.monotonic() - started)) > 0:
        observation["attempts"] += 1
        try:
            data = adb("shell", "dumpsys", "activity", "activities", timeout=min(2, remaining))
            (directory / "home-readiness.private.log").write_bytes(data[:LIMIT])
            observation.update(home_state(data, home), unavailable=False)
        except (OSError, subprocess.SubprocessError):
            observation.update(unavailable=True, followOnResumed=False)
        observation["seconds"] = round(time.monotonic() - started, 3)
        if observation.get("followOnResumed"):
            return
        time.sleep(min(.2, max(0, 30 - (time.monotonic() - started))))
    raise RuntimeError("wear_home_follow_on_not_resumed")
