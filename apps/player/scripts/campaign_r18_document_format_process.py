"""Fixed stdin bridge within the parent's owned process group; no file edits."""
import base64
import hashlib
import json
import os
import selectors
import shutil
import subprocess
import time

from campaign_r18_document_inputs import read_small
from campaign_r18_document_sources import GO_FILES, ROOT, fingerprint

SOURCE_SHA = ("18ab485d6f82196554b598152a15984f44db06cc2620e5cd04f17b2b9bccb0c5",
              "8de16a1fbb631a76bf22378c3fca25e6cff7dfda55239437e58e8ed377316cb8")
CONFIG = "apps/player/.golangci.yml"
CONFIG_SHA = "389f5d87f20f6ea354dc36be94150e4057a09e1fd31a5cfb6aa14de96953eafd"
CAP = 32768
EMPTY = hashlib.sha256(b"").hexdigest()
FIELDS = {"schemaVersion", "index", "formatterExitCode", "timedOut", "formatterExited",
          "captureSettled", "sourceUnchanged", "toolUnchanged", "stdoutBytes",
          "stdoutSHA256", "stderrBytes", "stderrSHA256", "outputBase64", "failureKind"}
FAILURES = {"formatter-launch", "formatter-timeout", "formatter-output-bound",
            "formatter-capture", "formatter-exit", "formatter-stderr",
            "formatter-unsettled", "formatter-input-drift", "formatter-input-prerequisite"}


def formatter_command(tool):
    return [str(tool), "fmt", "--stdin", "--config", CONFIG]


def settle_child(process):
    """Only the owned formatter leader; parent execute settles its entire group."""
    try:
        if process.poll() is None:
            process.kill()
        return type(process.wait(timeout=3)) is int
    except (OSError, subprocess.SubprocessError):
        return False


def pairs(items):
    result = {}
    for key, value in items:
        if key in result:
            raise ValueError("duplicate-packet-key")
        result[key] = value
    return result


def reject_constant(_value):
    raise ValueError("packet-nonfinite")


def valid_packet(value, index):
    if type(value) is not dict or set(value) != FIELDS:
        return False
    if type(value["schemaVersion"]) is not int or value["schemaVersion"] != 1:
        return False
    if type(value["index"]) is not int or value["index"] != index:
        return False
    for name in ("timedOut", "formatterExited", "captureSettled", "sourceUnchanged", "toolUnchanged"):
        if type(value[name]) is not bool:
            return False
    code = value["formatterExitCode"]
    if code is not None and (type(code) is not int or not -255 <= code <= 255):
        return False
    if value["formatterExited"] and code is None:
        return False
    for stream in ("stdout", "stderr"):
        count, sha = value[stream + "Bytes"], value[stream + "SHA256"]
        if type(count) is not int or not 0 <= count <= CAP + 4096:
            return False
        if type(sha) is not str or len(sha) != 64 or any(c not in "0123456789abcdef" for c in sha):
            return False
        if count == 0 and sha != EMPTY:
            return False
    failure, encoded = value["failureKind"], value["outputBase64"]
    if failure is not None:
        return failure in FAILURES and encoded is None
    if code != 0 or value["timedOut"] or value["stderrBytes"] != 0:
        return False
    if not all(value[k] for k in ("formatterExited", "captureSettled", "sourceUnchanged", "toolUnchanged")):
        return False
    if type(encoded) is not str or len(encoded) > 44000:
        return False
    try:
        raw = base64.b64decode(encoded, validate=True)
    except (ValueError, TypeError):
        return False
    return len(raw) == value["stdoutBytes"] and 0 < len(raw) <= CAP and hashlib.sha256(raw).hexdigest() == value["stdoutSHA256"]


class PacketProjection:
    def __init__(self, index):
        self.index, self.value, self.prerequisite = index, None, False

    def consume(self, raw):
        if self.prerequisite:
            return
        try:
            if self.value is not None or type(raw) is not bytes or len(raw) > 65536:
                raise ValueError("packet-bound")
            value = json.loads(raw.decode("utf-8"), object_pairs_hook=pairs, parse_constant=reject_constant)
            if not valid_packet(value, self.index):
                raise ValueError("packet-shape")
            self.value = value
        except (ValueError, TypeError, UnicodeError):
            self.value, self.prerequisite = None, True

    def result(self):
        return None if self.prerequisite else self.value


def run_formatter(tool, original):
    deadline = time.monotonic() + 10
    output, totals = bytearray(), {"stdout": 0, "stderr": 0}
    hashes = {name: hashlib.sha256() for name in totals}
    process, failure, settled, eof = None, None, False, False
    try:
        process = subprocess.Popen(formatter_command(tool), cwd=ROOT, stdin=subprocess.PIPE,
                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        with selectors.DefaultSelector() as selector:
            for name in ("stdin", "stdout", "stderr"):
                stream = getattr(process, name)
                os.set_blocking(stream.fileno(), False)
                selector.register(stream, selectors.EVENT_WRITE if name == "stdin" else selectors.EVENT_READ, name)
            sent = 0
            while selector.get_map():
                if time.monotonic() >= deadline:
                    failure = "formatter-timeout"
                    break
                for key, _events in selector.select(min(0.1, deadline - time.monotonic())):
                    name = key.data
                    if name == "stdin":
                        try:
                            sent += os.write(key.fd, original[sent:sent + 4096])
                        except BlockingIOError:
                            continue
                        if sent == len(original):
                            selector.unregister(key.fileobj)
                            key.fileobj.close()
                        continue
                    try:
                        chunk = os.read(key.fd, 4096)
                    except BlockingIOError:
                        continue
                    if not chunk:
                        selector.unregister(key.fileobj)
                        key.fileobj.close()
                        continue
                    totals[name] += len(chunk)
                    hashes[name].update(chunk)
                    if totals[name] > CAP:
                        failure = "formatter-output-bound"
                        break
                    if name == "stdout":
                        output.extend(chunk)
                if failure is not None:
                    break
            eof = not selector.get_map() and sent == len(original)
            if eof and failure is None:
                process.wait(timeout=max(0.001, min(1, deadline - time.monotonic())))
    except (OSError, ValueError, subprocess.SubprocessError):
        failure = "formatter-launch" if process is None else "formatter-capture"
    finally:
        if process is not None:
            settled = settle_child(process)
            for name in ("stdin", "stdout", "stderr"):
                try:
                    getattr(process, name).close()
                except OSError:
                    settled = False
    code = None if process is None else process.returncode
    if failure is None:
        if not settled or not eof:
            failure = "formatter-unsettled"
        elif code != 0:
            failure = "formatter-exit"
        elif totals["stderr"]:
            failure = "formatter-stderr"
    return {"formatterExitCode": code, "timedOut": failure == "formatter-timeout",
            "formatterExited": settled, "captureSettled": eof,
            "stdoutBytes": totals["stdout"], "stdoutSHA256": hashes["stdout"].hexdigest(),
            "stderrBytes": totals["stderr"], "stderrSHA256": hashes["stderr"].hexdigest(),
            "outputBase64": base64.b64encode(output).decode() if failure is None else None,
            "failureKind": failure}


def formatter_binary(deadline, budget=None):
    found = shutil.which("golangci-lint")
    if not found:
        raise ValueError("formatter-unavailable")
    from pathlib import Path
    tool = Path(found).resolve(strict=True)
    state = fingerprint(tool, deadline=deadline, budget=budget)
    if not state["mode"] & 0o111 or not state["bytes"]:
        raise ValueError("formatter-tool-boundary")
    return tool, state


def guarded_inputs(index, expected_tool, deadline):
    if type(index) is not int or index not in (0, 1):
        raise ValueError("formatter-source-index")
    original = read_small(ROOT / GO_FILES[index], 16384, deadline)
    if hashlib.sha256(original).hexdigest() != SOURCE_SHA[index]:
        raise ValueError("formatter-source-pin")
    config = read_small(ROOT / CONFIG, 16384, deadline)
    if hashlib.sha256(config).hexdigest() != CONFIG_SHA:
        raise ValueError("formatter-config-pin")
    tool, state = formatter_binary(deadline)
    if state["sha256"] != expected_tool:
        raise ValueError("formatter-tool-pin")
    return original, config, tool, state


def bridge(index, expected_tool):
    deadline = time.monotonic() + 16
    value = {"schemaVersion": 1, "index": index, "formatterExitCode": None,
             "timedOut": False, "formatterExited": False, "captureSettled": False,
             "sourceUnchanged": False, "toolUnchanged": False, "stdoutBytes": 0,
             "stdoutSHA256": EMPTY, "stderrBytes": 0, "stderrSHA256": EMPTY,
             "outputBase64": None, "failureKind": "formatter-input-prerequisite"}
    try:
        original, config, tool, before = guarded_inputs(index, expected_tool, deadline)
        value.update(run_formatter(tool, original))
        after, config_after, tool_after, state_after = guarded_inputs(index, expected_tool, deadline)
        value["sourceUnchanged"] = original == after and config == config_after
        value["toolUnchanged"] = tool == tool_after and before == state_after
        if not value["sourceUnchanged"] or not value["toolUnchanged"]:
            value.update(outputBase64=None, failureKind="formatter-input-drift")
    except (OSError, ValueError, KeyError, ImportError, KeyboardInterrupt):
        value.update(outputBase64=None, failureKind="formatter-input-prerequisite")
    return value
