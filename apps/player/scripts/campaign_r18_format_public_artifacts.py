"""Closed formatter artifacts only; never authorize canonical adoption."""
import base64
from datetime import datetime, timedelta
import hashlib
import json
from pathlib import PurePosixPath
import re

from campaign_r18_document_format_process import FIELDS, FAILURES, valid_packet
from campaign_r18_format_public_files import CAPS, NAMES, publish

REQUIRED = {'apps/player/scripts/campaign_r18_document_events.py': ('8fa40854f7e5e55bdd31eb1bb23ad1c2c903cf17', 'da42ba31d5e7334e065bb66822d139720dd6d52b1cd887cabe12016cbeaa0a57', 7028), 'apps/player/scripts/campaign_r18_document_admission.py': ('8dcfaed7534a87dbd147740e16f9015772a4f6d9', '598ffa43a1a9d62388c6048e822f0caa5b7af1e871ad71b7f56d4b3f1144182b', 4096), 'apps/player/scripts/campaign_r18_document_sources.py': ('a73b0940fe148d217ec407ec54625528f4aeea83', 'cf304f4bdbdee2fc219fb279dd67bf34c031823a71a9d2940ed2ca8836b5b140', 9032), 'apps/player/scripts/campaign-r18-document-contract.py': ('cd3994a3b83677d84f81e708c32d081267154570', '257736615cfca467874775c6b562f5059bb797c9848945e8ab6d4911415b8ca4', 11663), 'apps/player/scripts/campaign_r18_document_inputs.py': ('5e1c5e36abc4ec60fe13030e8a4bce3fc35f6672', 'c64cfa8f8b85320c3d110c83ca3e7c8745f1ee894eb471fd1276829e0381ae98', 6702), 'apps/subtitles/scripts/campaign_r06_browser_process.py': ('c88075fb514f93209796e79d6dd0bb37b079e7e7', '45aa8fe859bde5c0c978589d220e938d6e341e05da5fd6e004c583ada5b4462e', 6226), 'apps/subtitles/scripts/campaign_r06_sources.py': ('7bb611975c0e42f65780f73d982ca5a35c70f019', '6c20ed559d58d13b8bbe9a0cd8e1306b60b05dcd79897cd94b8a92af617172d8', 4705), 'apps/player/scripts/campaign_r18_document_format_process.py': ('d63e5f4179261ef47e06406288a70b4a3231b655', '46e2f837ade76fa1ab005a3454ce85edd94b3c4479474518af15bdb64eb1dbf6', 10254), 'apps/player/scripts/campaign_r18_document_format.py': ('4b62fed470e3eb8de54e74027260b4147458d3bd', 'f19f5edb3d45671e6e4504d24d8f96436772e95a59f96373a3d03dacfb34fe53', 13771), 'apps/player/internal/server/home_assistant_document_targets_test.go': ('69f15a59a3291dbedcef0df575f851e4c95c4d50', '18ab485d6f82196554b598152a15984f44db06cc2620e5cd04f17b2b9bccb0c5', 4465), 'apps/player/internal/server/home_assistant_document_targets_helpers_test.go': ('01f262d1b308736172f33d751e7c17a3e3efda6b', 'cfe4c812a3554beec7e15861d4518be4acedca15023a305bb5237482f83ad49c', 7997), 'packages/homeassistant/http.go': ('245a18cc5ddd2ad0ad921dd0dd6ca03f1407633e', '803124e3a4b4f54b3105dc09d6bc90f524f8bb0626eee0bfc561453e03be059a', 11877), 'packages/homeassistant/player.go': ('ceb2130f568d0c5300f4dd2c2c5dc8868efbf076', '9cd2867052ccf2b6a3dfb240c3c9542c3e3ee77dabaae70f5a6b18ef7cc64374', 4548), 'packages/homeassistant/integration.go': ('11598c3bd1360b71d19a7d662353fdffbe500f20', 'a77a8b85a5daceda142900390bb8146c7af33a5328bc072e86d86a71a56ce2b0', 3849), 'apps/player/.golangci.yml': ('d63849acf919209a9da64cdc687d6d63f073f01c', '389f5d87f20f6ea354dc36be94150e4057a09e1fd31a5cfb6aa14de96953eafd', 1054)}
PURPOSES = ("history-shallow", "history-anchor", "history-ancestor", "hash", "silent", "silent", "files",
            "format-source-0", "format-source-1", "hash", "silent", "silent", "files")
PROCESS = {"purpose", "exitCode", "ownedProcessExited", "ownedGroupSettled", "captureSettled", "processLaunched",
           "seconds", "stopReason", "captureBytes", "captureLines", "captureSHA256"}
RECEIPT = {"schemaVersion", "phase", "expectedRevision", "baseRevision", "baseTree", "invocation", "startedUTC",
           "finishedUTC", "classification", "autoAdoption", "semanticsVerified", "processes", "formatterPackets",
           "historyKind", "beforeState", "afterState", "formatterBefore", "formatterAfter", "count",
           "identity", "seconds", "failureClass"}
HISTORY = {"history-admitted", "history-shallow", "history-anchor-unavailable", "history-not-proven-ancestor", "history-incomplete"}
STOP = {None, "interrupted", "external-timeout", "prerequisite-failure", "output-bound",
        "process-boundary-failure", "process-settlement-failure"}
CLASSES = {"OSError", "ValueError", "KeyError", "ImportError", "TypeError", "KeyboardInterrupt"}

def require(condition):
    if not condition:
        raise ValueError("artifact-admission")

def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":")).encode()).hexdigest()

def pairs(items):
    out = {}
    for key, value in items:
        require(key not in out)
        out[key] = value
    return out

def constant(_value):
    raise ValueError("artifact-nonfinite")

def bounded_json_nesting(raw):
    # Bound parsing independently of the interpreter recursion limit.
    depth, quoted, escaped = 0, False, False
    for byte in raw:
        if quoted:
            if escaped:
                escaped = False
            elif byte == 92:
                escaped = True
            elif byte == 34:
                quoted = False
        elif byte == 34:
            quoted = True
        elif byte in (91, 123):
            depth += 1
            require(depth <= 32)
        elif byte in (93, 125):
            depth -= 1
            require(depth >= 0)
    require(depth == 0 and not quoted)

def safe_json(raw):
    require(type(raw) is bytes and 0 < len(raw) <= 16 * 1024 * 1024)
    bounded_json_nesting(raw)
    try:
        return json.loads(raw.decode("utf-8"), object_pairs_hook=pairs, parse_constant=constant)
    except (ValueError, UnicodeError, RecursionError):
        raise ValueError("artifact-json") from None

def integer(value, low, high):
    return type(value) is int and low <= value <= high

def sha(value, count=64):
    return type(value) is str and re.fullmatch(r"[a-f0-9]{" + str(count) + "}", value) is not None

def seconds(value):
    return type(value) in (int, float) and 0 <= value <= 90

def path(value):
    return (type(value) is str and 0 < len(value) <= 512 and not value.startswith("/")
            and not any(c in value for c in ("\0", "\n", "\r", "\t", "\\", ":"))
            and value != "." and ".." not in PurePosixPath(value).parts and PurePosixPath(value).as_posix() == value)

def metadata(row):
    return (type(row) is dict and set(row) == {"sha256", "gitBlob", "bytes", "mode"}
            and sha(row["sha256"]) and sha(row["gitBlob"], 40) and integer(row["bytes"], 0, 128 * 1024 * 1024)
            and integer(row["mode"], 0, 0o777))

def state(value, revision):
    return (type(value) is dict and set(value) == {"revision", "tree", "sourceSHA256", "toolchainSHA256", "inputCount"}
            and value["revision"] == revision and sha(value["tree"], 40)
            and sha(value["sourceSHA256"]) and sha(value["toolchainSHA256"]) and integer(value["inputCount"], 1, 20000))

def inventory(rows, field, maximum, per_file):
    require(type(rows) is list and len(rows) <= maximum)
    names, total = set(), 0
    for row in rows:
        require(type(row) is dict and set(row) == {field, "sha256", "gitBlob", "bytes", "mode"})
        name = row[field]
        require(path(name) and name not in names and metadata({k: v for k, v in row.items() if k != field}))
        require(row["bytes"] <= per_file)
        names.add(name)
        total += row["bytes"]
        require(total <= 512 * 1024 * 1024)
    return names

def source_manifest(value, revision):
    require(type(value) is dict and set(value) == {"schemaVersion", "state", "trackedInputs", "tools"} and type(value["schemaVersion"]) is int and value["schemaVersion"] == 1)
    rows, tools = value["trackedInputs"], value["tools"]
    names = inventory(rows, "path", 20000, 16 * 1024 * 1024)
    tool_names = inventory(tools, "name", 128, 128 * 1024 * 1024)
    if rows:
        lookup = {row["path"]: (row["gitBlob"], row["sha256"], row["bytes"]) for row in rows}
        require(all(lookup.get(name) == pin for name, pin in REQUIRED.items()))
    if value["state"] is not None:
        require(state(value["state"], revision) and value["state"]["inputCount"] == len(rows))
        require(value["state"]["sourceSHA256"] == digest(rows) and value["state"]["toolchainSHA256"] == digest(tools))
        require({"bin/go", "bin/gofmt", "VERSION", "python", "golangci-lint"} <= tool_names and any(name.endswith("/compile") for name in tool_names))
    return rows, tools

def process(value, purpose=None):
    require(type(value) is dict and set(value) <= PROCESS and {"exitCode", "ownedProcessExited", "ownedGroupSettled", "captureSettled", "seconds", "stopReason"} <= set(value))
    require(value["exitCode"] is None or integer(value["exitCode"], -255, 255))
    require(value["stopReason"] in STOP and seconds(value["seconds"]))
    for name in ("ownedProcessExited", "ownedGroupSettled", "captureSettled", "processLaunched"):
        require(name not in value or type(value[name]) is bool)
    if purpose is not None:
        require(value.get("purpose") == purpose)
        counter = "captureBytes" if purpose.startswith("history-") else "captureLines"
        if purpose not in ("format-source-0", "format-source-1"):
            require(counter in value and integer(value[counter], 0, 20000) and sha(value.get("captureSHA256")))
        require("captureBytes" not in value or purpose.startswith("history-"))
        require("captureLines" not in value or purpose in ("hash", "silent", "files"))

def proposal(row, formatter):
    require(type(row) is dict and set(row) == formatter.RECORD_FIELDS and row["path"] in formatter.GO_FILES)
    index = formatter.GO_FILES.index(row["path"])
    original = {"sha256": formatter.SOURCE_SHA[index], "bytes": formatter.SOURCE_BYTES[index]}
    require(row["original"] == original and type(row["formattedSourceBase64"]) is str and len(row["formattedSourceBase64"]) <= 44000)
    data = base64.b64decode(row["formattedSourceBase64"], validate=True)
    data.decode("utf-8")
    require(0 < len(data) <= 32768 and b"\0" not in data and data.startswith(b"package server_test\n") and len(data.splitlines()) <= 300)
    require(row["formatted"] == {"sha256": hashlib.sha256(data).hexdigest(), "bytes": len(data)})
    require(type(row["changed"]) is bool and row["changed"] == (row["original"] != row["formatted"]))
    require(row["autoAdoption"] is False and row["semanticsVerified"] is False)

def packet(value, index, records, formatter):
    require(type(value) is dict and set(value) == FIELDS and value["outputBase64"] is None)
    record = next((row for row in records if row["path"] == formatter.GO_FILES[index]), None)
    if value["failureKind"] is not None:
        require(valid_packet(value, index))
    elif record is not None:
        require(valid_packet(value | {"outputBase64": record["formattedSourceBase64"]}, index))
    else:
        require(value["schemaVersion"] == 1 and type(value["schemaVersion"]) is int and value["index"] == index and type(value["index"]) is int)
        require(type(value["formatterExitCode"]) is int and value["formatterExitCode"] == 0 and value["timedOut"] is False)
        require(all(value[k] is True for k in ("formatterExited", "captureSettled", "sourceUnchanged", "toolUnchanged")))
        require(integer(value["stdoutBytes"], 1, 32768) and sha(value["stdoutSHA256"]))
        require(type(value["stderrBytes"]) is int and value["stderrBytes"] == 0 and value["stderrSHA256"] == hashlib.sha256(b"").hexdigest())

def receipt(value, revision, manifest, records, formatter):
    needed = {"schemaVersion", "phase", "expectedRevision", "baseRevision", "baseTree", "invocation",
              "startedUTC", "finishedUTC", "classification", "autoAdoption", "semanticsVerified",
              "processes", "formatterPackets", "identity", "seconds"}
    require(type(value) is dict and needed <= set(value) <= RECEIPT)
    require(type(value["schemaVersion"]) is int and value["schemaVersion"] == 1 and value["phase"] == "canonical-format")
    require(value["expectedRevision"] == revision and value["baseRevision"] == formatter.BASE and value["baseTree"] == formatter.BASE_TREE)
    require(value["invocation"] == "golangci-fmt-stdin-player-two-v1" and value["autoAdoption"] is False and value["semanticsVerified"] is False)
    require(value["classification"] in ("canonical-source-proposals", "prerequisite-blocked") and seconds(value["seconds"]))
    for name in ("startedUTC", "finishedUTC"):
        require(type(value[name]) is str and len(value[name]) <= 40)
        require(datetime.fromisoformat(value[name]).utcoffset() == timedelta(0))
    require(type(value["identity"]) is dict and set(value["identity"]) == formatter.IDENTITY_FIELDS and all(type(v) is bool for v in value["identity"].values()))
    require("historyKind" not in value or value["historyKind"] in HISTORY)
    require("failureClass" not in value or value["failureClass"] in CLASSES)
    require("count" not in value or integer(value["count"], 0, 2))
    for name in ("beforeState", "afterState"):
        require(name not in value or state(value[name], revision))
    for name in ("formatterBefore", "formatterAfter"):
        require(name not in value or metadata(value[name]))
    require(type(value["processes"]) is list and len(value["processes"]) <= len(PURPOSES))
    for row, purpose in zip(value["processes"], PURPOSES):
        process(row, purpose)
    require(type(value["formatterPackets"]) is list and len(value["formatterPackets"]) <= 2)
    for index, row in enumerate(value["formatterPackets"]):
        packet(row, index, records, formatter)
    if "beforeState" in value:
        require(value["beforeState"] == manifest["state"])
    if value["classification"] == "canonical-source-proposals":
        require(len(value["processes"]) == 13 and len(value["formatterPackets"]) == 2 and value.get("count") == 2)
        require(value.get("historyKind") == "history-admitted" and "failureClass" not in value)
        require(value.get("afterState") == value.get("beforeState") == manifest["state"] and manifest["state"] is not None)
        require(value.get("formatterBefore") == value.get("formatterAfter") and value.get("formatterBefore") is not None)
        tool = next((row for row in manifest["tools"] if row["name"] == "golangci-lint"), None)
        require(tool is not None and {k: v for k, v in tool.items() if k != "name"} == value["formatterBefore"])
        require(all(formatter.settled(row) and row.get("processLaunched") is True for row in value["processes"]))
        require(formatter.admit_format(records, [value["processes"][7], value["processes"][8]], value["identity"])["classification"] == "canonical-source-proposals")

def admit_bundle(raw, revision, execution, formatter):
    try:
        require(type(raw) is dict and set(raw) == set(NAMES) and sha(revision, 40))
        require(all(type(raw[name]) is bytes and 0 < len(raw[name]) <= CAPS[name] for name in NAMES))
        values = {name: safe_json(raw[name]) for name in NAMES}
        binding = values["artifact-manifest.json"]
        require(type(binding) is dict and set(binding) == {"schemaVersion", "artifacts"} and type(binding["schemaVersion"]) is int and binding["schemaVersion"] == 1)
        require(type(binding["artifacts"]) is list and len(binding["artifacts"]) == 3)
        seen = set()
        for row in binding["artifacts"]:
            require(type(row) is dict and set(row) == {"name", "bytes", "sha256"} and row["name"] in NAMES[:3] and row["name"] not in seen)
            seen.add(row["name"])
            require(type(row["bytes"]) is int and row["bytes"] == len(raw[row["name"]]) and row["sha256"] == hashlib.sha256(raw[row["name"]]).hexdigest())
        results = values["results.json"]
        require(type(results) is dict and set(results) == {"schemaVersion", "records", "autoAdoption"} and type(results["schemaVersion"]) is int and results["schemaVersion"] == 1 and results["autoAdoption"] is False)
        records = results["records"]
        require(type(records) is list and len(records) <= 2)
        paths = set()
        for row in records:
            proposal(row, formatter)
            require(row["path"] not in paths)
            paths.add(row["path"])
        source_manifest(values["source-manifest.json"], revision)
        receipt(values["receipt.json"], revision, values["source-manifest.json"], records, formatter)
        process(execution)
        require(execution.get("processLaunched") is True and execution["stopReason"] is None and all(execution[k] is True for k in ("ownedProcessExited", "ownedGroupSettled", "captureSettled")))
        classification = values["receipt.json"]["classification"]
        require(type(execution["exitCode"]) is int and execution["exitCode"] == (0 if classification == "canonical-source-proposals" else 2))
        return classification
    except (KeyError, TypeError, ValueError, UnicodeError, RecursionError):
        raise ValueError("artifact-admission") from None

def admit_and_publish(raw, revision, execution, formatter, root, deadline, publisher=publish):
    classification = admit_bundle(raw, revision, execution, formatter)
    publisher(raw, root, deadline)
    return classification
