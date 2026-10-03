#!/usr/bin/env python3
"""Verify all original Subtitles test dispositions against the current checkout.
No Go build or network access; original names stay in the ledger, while renamed_to
records existing tests whose names were corrected to reflect their actual assertions.
Canonical package/Player keeper presence is checked by the root cross-scope verifier.
"""
import collections
import json
import pathlib
import re
import subprocess

root = pathlib.Path(subprocess.check_output(['git', 'rev-parse', '--show-toplevel']).decode().strip())
ledger = json.loads((root / 'engineering/qa/2026-10-03-test-overhaul/subtitles-shared.json').read_text())
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
result = {'ledger_base_sha': ledger['base_sha'], 'checkout_sha': subprocess.check_output(['git', 'rev-parse', 'HEAD']).decode().strip(), 'decisions': dict(collections.Counter(row['decision'] for row in ledger['declarations_inventory'])), 'expected_retained': len(expected), 'actual_retained': len(actual), 'errors': errors}
print(json.dumps(result, indent=2))
raise SystemExit(bool(errors))
