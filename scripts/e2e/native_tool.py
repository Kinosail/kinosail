"""Bounded version evidence for the two fixed native fixture tools."""
import os
import re
import selectors
import signal
import subprocess
import time

DEADLINE_SECONDS = 30


def version(platform, raw):
    text = raw.decode("utf-8", errors="strict").strip()
    if platform == "ios":
        match = re.fullmatch(r"Xcode (27(?:\.\d{1,2}){0,2})\nBuild version (\d{2}[A-Z]\d{1,7}[a-z]?)", text)
        return {"name": "Xcode", "version": match[1], "build": match[2]} if match else None
    release = re.search(r'^(?:openjdk|java) version "(17(?:\.\d{1,3}){1,3}(?:-ea)?)"(?: |$)', text, re.MULTILINE)
    build = re.search(r"^.*Runtime Environment.*\(build (17(?:\.\d{1,3}){1,3}(?:-ea)?\+\d{1,5}(?:-LTS)?)\)$", text, re.MULTILINE)
    if release and build and build[1].split("+")[0] == release[1]:
        return {"name": "Java", "version": release[1], "build": build[1]}
    return None


def probe_native_tool(platform, args, project, env, witness):
    if platform not in ("ios", "android") or args != (["xcodebuild", "-version"] if platform == "ios" else ["java", "-version"]):
        raise RuntimeError("Invalid native fixture tool")
    stage = {"tool": "Xcode" if platform == "ios" else "Java", "outcome": "started", "exitCode": None,
             "stdoutBytes": 0, "stderrBytes": 0, "stdoutTokensRecognized": False,
             "combinedTokensRecognized": False, "outputTruncated": False}
    witness["nativeToolProbe"] = stage
    streams = [bytearray(), bytearray()]
    process = None
    selector = selectors.DefaultSelector()
    deadline = time.monotonic() + DEADLINE_SECONDS
    try:
        try:
            process = subprocess.Popen(args, cwd=project, env=env, stdin=subprocess.DEVNULL,
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
        except OSError:
            stage["outcome"] = "unavailable"
            raise RuntimeError("Native tool unavailable") from None
        selector.register(process.stdout, selectors.EVENT_READ, 0)
        selector.register(process.stderr, selectors.EVENT_READ, 1)
        while selector.get_map():
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                stage["outcome"] = "timeout"
                raise RuntimeError("Native tool timed out")
            for key, _ in selector.select(remaining):
                raw = os.read(key.fileobj.fileno(), 4096)
                if not raw:
                    selector.unregister(key.fileobj)
                    continue
                streams[key.data].extend(raw)
                if sum(map(len, streams)) > 8192:
                    stage.update(outcome="overflow", outputTruncated=True)
                    raise RuntimeError("Oversized native tool witness")
        try:
            stage["exitCode"] = process.wait(timeout=max(.001, deadline - time.monotonic()))
        except subprocess.TimeoutExpired:
            stage["outcome"] = "timeout"
            raise RuntimeError("Native tool timed out") from None
        try:
            stage["stdoutTokensRecognized"] = version(platform, bytes(streams[0])) is not None
            accepted = version(platform, bytes(streams[0] + streams[1]))
            stage["combinedTokensRecognized"] = accepted is not None
        except UnicodeError:
            accepted = None
        if stage["exitCode"]:
            stage["outcome"] = "nonzero"
        elif accepted is None:
            stage["outcome"] = "rejected"
        else:
            stage["outcome"] = "valid"
            return accepted
        raise RuntimeError("Native tool version rejected")
    finally:
        if process is not None:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGKILL)
            process.wait(timeout=5)
            stage["exitCode"] = process.returncode
            process.stdout.close()
            process.stderr.close()
        selector.close()
        stage["stdoutBytes"], stage["stderrBytes"] = (min(len(raw), 8193) for raw in streams)
