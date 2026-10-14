"""Temporary, bounded system lifecycle observations for the owned Wear emulator."""
import re
import subprocess

SCHEMA = {
    "wm_create_activity": ("create", 8, 3, None),
    "wm_new_intent": ("new-intent", 8, 3, None),
    "wm_finish_activity": ("finish", 5, 3, 4),
    "wm_resume_activity": ("resume", 4, 3, None),
    "wm_pause_activity": ("pause", 5, 2, 4),
    "wm_set_resumed_activity": ("set-resumed", 3, 1, 2),
    "wm_stop_activity": ("stop", 3, 2, None),
}
REASONS = {"resumeTopActivity", "realStart", "sleep", "pauseBackTasks", "finishActivity",
           "finish-imm", "finish-top", "completePause", "startActivity", "moveTaskToFront"}
LIMIT = 1 << 20
PACKAGE_ROLES = {
    "com.kinosail.player.dev": "target-package",
    "com.kinosail.player.dev.test": "instrumentation-package",
    "com.google.android.wearable.app": "wearable-app",
    "com.google.android.wearable.sysui": "wearable-system-ui",
    "com.google.android.wearable.setupwizard": "wearable-setup",
    "com.google.android.gms": "play-services",
    "com.android.systemui": "system-ui",
    "com.android.settings": "system-settings",
    "android": "android-system",
}


def integer(value):
    return bool(re.fullmatch(r"-?[0-9]{1,10}", value)) and -(1 << 31) <= int(value) < 1 << 31


def component(value):
    if not re.fullmatch(r"[A-Za-z0-9_.$]{1,200}/[A-Za-z0-9_.$]{1,200}", value):
        return None
    package, name = value.split("/")
    return package + "/" + (package + name if name.startswith(".") else name)


def project(data, home_output):
    result = {"events": [], "matchedEvents": 0, "malformedEvents": 0,
              "inputTruncated": len(data) > LIMIT, "eventsTruncated": False,
              "tailLimitReached": len(data.splitlines()) >= 1000}
    if result["inputTruncated"]:
        return result
    home_match = re.search(rb"(?m)^Activity: ([A-Za-z0-9_.$/]{1,401})\r?$", home_output[:4096])
    home = component(home_match[1].decode()) if home_match else None
    for line in data.decode("utf-8", errors="replace").splitlines():
        match = re.fullmatch(r"I/(wm_[a-z_]+)\(\s*\d+\): \[(.*)\]", line)
        if not match or match[1] not in SCHEMA:
            continue
        name, count, index, reason_index = SCHEMA[match[1]]
        fields = match[2].split(",")
        target = component(fields[index]) if len(fields) == count else None
        numbers = fields[:index] + (fields[7:] if count == 8 else [])
        if (target is None or not all(integer(value) for value in numbers)
                or (name == "pause" and fields[3] not in ("userLeaving=true", "userLeaving=false"))):
            result["malformedEvents"] += 1
            continue
        role = "home" if home is not None and target == home else "other"
        if target == "com.kinosail.player.dev/com.kinosail.player.wear.WearActivity":
            role = "wear"
        elif target == "com.kinosail.player.dev/androidx.activity.ComponentActivity":
            role = "fixture"
        elif target in {p + "/androidx.test.core.app.InstrumentationActivityInvoker$" + activity
                        for p in ("com.kinosail.player.dev", "com.kinosail.player.dev.test")
                        for activity in ("EmptyActivity", "EmptyFloatingActivity", "BootstrapActivity")}:
            role = "test-driver"
        if role == "other":
            package, activity = target.split("/")
            role = PACKAGE_ROLES.get(package, "other")
            if home is not None and package == home.split("/")[0]:
                role = "home-package"
            if activity == "android.service.dreams.DreamActivity":
                role = "system-dream"
        event = {"event": name, "role": role}
        if reason_index is not None:
            event["reason"] = fields[reason_index] if fields[reason_index] in REASONS else "other"
        if name == "pause":
            event["userLeaving"] = fields[3] == "userLeaving=true"
        result["matchedEvents"] += 1
        if len(result["events"]) < 64:
            result["events"].append(event)
        else:
            result["eventsTruncated"] = True
    return result


def reset_events(adb):
    try:
        adb("logcat", "-b", "events", "-c", timeout=10)
        return True
    except (OSError, subprocess.SubprocessError):
        return False


def capture_events(adb, directory, home):
    try:
        data = adb("logcat", "-b", "events", "-d", "-v", "brief", "-t", "1000", "*:V", timeout=10)
        (directory / "system-events.private.log").write_bytes(data[:LIMIT])
        return project(data, home)
    except (OSError, subprocess.SubprocessError):
        return {"unavailable": True}
