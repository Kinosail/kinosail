#!/usr/bin/env python3
"""Run govulncheck for one Go module and fail on every called finding without a current, version-bound exception."""

import argparse
import datetime
import json
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[2]
EXCEPTIONS = Path(__file__).with_name("vuln-exceptions.json")
FIELDS = {"id", "module", "fixed", "expires", "reason"}
TOOL = re.compile(r"golang\.org/x/vuln/cmd/govulncheck@v\d+\.\d+\.\d+")
RELEASE = re.compile(r"v(\d+)\.(\d+)\.(\d+)")


def release(version):
    match = RELEASE.fullmatch(version or "")
    return tuple(int(part) for part in match.groups()) if match else None


def load_exceptions(path=EXCEPTIONS):
    try:
        entries = json.loads(Path(path).read_text())
    except (OSError, ValueError) as error:
        raise ValueError(f"cannot read vulnerability exceptions: {error}") from error
    if not isinstance(entries, list):
        raise ValueError("vulnerability exceptions must be a list")
    for entry in entries:
        if not isinstance(entry, dict) or set(entry) != FIELDS or not all(isinstance(value, str) for value in entry.values()):
            raise ValueError("each exception needs id, module, fixed, expires and reason")
        if not re.fullmatch(r"GO-\d{4}-\d+", entry["id"]) or not entry["module"] or release(entry["fixed"]) is None:
            raise ValueError(f"invalid exception {entry['id']!r}")
        if len(entry["reason"]) < 40:
            raise ValueError(f"exception {entry['id']} needs a reason")
        try:
            datetime.date.fromisoformat(entry["expires"])
        except ValueError as error:
            raise ValueError(f"exception {entry['id']} needs an ISO expiry date") from error
    if len({entry["id"] for entry in entries}) != len(entries):
        raise ValueError("duplicate vulnerability exception")
    return entries


def messages(output):
    decoder, position, found = json.JSONDecoder(), 0, []
    while position < len(output):
        if output[position].isspace():
            position += 1
            continue
        message, position = decoder.raw_decode(output, position)
        if not isinstance(message, dict):
            raise ValueError("unexpected govulncheck output")
        found.append(message)
    return found


def excepted(osv, frame, exceptions, today):
    found = release(frame.get("version", ""))
    for entry in exceptions:
        if entry["id"] == osv and entry["module"] == frame.get("module"):
            return found is not None and found >= release(entry["fixed"]) and today <= datetime.date.fromisoformat(entry["expires"])
    return False


def blocking(output, exceptions, today):
    found = messages(output)
    if not any("config" in message for message in found):
        raise ValueError("govulncheck reported no scan")
    blocked = []
    for message in found:
        trace = message.get("finding", {}).get("trace") or [{}]
        osv = message.get("finding", {}).get("osv", "")
        if "function" in trace[0] and not excepted(osv, trace[0], exceptions, today) and osv not in blocked:
            blocked.append(osv)
    return sorted(blocked)


def command(directory, tool):
    target = (ROOT / directory).resolve()
    if ROOT not in target.parents or not (target / "go.mod").is_file() or not TOOL.fullmatch(tool):
        raise ValueError("expected a Go module in this repository and a pinned govulncheck release")
    return ["go", "-C", str(target), "run", tool, "-format", "json", "./..."]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory")
    parser.add_argument("tool")
    args = parser.parse_args()
    exceptions = load_exceptions()
    result = subprocess.run(command(args.directory, args.tool), stdout=subprocess.PIPE, text=True, check=True, timeout=1200)
    blocked = blocking(result.stdout, exceptions, datetime.datetime.now(datetime.timezone.utc).date())
    for entry in exceptions:
        print(f"{args.directory}: exception {entry['id']} for {entry['module']} at or above {entry['fixed']} until {entry['expires']}")
    if blocked:
        raise SystemExit(f"{args.directory}: called vulnerabilities without a current exception: {', '.join(blocked)}")
    print(f"{args.directory}: no blocking vulnerabilities")


if __name__ == "__main__":
    main()
