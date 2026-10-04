"""Compare retained Go declarations in exact Git trees without compiling them."""
from pathlib import Path
import argparse
import collections
import difflib
import hashlib
import json
import re
import subprocess

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("sha", help="Exact integrated commit to inspect")
parser.add_argument("--repo", type=Path, default=Path.cwd())
parser.add_argument("--output", type=Path, required=True)
args = parser.parse_args()
repo = args.repo.resolve()
head = subprocess.check_output(["git", "rev-parse", args.sha + "^{commit}"], cwd=repo, text=True).strip()
base = "876b771a7dd0aef5e65957fcee87add0312535e8"
cache = {}


def blob(rev, path):
    key = (rev, path)
    if key not in cache:
        result = subprocess.run(["git", "show", rev + ":" + path], cwd=repo, capture_output=True)
        cache[key] = result.stdout.decode() if result.returncode == 0 else None
    return cache[key]


def declaration(source, name):
    if source is None:
        return None
    match = re.search(r"^func " + re.escape(name) + r"\(", source, re.M)
    if not match:
        return None
    index = source.find("{", match.end())
    if index == -1:
        return None
    depth, state = 0, "code"
    while index < len(source):
        char, next_char = source[index], source[index:index + 2]
        if state == "line":
            if char == "\n":
                state = "code"
        elif state == "block":
            if next_char == "*/":
                state = "code"
                index += 1
        elif state == "raw":
            if char == "`":
                state = "code"
        elif state in ('"', "'"):
            if char == "\\":
                index += 1
            elif char == state:
                state = "code"
        elif next_char == "//":
            state = "line"
            index += 1
        elif next_char == "/*":
            state = "block"
            index += 1
        elif char == "`":
            state = "raw"
        elif char in ('"', "'"):
            state = char
        elif char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                return source[match.start():index + 1]
        index += 1
    return None


def digest(value):
    return hashlib.sha256(value.encode()).hexdigest()


specs = {
    "player-backend.json": "tests",
    "subtitles-shared.json": "declarations_inventory",
    "shared-identity.json": "declarations",
    "player-nonserver.json": "tests",
}
seen, changes, gaps = {}, [], []
for filename, key in specs.items():
    ledger = json.loads(blob(head, "engineering/qa/2026-10-03-test-overhaul/" + filename))
    for row in ledger[key]:
        path = row.get("file", row.get("path"))
        name = row.get("name", row.get("test"))
        if not path or not path.endswith("_test.go") or not name or row.get("decision") == "D":
            continue
        current_name = row.get("renamed_to") or name
        if (path, current_name) in seen:
            continue
        original = declaration(blob(base, path), name)
        current = declaration(blob(head, path), current_name)
        entry = {"file": path, "name": name, "current_name": current_name, "decision": row.get("decision")}
        seen[(path, current_name)] = entry
        if original is None or current is None:
            gaps.append({**entry, "baseline_body_found": original is not None, "current_body_found": current is not None})
            continue
        normalized = current.replace("func " + current_name + "(", "func " + name + "(", 1)
        if normalized != original:
            changes.append({
                **entry,
                "baseline_body_sha256": digest(original),
                "current_body_sha256": digest(current),
                "diff_after_explicit_name_normalization": "\n".join(difflib.unified_diff(
                    original.splitlines(), normalized.splitlines(), fromfile=base + ":" + path,
                    tofile=head + ":" + path, lineterm="")),
            })

report = {
    "schema": "kinosail-retained-body-inventory-v1",
    "reviewer": "frontend_native",
    "source_sha": head,
    "baseline_sha": base,
    "retained_declarations_compared": len(seen),
    "decision_counts": dict(collections.Counter(row["decision"] for row in seen.values())),
    "changed_retained_bodies": changes,
    "body_extraction_gaps": gaps,
    "failure_analysis": [
        "Physical name presence alone could hide removed assertions; compare retained declaration bodies.",
        "Honest renames could create false loss reports; normalize only the ledger's explicit renamed_to declaration header.",
        "Braces in comments, quoted strings or raw fixtures and one-line functions could create false differences; scan lexical states.",
        "Changes need manual semantic review; this script does not automatically classify assertion equivalence.",
    ],
    "limits": ["No builds or runtime/E2E equivalence claim.", "Compares baseline ledger Go declarations only; new upstream declarations appear in the physical inventory."],
    "reproduce": {
        "command": "python3 " + Path(__file__).name + " " + head + " --repo <checkout> --output <receipt.json>",
        "generator_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
    },
}
args.output.write_text(json.dumps(report, indent=2) + "\n")
print(json.dumps({"path": str(args.output), "sha256": hashlib.sha256(args.output.read_bytes()).hexdigest(),
                  "compared": len(seen), "changed": len(changes), "gaps": len(gaps)}))
