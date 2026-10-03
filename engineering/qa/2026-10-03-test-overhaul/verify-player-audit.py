#!/usr/bin/env python3
"""Reproduce Player ledger inventory and removals without building the product.

Failure analysis before this verifier was written:
- A row may omit or duplicate an original declaration, or carry the wrong body hash.
- A removed declaration may still exist, or a retained declaration may be lost/changed.
- Reconciled upstream additions/body changes may be missing from the appendix.
- A keeper citation may name a removed or nonexistent declaration/subtest.
- A partial inventory may be misreported as a completed semantic review.
This artifact checker validates those facts only; it does not infer test quality,
execute Go/browser tests, or turn fixtures into genuine E2E evidence.
"""
import argparse
import collections
import hashlib
import json
import platform
import re
import subprocess
import sys
from pathlib import Path


def sha(data):
    return hashlib.sha256(data).hexdigest()


def git(checkout, *args):
    return subprocess.check_output(["git", "-C", str(checkout), *args])


def function_end(source, start):
    index = source.index("{", start)
    depth, quote, comment = 0, None, None
    while index < len(source):
        character, pair = source[index], source[index:index + 2]
        if comment == "line":
            if character == "\n":
                comment = None
        elif comment == "block":
            if pair == "*/":
                comment = None
                index += 1
        elif quote:
            if character == "\\" and quote != "`":
                index += 1
            elif character == quote:
                quote = None
        elif pair == "//":
            comment = "line"
            index += 1
        elif pair == "/*":
            comment = "block"
            index += 1
        elif character in "`\"'":
            quote = character
        elif character == "{":
            depth += 1
        elif character == "}":
            depth -= 1
            if depth == 0:
                return index + 1
        index += 1
    raise ValueError("Unterminated Go declaration")


def declarations(source):
    result = {}
    for match in re.finditer(r"^func ((?:Test|Fuzz|Benchmark)\w+)\(", source, re.M):
        end = function_end(source, match.start())
        name = match.group(1)
        if name in result:
            raise ValueError("Duplicate declaration in one file: " + name)
        result[name] = sha(source[match.start():end].encode())
    return result


def inventory(checkout, revision):
    paths = git(checkout, "ls-tree", "-r", "--name-only", revision, "--",
                "apps/player/internal", "apps/player/cmd", "apps/player/pkg").decode().splitlines()
    files, result = {}, {}
    for path in paths:
        if not path.endswith("_test.go"):
            continue
        data = git(checkout, "show", revision + ":" + path)
        files[path] = sha(data)
        for name, body_hash in declarations(data.decode()).items():
            result[(path, name)] = body_hash
    return files, result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--checkout", type=Path, default=Path.cwd())
    parser.add_argument("--revision", default="HEAD")
    parser.add_argument("--ledger", type=Path, default=Path(__file__).with_name("player-backend.json"))
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    checkout = args.checkout.resolve()
    ledger_data = args.ledger.read_bytes()
    ledger = json.loads(ledger_data)
    revision = git(checkout, "rev-parse", args.revision + "^{commit}").decode().strip()
    base_files, base = inventory(checkout, ledger["base_sha"])
    current_files, current = inventory(checkout, revision)
    rows = ledger["tests"]
    expected = {(row["path"], row["name"]): row for row in rows}
    errors = []
    if len(expected) != len(rows) or set(expected) != set(base):
        errors.append({"kind": "original_inventory_mismatch", "duplicates": len(rows) - len(expected),
                       "missing": sorted(set(base) - set(expected)), "extra": sorted(set(expected) - set(base))})
    for row in ledger["files"]:
        if base_files.get(row["path"]) != row["sha256"]:
            errors.append({"kind": "original_file_hash_mismatch", "path": row["path"]})
    appendix = ledger.get("upstream_reconciliation", {}).get("rows", [])
    updates = {(row["path"], row["name"]): row for row in appendix}
    for key, row in expected.items():
        if base.get(key) != row["sha256"]:
            errors.append({"kind": "original_body_hash_mismatch", "key": key})
        if row["review_status"] not in {"semantic_review_complete", "candidate_evidence_complete"}:
            errors.append({"kind": "semantic_review_pending", "key": key, "status": row["review_status"]})
        if row["decision"] == "D":
            if key in current:
                errors.append({"kind": "removed_declaration_still_present", "key": key})
        else:
            wanted = updates.get(key, row)["sha256"]
            if current.get(key) != wanted:
                errors.append({"kind": "retained_declaration_missing_or_changed", "key": key,
                               "wanted_sha256": wanted, "actual_sha256": current.get(key)})
    for key, row in updates.items():
        if current.get(key) != row["sha256"]:
            errors.append({"kind": "upstream_declaration_missing_or_changed", "key": key})
    for update in ledger.get("upstream_reconciliation", {}).get("file_updates", []):
        if current_files.get(update["path"]) != update["sha256"]:
            errors.append({"kind": "upstream_fixture_file_missing_or_changed", "path": update["path"],
                           "wanted_sha256": update["sha256"], "actual_sha256": current_files.get(update["path"])})
    undocumented = set(current) - set(expected) - set(updates)
    if undocumented:
        errors.append({"kind": "upstream_declarations_missing_from_appendix", "keys": sorted(undocumented)})
    # Keeper names include actual declarations and named CLI/helper subtests.
    keeper_names = set()
    paths = git(checkout, "ls-tree", "-r", "--name-only", revision, "--",
                "apps/player", "apps/subtitles", "packages").decode().splitlines()
    for path in paths:
        if not path.endswith(".go"):
            continue
        source = git(checkout, "show", revision + ":" + path).decode()
        keeper_names.update(re.findall(r"^func (Test\w+)\(", source, re.M))
        keeper_names.update(re.findall(r't\.Run\("(Test\w+)"', source))
    for row in rows + appendix:
        for field in ("remaining_proof", "remaining_coverage"):
            for name in set(re.findall(r"\bTest[A-Z]\w+", str(row.get(field, "")))):
                if name not in keeper_names:
                    errors.append({"kind": "keeper_name_absent", "row": row["name"], "field": field, "name": name})
    report = {
        "schema": "kinosail-player-audit-verification-v1", "result": "pass" if not errors else "fail",
        "revision": revision, "base_sha": ledger["base_sha"], "checkout": str(checkout),
        "ledger_sha256": sha(ledger_data), "verifier_sha256": sha(Path(__file__).read_bytes()),
        "command": [sys.executable, *sys.argv],
        "environment": {"python": platform.python_version(), "platform": platform.platform()},
        "counts": {"original_files": len(base_files), "original_declarations": len(base),
                   "current_files": len(current_files), "current_declarations": len(current),
                   "decisions": dict(collections.Counter(row["decision"] for row in rows)),
                   "upstream_rows": len(appendix),
                   "upstream_fixture_files": len(ledger.get("upstream_reconciliation", {}).get("file_updates", [])),
                   "preserved_C_names": [row["name"] for row in rows if row["decision"] == "C"]},
        "scope": "Inventory/body integrity, actual removals/preservation and exact keeper-name existence. Semantic quality and product execution remain evidenced by ledger reviewers and root run artifacts.",
        "errors": errors,
    }
    rendered = json.dumps(report, indent=2) + "\n"
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered)
    print(rendered, end="")
    return 0 if not errors else 1


if __name__ == "__main__":
    sys.exit(main())
