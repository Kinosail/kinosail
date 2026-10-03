#!/usr/bin/env python3
"""Verify all original Subtitles test dispositions against the current checkout.
No Go build or network access; original names stay in the ledger, while renamed_to
records existing tests whose names were corrected to reflect their actual assertions.
Canonical package/Player keeper presence is checked by the root cross-scope verifier.
"""
import argparse
import collections
import hashlib
import runpy
import json
import pathlib
import re
import subprocess

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--root', type=pathlib.Path)
parser.add_argument('--ledger', type=pathlib.Path, help='Optional unmerged ledger for read-only candidate verification')
args = parser.parse_args()
root = args.root or pathlib.Path(subprocess.check_output(['git', 'rev-parse', '--show-toplevel']).decode().strip())
ledger_path = args.ledger or root / 'engineering/qa/2026-10-03-test-overhaul/subtitles-shared.json'
ledger = json.loads(ledger_path.read_text())
assert ledger['scope_complete'] and ledger['pending'] == 0
assert len(ledger['declarations_inventory']) == 954
expected = set()
errors = []
for row in ledger['declarations_inventory']:
    key = (row['file'], row.get('renamed_to', row['test']))
    if row['decision'] in ('R', 'F', 'C'):
        expected.add(key)
    else:
        assert row['decision'] == 'D'
actual = set()
for prefix in ['apps/subtitles/cmd', 'apps/subtitles/internal']:
    for path in (root / prefix).rglob('*_test.go'):
        for name in re.findall(r'^func ((?:Test|Fuzz|Benchmark)\w+)\(', path.read_text(), re.MULTILINE):
            actual.add((str(path.relative_to(root)), name))
for key in sorted(expected - actual):
    errors.append('retained declaration absent: ' + '::'.join(key))
for key in sorted(actual - expected):
    errors.append('deleted/unlisted declaration present: ' + '::'.join(key))
upstream_checks = []
appendix = ledger.get('upstream_preservation_appendix')
if appendix:
    functions = runpy.run_path(str(pathlib.Path(__file__).with_name('verify-go-audit-integrity.py')))['declarations']
    pinned = appendix['pinned_main_sha']
    merged = subprocess.run(['git', '-C', str(root), 'merge-base', '--is-ancestor', pinned, 'HEAD'], capture_output=True).returncode == 0
    if not merged:
        errors.append('pinned upstream main is not an ancestor of checkout HEAD: ' + pinned)
    for row in appendix['subtitles_go_changed_declarations'] + appendix['upstream_regressions_outside_this_scope']:
        path, name = row['file'], row['name']
        source = (root / path).read_text() if (root / path).is_file() else ''
        body = functions(source).get(name, '')
        expected_hash = row.get('upstream_body_sha256', row.get('body_sha256'))
        body_hash = hashlib.sha256(body.encode()).hexdigest()
        file_hash = hashlib.sha256(source.encode()).hexdigest()
        valid = body_hash == expected_hash and ('upstream_file_sha256' not in row or file_hash == row['upstream_file_sha256'])
        upstream_checks.append({'file': path, 'name': name, 'body_sha256': body_hash, 'file_sha256': file_hash, 'preserved_exact': valid})
        if not valid:
            errors.append('pinned upstream regression missing/changed: ' + path + '::' + name)
    if appendix['subtitles_go_added_declarations'] or appendix['new_isolated_tests_added_by_overhaul']:
        errors.append('unexpected new Subtitles isolated declarations in appendix')
else:
    errors.append('upstream preservation appendix absent')
result = {'upstream_checks': upstream_checks, 'ledger_sha256': hashlib.sha256(ledger_path.read_bytes()).hexdigest(), 'ledger_base_sha': ledger['base_sha'], 'checkout_sha': subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD']).decode().strip(), 'decisions': dict(collections.Counter(row['decision'] for row in ledger['declarations_inventory'])), 'expected_retained': len(expected), 'actual_retained': len(actual), 'errors': errors}
print(json.dumps(result, indent=2))
raise SystemExit(bool(errors))
