#!/usr/bin/env python3
"""Enforce 300 lines per source file, with frozen pre-existing cleanup debt."""

import argparse
from contextlib import contextmanager
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
SUFFIXES = {'.go', '.ts', '.tsx', '.js', '.mjs', '.py', '.sh', '.css', '.html', '.swift'}
EXCLUDED = {'.codex', 'third_party', 'testdata', 'docs', 'engineering', 'assets'}
# Remove each exception when its responsibility-based refactor lands. Never grow it.
LEGACY = {
    'apps/dashboard/internal/server/web/static/dashboard.css': 316,
    'apps/player/apps/native/Sources/Design/MediaViews.swift': 363,
    'apps/player/apps/native/Sources/Platform/OfflineDownloadManager.swift': 347,
    'apps/player/apps/native/Sources/Platform/PlaybackCoordinator.swift': 616,
    'apps/player/e2e/player-direct-fallback.spec.ts': 313,
    'apps/player/e2e/player-experience.spec.ts': 311,
    'apps/subtitles/internal/server/static/subtitle-dashboard.css': 342,
    'packages/webassets/static/downloads.js': 335,
    'packages/webassets/static/player-app.css': 304,
    'packages/webassets/static/player-controls.js': 396,
}


def git(*args):
    return subprocess.check_output(['git', '-C', str(ROOT), *args])


def line_limit(value):
    if not re.fullmatch(r'[1-9][0-9]{0,3}', value):
        raise argparse.ArgumentTypeError('line limit must be an integer from 1 to 9999')
    return int(value)


def commit_id(value):
    if not re.fullmatch(r'[0-9a-f]{40}|[0-9a-f]{64}', value):
        raise argparse.ArgumentTypeError('revision must be a full Git object ID')
    return value


@contextmanager
def blobs():
    with subprocess.Popen(['git', '-C', str(ROOT), 'cat-file', '--batch'],
                          stdin=subprocess.PIPE, stdout=subprocess.PIPE) as process:
        def read(object_id):
            process.stdin.write(object_id + b'\n')
            process.stdin.flush()
            header = process.stdout.readline().split()
            if len(header) != 3 or header[1] != b'blob':
                raise ValueError('cannot read source blob')
            content = process.stdout.read(int(header[2]))
            if process.stdout.read(1) != b'\n':
                raise ValueError('incomplete source blob')
            return content
        try:
            yield read
        finally:
            process.stdin.close()


def sources(mode):
    if mode and mode != 'staged':
        entries = git('ls-tree', '-rz', 'HEAD' if mode == 'head' else mode).split(b'\0')
        for entry in filter(None, entries):
            metadata, path = entry.split(b'\t', 1)
            if metadata.split()[0] in (b'100644', b'100755'):
                yield path.decode(), metadata.split()[2]
    elif mode == 'staged':
        for entry in filter(None, git('ls-files', '--stage', '-z').split(b'\0')):
            metadata, path = entry.split(b'\t', 1)
            permissions, object_id, stage = metadata.split()
            if stage != b'0':
                raise ValueError('resolve unmerged files before checking the line cap')
            if permissions in (b'100644', b'100755'):
                yield path.decode(), object_id
    else:
        paths = git('ls-files', '-co', '--exclude-standard', '-z').split(b'\0')
        for raw in sorted(set(filter(None, paths))):
            path = raw.decode()
            if (ROOT / path).is_file() and not (ROOT / path).is_symlink():
                yield path, None


def check(args, read_blob):
    failures = 0
    debt = 0
    for name, revision in sources(args.mode):
        path = Path(name)
        if args.scope and not name.startswith(args.scope + '/'):
            continue
        if args.go_only:
            if path.suffix != '.go':
                continue
        elif path.suffix not in SUFFIXES or EXCLUDED.intersection(path.parts):
            continue
        content = read_blob(revision) if revision else (ROOT / path).read_bytes()
        lines = content.splitlines()
        if path.suffix == '.go' and any(
            re.fullmatch(rb'// Code generated .* DO NOT EDIT\.', line) for line in lines[:20]
        ):
            continue
        maximum = args.limit if args.strict else max(args.limit, LEGACY.get(name, 0))
        if len(lines) > maximum:
            print(f'{name}: {len(lines)} lines (maximum {maximum})', file=sys.stderr)
            failures += 1
        elif len(lines) > args.limit:
            debt += 1
    if debt:
        print(f'Line cap: {debt} existing oversized files remain; frozen allowances apply.')
    if failures:
        print('Split oversized files by responsibility.', file=sys.stderr)
    return bool(failures)


def main():
    parser = argparse.ArgumentParser(description=__doc__, allow_abbrev=False)
    parser.add_argument('--scope', choices=('packages', 'apps/player', 'apps/subtitles', 'apps/dashboard'))
    parser.add_argument('--limit', type=line_limit, default=300)
    parser.add_argument('--go-only', action='store_true')
    parser.add_argument('--strict', action='store_true', help='also reject pre-existing oversized files')
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument('--staged', dest='mode', action='store_const', const='staged')
    mode.add_argument('--head', dest='mode', action='store_const', const='head')
    mode.add_argument('--revision', dest='mode', type=commit_id)
    args = parser.parse_args()
    try:
        with blobs() as read_blob:
            return check(args, read_blob)
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f'Line cap could not run: {error}', file=sys.stderr)
        return 2


if __name__ == '__main__':
    sys.exit(main())
