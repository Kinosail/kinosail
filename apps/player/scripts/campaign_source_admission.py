"""Project source drift without exporting arbitrary paths or file contents."""
import hashlib

MANIFESTS = frozenset({'go.work', 'go.work.sum', 'apps/player/go.mod', 'apps/player/go.sum',
                      'apps/subtitles/go.mod', 'apps/subtitles/go.sum', 'packages/go.mod',
                      'packages/go.sum', 'apps/player/e2e/package.json', 'apps/player/e2e/pnpm-lock.yaml'})


def pin(data):
    return {'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


def source_admission(root, git):
    raw = git('diff', '--name-only', '-z', 'HEAD', '--')
    if len(raw) > 1024 * 1024 or (raw and not raw.endswith(b'\0')):
        raise ValueError('invalid changed-source inventory')
    names = raw.split(b'\0')[:-1] if raw else []
    result = {'clean': not names, 'changedTrackedCount': len(names), 'knownManifests': [],
              'otherChangedTrackedCount': 0, 'changedPathsSHA256': hashlib.sha256(raw).hexdigest()}
    for name in names:
        decoded = name.decode('utf-8', errors='replace')
        if decoded not in MANIFESTS:
            result['otherChangedTrackedCount'] += 1
            continue
        path = root / decoded
        if path.is_symlink():
            working = {'symlink': True}
        elif not path.exists():
            working = {'missing': True}
        elif path.stat().st_size > 16 * 1024 * 1024:
            working = {'oversize': True}
        else:
            working = pin(path.read_bytes())
        original = git('show', 'HEAD:' + decoded)
        if len(original) > 16 * 1024 * 1024:
            raise ValueError('oversize manifest baseline')
        result['knownManifests'].append({'file': decoded, 'head': pin(original), 'working': working})
    return result
