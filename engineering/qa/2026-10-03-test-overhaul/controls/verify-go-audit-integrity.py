#!/usr/bin/env python3
"""Read-only Go ledger/source integrity check; never builds or changes a checkout.

Usage: python3 .../verify-go-audit-integrity.py [--root CHECKOUT] [--output FILE]
Reports stale keeper references and all changed assertions for owner review. Unknown
changes are errors, not silently approved. C permits only documented consolidation;
Subtitles assertion repairs are pinned to their prewritten failure-analysis receipt.
"""
import argparse
import collections
import hashlib
import json
import pathlib
import re
import subprocess

BASE = '876b771a7dd0aef5e65957fcee87add0312535e8'
UPSTREAM = 'e8eca5f762822b239cf322065a915c87482fb8b0'
QA = 'engineering/qa/2026-10-03-test-overhaul/'
SPECS = [('player-backend.json', 'tests', 837), ('subtitles-shared.json', 'declarations_inventory', 954), ('shared-identity.json', 'declarations', 1900)]
DECL = re.compile(r'^func ((?:Test|Fuzz|Benchmark)\w+)\(', re.M)
PREFIXES = ('apps/player/cmd/', 'apps/player/internal/', 'apps/subtitles/cmd/', 'apps/subtitles/internal/', 'packages/')


def sha(data):
    return hashlib.sha256(data.encode() if isinstance(data, str) else data).hexdigest()


def without_comments(source):
    """Compare renamed bodies without treating revised lint prose as assertions."""
    tokens = re.compile(r'"(?:\\.|[^"\\])*"|\'(?:\\.|[^\'\\])*\'|`[^`]*`|//[^\n]*|/\*[\s\S]*?\*/')
    return tokens.sub(lambda m: '' if m[0].startswith(('//', '/*')) else m[0], source)


def declarations(source):
    """Exact func..closing-brace spans, excluding braces in Go strings/comments."""
    found = {}
    for match in DECL.finditer(source):
        depth, state, escaped, opened = 0, 'code', False, False
        i = match.end()
        while i < len(source):
            ch, pair = source[i], source[i:i + 2]
            if state in ('double', 'rune'):
                if escaped:
                    escaped = False
                elif ch == '\\':
                    escaped = True
                elif ch == ('"' if state == 'double' else "'"):
                    state = 'code'
            elif state == 'raw':
                if ch == '`':
                    state = 'code'
            elif state == 'line':
                if ch == '\n':
                    state = 'code'
            elif state == 'block':
                if pair == '*/':
                    state, i = 'code', i + 1
            elif pair in ('//', '/*'):
                state, i = ('line' if pair == '//' else 'block'), i + 1
            elif ch in ('"', "'", '`'):
                state = {'"': 'double', "'": 'rune', '`': 'raw'}[ch]
            elif ch == '{':
                opened, depth = True, depth + 1
            elif ch == '}':
                depth -= 1
                if opened and depth == 0:
                    found[match.group(1)] = source[match.start():i + 1]
                    break
            i += 1
        else:
            raise ValueError('unclosed declaration: ' + match.group(1))
    return found


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=pathlib.Path)
    parser.add_argument('--output', type=pathlib.Path)
    args = parser.parse_args()
    root = args.root or pathlib.Path(subprocess.check_output(['git', 'rev-parse', '--show-toplevel']).decode().strip())
    def git(*parts):
        return subprocess.check_output(['git', '-C', str(root), *parts])
    initial_sha = git('rev-parse', 'HEAD').decode().strip()
    initial_status = git('status', '--porcelain', '--untracked-files=no').decode().splitlines()
    cache = {}
    def blob(ref, path):
        key = (ref, path)
        if key not in cache:
            try:
                cache[key] = git('show', ref + ':' + path).decode()
            except subprocess.CalledProcessError:
                cache[key] = None
        return cache[key]
    def test_files(ref=None):
        names = git('ls-tree', '-r', '--name-only', ref).decode().splitlines() if ref else git('ls-files').decode().splitlines()
        return [n for n in names if n.endswith('_test.go') and n.startswith(PREFIXES)]
    baseline, actual, current_sources = {}, {}, {}
    for path in test_files(BASE):
        baseline.update({(path, n): b for n, b in declarations(blob(BASE, path)).items()})
    for path in test_files():
        source = (root / path).read_text()
        current_sources[path] = source
        actual.update({(path, n): b for n, b in declarations(source).items()})
    pinned = {}
    upstream_paths = git('diff', '--name-only', BASE, UPSTREAM, '--', '*_test.go').decode().splitlines()
    for path in upstream_paths:
        if path.startswith(PREFIXES):
            pinned.update({(path, n): b for n, b in declarations(blob(UPSTREAM, path)).items()})
    upstream_added = {k: b for k, b in pinned.items() if k not in baseline}
    upstream_changed = {k: b for k, b in pinned.items() if k in baseline and b != baseline[k]}
    errors, warnings, changes, counts, expected, normalized, file_hashes = [], [], [], {}, set(), {}, {}
    pins = json.loads(pathlib.Path(__file__).with_name('approved-go-body-changes.json').read_text())
    approved = {(r['file'], r['name']): r for r in pins['body_changes']}
    analysis_checks = []
    for group, receipt in pins['analysis_receipts'].items():
        def analysis_digest(source):
            if 'json_path' not in receipt:
                return sha(source)
            value = json.loads(source)
            for key in receipt['json_path']:
                value = value[key]
            return sha(json.dumps({k: value[k] for k in receipt['analysis_keys']}, sort_keys=True, separators=(',', ':')))
        before = blob(receipt['analysis_commit'], receipt['path'])
        now = (root / receipt['path']).read_text()
        valid = before is not None and analysis_digest(before) == receipt['sha256'] == analysis_digest(now)
        for older, newer in [(receipt['analysis_commit'], receipt['repair_commit']), (receipt['repair_commit'], 'HEAD')]:
            valid &= subprocess.run(['git', '-C', str(root), 'merge-base', '--is-ancestor', older, newer], capture_output=True).returncode == 0
        if 'helper_path' in receipt:
            valid &= sha((root / receipt['helper_path']).read_bytes()) == receipt['helper_file_sha256']
        valid &= (root / receipt['validation_receipt']).is_file()
        analysis_checks.append({'group': group, 'valid': valid, **receipt})
        if not valid:
            errors.append('missing/changed pre-edit analysis or helper receipt: ' + group)
    for filename, key, size in SPECS:
        ledger = json.loads((root / QA / filename).read_text())
        rows = ledger[key]
        if len(rows) != size:
            errors.append(f'{filename}: expected{size} rows, got{len(rows)}')
        counts[filename] = dict(collections.Counter(r.get('decision') for r in rows))
        for file in ledger.get('files', []) if isinstance(ledger.get('files'), list) else []:
            path = file.get('path', file.get('file'))
            digest = file.get('baseline_sha256', file.get('sha256'))
            if path and digest:
                file_hashes[path] = digest
        for row in rows:
            path = row.get('file', row.get('path'))
            name = row.get('test', row.get('name'))
            ident = (path, name)
            if ident in normalized:
                errors.append('duplicate ledger identity: ' + '::'.join(ident))
            normalized[ident] = row
            decision = row.get('decision')
            if decision not in ('R', 'D', 'F', 'C'):
                errors.append(f'unreviewed row: {path}::{name}')
            old = baseline.get(ident)
            if old is None:
                errors.append(f'baseline declaration absent: {path}::{name}')
                continue
            digest = row.get('body_sha256', row.get('declaration_sha256', row.get('sha256')))
            if digest and sha(old) != digest:
                errors.append(f'baseline body hash mismatch: {path}::{name}')
            digest = row.get('baseline_file_sha256', row.get('source_file_sha256', row.get('source_file_checksum')))
            if digest:
                file_hashes[path] = digest
            new_name = row.get('renamed_to', name)
            current = (path, new_name)
            if decision == 'D':
                if ident in actual:
                    errors.append(f'D still present: {path}::{name}')
                continue
            expected.add(current)
            if current not in actual:
                errors.append(f'{decision} missing: {path}::{new_name}')
                continue
            body = actual[current]
            if new_name != name:
                body = body.replace('func ' + new_name + '(', 'func ' + name + '(', 1)
            if body == old:
                continue
            kind = 'unexpected_assertion_change'
            if current in upstream_changed and actual[current] == upstream_changed[current]:
                kind = 'pinned_upstream_change'
            elif new_name != name and without_comments(body) == without_comments(old):
                kind = 'existing_name_and_comment_correction_only'
            elif ident in approved and sha(actual[current]) == approved[ident]['candidate_body_sha256'] and sha(old) == approved[ident]['baseline_body_sha256']:
                kind = approved[ident]['kind']
            changes.append({'file': path, 'name': name, 'kind': kind, 'baseline_body_sha256': sha(old), 'candidate_body_sha256': sha(actual[current])})
            if kind == 'unexpected_assertion_change':
                errors.append(f'unapproved changed assertion: {path}::{name}')
    for path, digest in file_hashes.items():
        source = blob(BASE, path)
        if source is None or sha(source) != digest:
            errors.append('baseline file hash mismatch: ' + path)
    if set(baseline) != set(normalized):
        errors.append('baseline inventory differs from normalized837/954/1900 ledger identities')
    for ident, body in upstream_added.items():
        expected.add(ident)
        if actual.get(ident) != body:
            errors.append('upstream regression absent/changed: ' + '::'.join(ident))
    for ident in sorted(set(actual) - expected):
        errors.append('unlisted test declaration: ' + '::'.join(ident))
    # Explicit path/scenario refs and name-only proof refs are verified, never guessed.
    refs, resolved = set(), []
    names = re.compile(r'\b(?:Test|Fuzz|Benchmark)\w+(?:/(?:Test|Fuzz|Benchmark)\w+)*')
    explicit = re.compile(r'((?:apps|packages)/[\w./-]+\.go)(?:::|:)(?:(?:\d+):)?((?:Test|Fuzz|Benchmark)\w+(?:/(?:Test|Fuzz|Benchmark)\w+)*)')
    def proof_refs(value):
        if isinstance(value, list):
            return sum((proof_refs(v) for v in value), [])
        if isinstance(value, dict):
            path = value.get('path', value.get('file'))
            if path and path.endswith('.go'):
                return [(path, n) for n in names.findall(str(value.get('scenario', value.get('name', value.get('test', '')))))]
            return sum((proof_refs(v) for v in value.values() if isinstance(v, (str, list, dict))), [])
        if not isinstance(value, str):
            return []
        pairs = explicit.findall(value)
        rest = explicit.sub('', value)
        return pairs + [(None, n) for n in names.findall(rest)]
    def resolve(path, name):
        parent, _, child = name.partition('/')
        matches = [(p, n) for p, n in actual if n == parent and (path is None or p == path)]
        if not child:
            return matches
        peers = [[(p, n) for p, n in actual if n == part and (path is None or p == path)] for part in name.split('/')]
        if all(peers):
            return sum(peers, [])  # Prose may slash-separate independent top-level tests.
        for p, n in matches:
            if f't.Run("{child}"' in actual[(p, n)]:
                return [(p, name)]
            # Existing CLIContract binds RunCLI -> CLI.Run -> named MCPOwner callback.
            helper = root / 'packages/commandtest/cli.go'
            if parent == 'TestCLIContract' and 'commandtest.RunCLI(t,' in actual[(p, n)] and helper.is_file() and f't.Run("{child}"' in helper.read_text():
                return [(p, name)]
        return []
    for ident, row in normalized.items():
        if row.get('decision') != 'D':
            continue
        for path, name in proof_refs([row.get('remaining_proof', []), row.get('remaining_coverage', [])]):
            refs.add((ident, path, name))
    for ident, path, name in sorted(refs, key=str):
        matches = resolve(path, name)
        if matches:
            resolved.append({'removed': '::'.join(ident), 'keeper': (path + '::' if path else '') + name, 'matches': ['::'.join(k) for k in matches]})
        else:
            warnings.append({'removed': '::'.join(ident), 'stale_keeper': (path + '::' if path else '') + name})
    protected = []
    for path in git('diff', '--name-only', BASE, UPSTREAM).decode().splitlines():
        pinned_source = blob(UPSTREAM, path)
        disk = root / path
        if pinned_source is None:
            continue
        same = disk.exists() and sha(disk.read_bytes()) == sha(pinned_source)
        protected.append({'file': path, 'pinned_sha256': sha(pinned_source), 'preserved_exact': same})
        if not same and path not in ('apps/player/docs/architecture-explorer/index.html', 'apps/subtitles/docs/architecture-explorer/index.html', 'apps/player/scripts/test-container.sh', 'apps/subtitles/scripts/test-container.sh'):
            errors.append('upstream source absent/changed: ' + path)
    final_sha = git('rev-parse', 'HEAD').decode().strip()
    final_status = git('status', '--porcelain', '--untracked-files=no').decode().splitlines()
    if (initial_sha, initial_status) != (final_sha, final_status):
        errors.append('checkout changed while verifier was reading; rerun on a stable checkout')
    if final_status:
        errors.append('tracked working tree is dirty; receipt cannot claim exact checkout SHA')
    result = {'schema': 1, 'base_sha': BASE, 'upstream_sha': UPSTREAM, 'checkout_sha': git('rev-parse', 'HEAD').decode().strip(), 'tracked_status': git('status', '--porcelain', '--untracked-files=no').decode().splitlines(), 'verifier_sha256': sha(pathlib.Path(__file__).read_bytes()), 'approved_changes_sha256': sha(pathlib.Path(__file__).with_name('approved-go-body-changes.json').read_bytes()), 'counts': counts, 'original_declarations': len(normalized), 'actual_declarations': len(actual), 'upstream_added': [{'file': k[0], 'name': k[1], 'body_sha256': sha(b)} for k, b in upstream_added.items()], 'upstream_changed': [{'file': k[0], 'name': k[1], 'body_sha256': sha(b)} for k, b in upstream_changed.items()], 'changed_assertions': changes, 'pre_edit_analysis_checks': analysis_checks, 'upstream_source_preservation': protected, 'verified_keeper_references': resolved, 'stale_keeper_references': warnings, 'errors': errors, 'limits': 'Source integrity only; no builds/tests executed. Name-only proof refs show all physical matches and do not prove semantic equivalence. Named CLI subtests resolve through the existing RunCLI/CLI.Run binding. Free-text gaps and unnamed browser scenarios remain manual review. Four root-maintained docs/container artifact harness files may differ from pinned main and are reported.'}
    encoded = json.dumps(result, indent=2) + '\n'
    if args.output:
        args.output.write_text(encoded)
    print(json.dumps({k: result[k] for k in ('checkout_sha', 'original_declarations', 'actual_declarations', 'upstream_added', 'upstream_changed', 'changed_assertions', 'stale_keeper_references', 'errors')}, indent=2))
    return bool(errors or warnings)


if __name__ == '__main__':
    raise SystemExit(main())
