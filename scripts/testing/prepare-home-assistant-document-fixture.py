#!/usr/bin/env python3
"""Copy the approved immutable fictional clip into a new disposable fixture."""
import argparse
import hashlib
import json
from pathlib import Path

FIXTURE_SHA256 = '9dbd85e7863d921e209978c8349992e5f8505b0d7ffa468c37b8caf97af3f0a4'


def prepare(directory):
    source = Path(__file__).resolve().parents[2] / 'apps/player/e2e/fixtures/r18-example.mp4'
    if source.is_symlink() or source.stat().st_size != 396548:
        raise ValueError('approved fictional fixture size is invalid')
    clip = source.read_bytes()
    if hashlib.sha256(clip).hexdigest() != FIXTURE_SHA256:
        raise ValueError('approved fictional fixture identity is invalid')
    # mkdir refuses existing directories and symlinks; historical media is preserved.
    directory.mkdir()
    for title in ('R18 Fictional Alpha', 'R18 Fictional Beta'):
        (directory / (title + '.mp4')).write_bytes(clip)
    return {'fixture': 'R18 immutable fictional documents', 'bytes': len(clip),
            'sha256': FIXTURE_SHA256, 'copies': 2}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory', type=Path, help='New directory under the disposable media root')
    args = parser.parse_args()
    print(json.dumps(prepare(args.directory), sort_keys=True))
