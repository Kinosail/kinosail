import json
import pathlib
import sys

contract = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert set(contract) == {"schemaVersion", "scope", "product", "defaults", "environment", "experience", "mediaAccess", "update", "platforms"}
assert contract["schemaVersion"] == 3 and contract["scope"] == "machine"
assert contract["product"] == {
    "name": "Kinosail Server",
    "packageId": "com.kinosail.server",
    "singleInstance": True,
    "installModes": ["install", "upgrade", "repair", "uninstall"],
}
assert contract["defaults"] == {
    "bind": "127.0.0.1",
    "port": 38127,
    "openFirewall": False,
    "automaticUpdates": True,
    "rebootRequired": False,
    "preserveDataOnUninstall": True,
}
assert contract["environment"] == {
    "media": "KINOSAIL_MEDIA_DIR",
    "data": "KINOSAIL_DATA_DIR",
    "cache": "KINOSAIL_CACHE_DIR",
    "backups": "KINOSAIL_BACKUP_DIR",
    "ffmpeg": "KINOSAIL_FFMPEG",
    "ffprobe": "KINOSAIL_FFPROBE",
    "fpcalc": "KINOSAIL_FPCALC",
}
assert contract["experience"] == {
    "wizardSteps": ["media", "updates", "review", "install", "owner-setup"],
    "advancedOptions": ["port"],
    "openOwnerSetup": True,
    "repair": True,
    "unattended": True,
}
assert contract["mediaAccess"] == {
    "mode": "selected-folders-read-only",
    "changeOwnership": False,
    "removeGrantedAccessOnUninstall": True,
}
update = contract["update"]
assert set(update) == {
    "schemaVersion", "manifest", "signatureBundle", "signatureIdentity", "agent",
    "recovery", "compatibility", "preflight", "health", "transaction", "phases",
    "errorCodes", "steps", "terminalStates",
}
assert update["schemaVersion"] == 2
assert update["manifest"] == "kinosail-release.json"
assert update["signatureBundle"] == "kinosail-release.json.sigstore.json"
assert update["signatureIdentity"] == r"^https://github\.com/MikeO7/kinosail/\.github/workflows/player-release\.yml@refs/tags/player-v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$"
assert update["terminalStates"] == ["current", "rolled-back"]
assert update["agent"] == {
    "schemaVersion": 1,
    "installWithServer": True,
    "removeWithServer": True,
    "singleInstance": True,
    "runAs": "system",
    "command": ["run-once"],
    "planCommand": ["kinosail", "update-plan"],
    "reportCommand": ["kinosail", "update-report"],
    "schedule": {
        "afterStartupSeconds": 120,
        "intervalSeconds": 60,
        "jitterSeconds": 15,
    },
    "networkWithoutRequest": False,
    "requiresPinnedTarget": True,
    "manualAndAutomaticSamePath": True,
}
assert update["recovery"] == {
    "backupCommand": ["backup"],
    "verifyCommand": ["backup", "verify"],
    "restoreCommand": ["restore"],
    "format": "kinosail-backup-v1-encrypted",
    "includesSecrets": True,
    "requiresInstallationKey": True,
    "restoreOnRollback": True,
}
assert update["compatibility"] == {
    "stateSchema": ["minimumStateSchema", "stateSchema"],
    "configurationSchema": ["minimumConfigurationSchema", "configurationSchema"],
}
assert update["preflight"] == [
    "supported-platform", "supported-architecture", "writable-data", "writable-cache",
    "readable-media", "available-port", "runtime-commands", "free-space",
]
assert update["health"] == {"command": ["healthcheck"], "timeoutSeconds": 20}
assert update["transaction"] == {
    "schemaVersion": 1,
    "maximumBytes": 16384,
    "permissions": "owner-read-write",
    "write": "atomic-replace-and-sync",
    "requiredFields": [
        "schemaVersion", "requestId", "targetVersion", "previousVersion", "phase",
        "startedAt", "updatedAt",
    ],
    "optionalFields": ["backupPath", "artifactPath", "errorCode"],
    "crashRecovery": [
        "read-journal", "inspect-installed-version", "health-check",
        "complete-target-if-healthy", "resume-before-replace", "restore-backup-otherwise",
    ],
    "clearAfter": ["current", "rolled-back"],
}
assert update["phases"] == [
    "waiting-for-idle", "backing-up", "verifying-backup", "downloading", "verifying-release",
    "stopping", "replacing", "starting", "checking-health", "restoring-backup",
]
assert update["errorCodes"] == [
    "download-failed", "signature-invalid", "digest-mismatch", "backup-failed",
    "service-stop-failed", "install-failed", "service-start-failed", "health-check-failed",
    "restore-failed", "rollback-failed", "insufficient-space", "permission-denied",
]
assert update["steps"] == [
    "defer-playback", "backup", "verify-backup", "verify-signature", "verify-digest", "stop",
    "replace", "start", "health-check", "restore-backup-on-failure",
]
platforms = contract["platforms"]
assert [item["os"] for item in platforms] == ["linux", "macos", "windows"]
required = {"os", "packageId", "service", "serviceAccount", "executable", "runtime", "configuration", "data", "cache", "backups", "updateAgent", "updateScheduler", "updateTask", "updateState", "installerLog", "logs"}
for platform in platforms:
    expected = required | ({"upgradeCode"} if platform["os"] == "windows" else set())
    assert set(platform) == expected
    assert all(isinstance(platform[key], str) and platform[key] for key in required)
    assert len({platform["executable"], platform["runtime"], platform["configuration"], platform["data"], platform["cache"], platform["backups"]}) == 6
assert platforms[2]["upgradeCode"] == "{D3D7B6C9-7F51-4F64-AD7C-CFA9B5A9A0E8}"
assert [(item["updateScheduler"], item["updateTask"], item["updateAgent"]) for item in platforms] == [
    ("systemd-timer", "kinosail-update.timer", "/usr/lib/kinosail/kinosail-updater"),
    ("launchd", "com.kinosail.update", "/Library/Application Support/Kinosail/bin/kinosail-updater"),
    ("task-scheduler", "Kinosail Update", "%ProgramFiles%\\Kinosail\\kinosail-updater.exe"),
]
assert "channel" not in json.dumps(contract).lower()
