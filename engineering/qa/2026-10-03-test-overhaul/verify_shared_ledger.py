#!/usr/bin/env python3
"""Read-only reproduction of the shared-package declaration/removal ledger.

Run from any checkout containing the original baseline Git objects:
  python3 engineering/qa/2026-10-03-test-overhaul/verify_shared_ledger.py
Optional original Go JSON receipt verification:
  ... --baseline-events .verification/test-overhaul/baseline/packages-go.jsonl
After reconciling newer main, require its separately recorded upstream additions:
  ... --require-upstream-additions

This verifies audit/source integrity; it does not execute or replace product tests.
"""

import argparse
import collections
import hashlib
import json
import pathlib
import re
import subprocess

DECLARATION = re.compile(r"^func ((Test|Benchmark|Fuzz)\w*)\(", re.MULTILINE)


def declaration_end(source, start):
    position = source.index("{", start)
    depth, quote = 0, None
    while position < len(source):
        char = source[position]
        if quote:
            if char == "\\" and quote != "`":
                position += 2
                continue
            if char == quote:
                quote = None
        elif source.startswith("//", position):
            newline = source.find("\n", position)
            position = len(source) if newline == -1 else newline
            continue
        elif source.startswith("/*", position):
            position = source.index("*/", position + 2) + 2
            continue
        elif char in ('"', "'", "`"):
            quote = char
        elif char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                return position + 1
        position += 1
    raise ValueError("Unclosed Go declaration")


def declarations(path, source):
    result = {}
    for match in DECLARATION.finditer(source):
        body = source[match.start():declaration_end(source, match.start())]
        key = (path, match.group(1))
        assert key not in result, key
        result[key] = {
            "body_sha256": hashlib.sha256(body.encode()).hexdigest(),
            "line": source.count("\n", 0, match.start()) + 1,
            "kind": match.group(2),
        }
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--baseline-events", type=pathlib.Path)
    parser.add_argument("--allow-pending", action="store_true")
    parser.add_argument("--require-review-receipts", action="store_true")
    parser.add_argument("--require-upstream-additions", action="store_true")
    args = parser.parse_args()
    root = pathlib.Path(__file__).resolve().parents[3]
    ledger = json.loads((pathlib.Path(__file__).parent / "shared-identity.json").read_text())

    def git(*arguments):
        return subprocess.check_output(["git", "-C", str(root), *arguments], text=True)

    base = ledger["baseline_sha"]
    files = [path for path in git("ls-tree", "-r", "--name-only", base, "packages").splitlines()
             if path.endswith("_test.go")]
    original, current, source_lines, final_lines = {}, {}, 0, 0
    file_receipts = {entry["path"]: entry for entry in ledger["files"]}
    for path in files:
        source = git("show", base + ":" + path)
        assert hashlib.sha256(source.encode()).hexdigest() == file_receipts[path]["baseline_sha256"], path
        source_lines += len(source.splitlines())
        original.update(declarations(path, source))
        candidate = root / path
        if candidate.exists():
            content = candidate.read_text()
            final_lines += len(content.splitlines())
            current.update(declarations(path, content))
    additions = ledger.get("preserved_upstream_additions", [])
    addition_paths = {entry["file"] for entry in additions}
    assert not addition_paths.intersection(files), "Upstream addition duplicates baseline file"
    preserved_upstream, absent_upstream = {}, []
    for entry in additions:
        candidate = root / entry["file"]
        if not candidate.exists():
            absent_upstream.append({"file": entry["file"], "name": entry["name"]})
            continue
        content = candidate.read_text()
        assert hashlib.sha256(content.encode()).hexdigest() == entry["file_sha256"], entry["file"]
        parsed = declarations(entry["file"], content)
        key = (entry["file"], entry["name"])
        assert parsed[key]["body_sha256"] == entry["body_sha256"], key
        assert parsed[key]["kind"] == entry["kind"], key
        assert parsed[key]["line"] == entry["line"], key
        preserved_upstream.update(parsed)
    assert set(preserved_upstream) == {(entry["file"], entry["name"]) for entry in additions
                                      if (root / entry["file"]).exists()}, "Unlisted upstream declaration"
    if args.require_upstream_additions:
        assert not absent_upstream, ("Upstream additions missing after reconciliation", absent_upstream)
    tracked = git("ls-files", "packages").splitlines()
    new_files = [path for path in tracked if path.endswith("_test.go")
                 and path not in files and path not in addition_paths]
    assert not new_files, ("New test files outside baseline ledger", new_files)
    rows = {(row["file"], row["name"]): row for row in ledger["declarations"]}
    assert len(rows) == len(ledger["declarations"]), "Duplicate ledger rows"
    assert set(rows) == set(original), "Ledger does not cover exact baseline inventory"
    missing_review_receipts = []
    repaired_bodies = []
    for key, row in rows.items():
        assert original[key]["body_sha256"] == row["body_sha256"], (key, "baseline body hash")
        assert original[key]["line"] == row["line"], (key, "baseline line")
        assert original[key]["kind"] == row["kind"], (key, "declaration kind")
        if row["decision"] == "D":
            assert key not in current, (key, "removed declaration survives")
            if not row.get("independent_removal_review"):
                missing_review_receipts.append({"file": key[0], "name": key[1]})
        else:
            assert key in current, (key, "retained declaration missing")
            expected = row["body_sha256"]
            if row.get("candidate_body_sha256") != expected and row.get("repair"):
                assert row["owner_group"] == "watchrooms", (key, "unexpected assertion repair")
                assert ledger["failure_analysis_first"]["watchrooms_socket_closure"]["written_before_assertion_changes"]
                expected = row["candidate_body_sha256"]
                repaired_bodies.append({"file": key[0], "name": key[1],
                                       "validated_candidate_sha": row["candidate_sha"]})
            assert current[key]["body_sha256"] == expected, (key, "retained assertion changed")
        if row["decision"] is not None:
            for field in ["credible_failure", "production_owner", "callers", "remaining_coverage", "gap"]:
                assert field in row, (key, "missing semantic evidence", field)
    assert set(current) == {key for key, row in rows.items() if row["decision"] != "D"}
    decisions = dict(collections.Counter(row["decision"] or "unreviewed" for row in rows.values()))
    assert decisions == ledger["counts"]["decisions"], "Decision summary differs"
    assert source_lines == ledger["counts"]["baseline_test_lines"]
    assert source_lines - final_lines == ledger["counts"]["removed_test_lines"]
    assert len(current) == ledger["counts"]["final_declarations"]
    if not args.allow_pending:
        assert "unreviewed" not in decisions and ledger["audit_complete"], "Audit still pending"
    if args.require_review_receipts:
        assert not missing_review_receipts, ("Missing structured independent-review receipts", missing_review_receipts)
        assert ledger["independent_removal_review_complete"], "Final independent review incomplete"

    support_cleanup = []
    for entry in ledger["test_support_inventory"]["removed_helpers"]:
        source = git("show", base + ":" + entry["file"])
        pattern = re.compile(r"^func " + re.escape(entry["name"]) + r"\(", re.MULTILINE)
        match = pattern.search(source)
        assert match, (entry["name"], "original support helper missing")
        body = source[match.start():declaration_end(source, match.start())]
        assert hashlib.sha256(body.encode()).hexdigest() == entry["baseline_body_sha256"], entry["name"]
        candidate = root / entry["file"]
        assert not candidate.exists() or not pattern.search(candidate.read_text()), entry["name"]
        support_cleanup.append({"file": entry["file"], "name": entry["name"]})
    assert len(support_cleanup) == ledger["counts"]["removed_support_helpers"]

    production_cleanup = []
    for entry in ledger.get("production_seam_cleanup", []):
        original_source = git("show", base + ":" + entry["path"])
        assert hashlib.sha256(original_source.encode()).hexdigest() == entry["baseline_sha256"], entry["path"]
        source = (root / entry["path"]).read_bytes()
        assert hashlib.sha256(source).hexdigest() == entry["candidate_sha256"], entry["path"]
        production_cleanup.append({"path": entry["path"], "sha256": entry["candidate_sha256"],
                                   "removed_lines": entry["removed_lines"]})

    receipt = None
    if args.baseline_events:
        raw = args.baseline_events.read_bytes()
        events = {}
        for line in raw.splitlines():
            event = json.loads(line)
            if event.get("Test") and event.get("Action") in ("pass", "skip", "fail"):
                events[(event["Package"], event["Test"])] = event
        for row in rows.values():
            package = "github.com/MikeO7/kinosail/" + str(pathlib.PurePosixPath(row["file"]).parent)
            event = events.get((package, row["name"]))
            if event:
                assert row["baseline_result"] == event["Action"], (row["name"], "terminal status")
                assert row["baseline_event"] == event, (row["name"], "terminal receipt")
            else:
                assert row["baseline_result"] in ("unrun", "not_run"), (row["name"], "absent event")
        receipt = {"sha256": hashlib.sha256(raw).hexdigest(), "bytes": len(raw)}
    print(json.dumps({
        "source_head": git("rev-parse", "HEAD").strip(),
        "working_tree_status": git("status", "--porcelain").splitlines(),
        "baseline_sha": base,
        "baseline_files_verified": len(files),
        "baseline_declarations_verified": len(original),
        "retained_bodies_unchanged": len(current) - len(repaired_bodies),
        "existing_assertion_repairs_with_prior_failure_analysis": repaired_bodies,
        "removed_declarations_absent": decisions.get("D", 0),
        "removed_test_lines": source_lines - final_lines,
        "upstream_additions_preserved_unchanged": [{"file": key[0], "name": key[1]}
                                                  for key in preserved_upstream],
        "upstream_additions_absent_in_older_isolated_checkout": absent_upstream,
        "dead_support_helpers_verified_absent": support_cleanup,
        "production_seam_cleanup_verified": production_cleanup,
        "decisions": decisions,
        "removals_without_structured_review_receipt": missing_review_receipts,
        "baseline_event_receipt": receipt,
        "execution": "Read-only source/audit integrity verification; no Go or browser execution",
    }, indent=2))


if __name__ == "__main__":
    main()
