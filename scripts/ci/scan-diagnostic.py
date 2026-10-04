#!/usr/bin/env python3
"""Keep the full-history failure and publish only validated finding locations."""
import argparse
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import tempfile


MAX_REPORT_BYTES = 4 * 1024 * 1024
MAX_FINDINGS = 10_000
SCAN_SECONDS = 180


def project(findings):
    """Whitelist metadata; never include a finding's secret, match or author."""
    if not isinstance(findings, list) or len(findings) > MAX_FINDINGS:
        raise ValueError("Invalid finding metadata")
    result = []
    for finding in findings:
        if not isinstance(finding, dict):
            raise ValueError("Invalid finding metadata")
        rule = finding.get("RuleID")
        file = finding.get("File")
        commit = finding.get("Commit")
        line = finding.get("StartLine")
        if (not isinstance(rule, str) or not re.fullmatch(r"[A-Za-z0-9_-]{1,128}", rule)
                or not isinstance(file, str) or not 1 <= len(file) <= 1024
                or any(ord(char) < 32 or ord(char) == 127 for char in file)
                or "\\" in file or PurePosixPath(file).is_absolute()
                or any(part in ("", ".", "..") for part in file.split("/"))
                or not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{40}", commit)
                or type(line) is not int or not 1 <= line <= 10_000_000):
            raise ValueError("Invalid finding metadata")
        result.append({"rule": rule, "file": file, "commit": commit, "line": line})
    return result


def publish(output, document):
    """Expose one complete safe document, without overwriting an existing file."""
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=output.parent,
                                         prefix=".scan-safe-", delete=False) as handle:
            temporary = Path(handle.name)
            json.dump(document, handle, indent=2)
            handle.write("\n")
        os.link(temporary, output)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def scan(scanner, output):
    output = Path(output)
    if output.exists() or output.is_symlink():
        return 2
    try:
        with tempfile.TemporaryDirectory(prefix="kinosail-private-scan-") as directory:
            report = Path(directory) / "report.json"
            command = [scanner, "git", "--redact", "--no-banner", "--log-opts=--all",
                       "--report-format=json", "--report-path=" + str(report)]
            # Scanner output can contain remote/private context. Never forward it.
            result = subprocess.run(command, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                                    timeout=SCAN_SECONDS, check=False)
            if result.returncode not in (0, 1) or not report.is_file() or report.is_symlink():
                return 2
            if report.stat().st_size > MAX_REPORT_BYTES:
                return 2
            findings = project(json.loads(report.read_text(encoding="utf-8")))
            if bool(findings) != (result.returncode == 1):
                return 2
            publish(output, {"scanner_exit_code": result.returncode, "coverage": "--all",
                             "findings": findings})
            return result.returncode
    except (OSError, ValueError, subprocess.TimeoutExpired):
        # Do not interpolate exception text: it can contain private report content.
        return 2


def main():
    parser = argparse.ArgumentParser(allow_abbrev=False)
    parser.add_argument("--scanner", default="gitleaks")
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    previous = os.umask(0o077)
    try:
        code = scan(args.scanner, args.output)
    finally:
        os.umask(previous)
    if code == 2:
        print("Full-history diagnostic unavailable; no new projection published.")
    return code


if __name__ == "__main__":
    raise SystemExit(main())
