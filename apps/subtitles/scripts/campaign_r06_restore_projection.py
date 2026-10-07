"""Strict Restore source projection plus explicitly unadmitted diagnostics.

The diagnostic never satisfies canonical acceptance and publishes no runtime data.
"""
import base64
import hashlib

from campaign_r06_restore_tokens import equivalent

PREFIX = "apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/"
GO_FILES = (
    PREFIX + "restore_assertions_test.go",
    PREFIX + "restore_controls_test.go",
    PREFIX + "restore_exchange_test.go",
    PREFIX + "restore_filesystem_test.go",
    PREFIX + "restore_http_test.go",
    PREFIX + "restore_main_test.go",
    PREFIX + "restore_owner_enrollment_test.go",
    PREFIX + "restore_owner_test.go",
    PREFIX + "restore_requests_test.go",
    PREFIX + "restore_routes_test.go",
    PREFIX + "restore_routing_test.go",
    PREFIX + "restore_target_test.go",
    PREFIX + "restore_transport_test.go",
    PREFIX + "restore_witness_test.go",
)
INPUT_CAP, OUTPUT_CAP = 256 * 1024, 512 * 1024


def pin(data):
    return {"bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}


def blob(data):
    return hashlib.sha1(("blob " + str(len(data)) + "\0").encode() + data).hexdigest()


def matches_pin(data, expected):
    return type(data) is bytes and type(expected) is dict and pin(data) == expected


def format_record(*, path, original, output, exit_code, stderr):
    if type(path) is not str or path not in GO_FILES:
        raise ValueError("format-path")
    for data, cap in ((original, INPUT_CAP), (output, OUTPUT_CAP)):
        if type(data) is not bytes or not 0 < len(data) <= cap or b"\0" in data:
            raise ValueError("format-bytes")
        data.decode("utf-8")
    if type(exit_code) is not int or exit_code != 0 or type(stderr) is not bytes or stderr:
        raise ValueError("format-result")
    if len(output.splitlines()) > 300:
        raise ValueError("format-file-lines")
    if not equivalent(original, output):
        raise ValueError("format-token-change")
    return {"path": path, "original": pin(original), "formatted": pin(output),
            "formattedLines": len(output.splitlines()), "changed": original != output,
            "tokensPreserved": True, "formattedSourceBase64": base64.b64encode(output).decode("ascii")}


def complete_files(records):
    return (type(records) is list and len(records) == len(GO_FILES)
            and all(type(row) is dict and row.get("tokensPreserved") is True for row in records)
            and [row.get("path") for row in records] == list(GO_FILES))



def unadmitted_record(*, path, original, output, expected, phase, stderr,
                      source_unchanged, failure_code):
    if source_unchanged is not True or type(failure_code) is not str:
        return None
    if failure_code != "format-token-change" or type(phase) is not dict:
        return None
    if (type(phase.get("exitCode")) is not int or phase["exitCode"] != 0
            or type(phase.get("timedOut")) is not bool or phase["timedOut"]
            or phase.get("ownedGroupStopped") is not True
            or phase.get("captureSettled") is not True
            or phase.get("captureFailed") is not False):
        return None
    try:
        format_record(path=path, original=original, output=output,
                      exit_code=phase["exitCode"], stderr=stderr)
    except (ValueError, TypeError, UnicodeDecodeError) as error:
        if type(error) is not ValueError or str(error) != "format-token-change":
            return None
    else:
        return None
    if (type(expected) is not tuple or len(expected) != 3
            or type(expected[0]) is not str or type(expected[1]) is not str
            or type(expected[2]) is not int
            or expected != (blob(original), pin(original)["sha256"], len(original))):
        return None
    return {"path": path, "admitted": False, "sourceOnly": True,
            "failureCode": "format-token-change", "tokensPreserved": False,
            "original": {"gitBlob": blob(original), **pin(original)},
            "formatted": pin(output), "formattedLines": len(output.splitlines()),
            "formattedSourceBase64": base64.b64encode(output).decode("ascii")}
