"""Bounded controls for disposable Version1 public HTTP cache fixtures."""
import hashlib
import json
import re
import urllib.error
import urllib.request
from hls_followon_public import bounded_bytes, check
from hls_timeline_packets import manifest_facts


def snapshot(cache):
    result = {}
    paths = sorted(cache.rglob('*'))
    check(len(paths) <= 96, 'binding_cache_path_bound')
    for path in paths:
        if path.is_dir():
            continue
        check(path.is_file() and not path.is_symlink(), 'binding_cache_kind')
        stat = path.stat()
        result[str(path.relative_to(cache))] = {'inode': stat.st_ino, 'bytes': stat.st_size,
            'mtimeNs': stat.st_mtime_ns, 'sha256': hashlib.sha256(
                bounded_bytes(path, 2 << 20, 'binding_cache_bytes_bound')).hexdigest()}
    return result



def http(owner, target, method, ranged=False):
    if not ranged:
        return owner.api.http(target, method=method)
    request = urllib.request.Request(owner.api.url + target, method=method,
        headers={'Authorization': 'Bearer ' + owner.api.token, 'Range': 'bytes=7-31'})
    try:
        response = owner.api.opener.open(request, timeout=40)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        body = response.read((2 << 20) + 1)
        check(len(body) <= 2 << 20, 'compat_range_response_bound')
        return response.getcode(), body, dict(response.headers)


def requests(asset, method, selected):
    prefix = selected.removesuffix('index.m3u8') + '360p/'
    names = {'master': selected, 'rendition': prefix + 'index.m3u8',
        'init': prefix + 'init.mp4', 'first': prefix + 'segment-00000.m4s',
        'last': prefix + 'segment-00009.m4s'}
    if asset == 'journey':
        return [(selected, 'GET', False), (prefix + 'index.m3u8', 'GET', False),
            (prefix + 'init.mp4', 'GET', False),
            *[(prefix + 'segment-' + str(n).zfill(5) + '.m4s', 'GET', False) for n in range(10)]]
    return [(names[asset], 'GET' if method == 'RANGE' else method, method == 'RANGE')]


def responses(owner, steps):
    result = []
    for target, method, ranged in steps:
        status, body, headers = http(owner, target, method, ranged)
        result.append((status, body, headers))
    return result


def fault(cache, name):
    timelines = sorted(cache.glob('*/.copy-timeline'))
    check(len(timelines) == 1, 'compat_indexed_generation_count')
    timeline = timelines[0]
    directory = timeline.parent
    check(timeline.is_file() and not timeline.is_symlink()
        and directory.is_dir() and not directory.is_symlink(), 'compat_indexed_generation_kind')
    binding = directory / '.source'
    check(binding.is_file() and not binding.is_symlink(), 'compat_seed_binding_kind')
    check(json.loads(bounded_bytes(timeline, 256 << 10, 'compat_seed_timeline_bound'))['Policy']
        == bounded_bytes(binding, 16 << 10, 'compat_seed_binding_bound').decode(), 'compat_seed_binding_exact')
    target = {'missing-source': '.source', 'wrong-source': '.source',
        'missing-clock': '.copy-clock', 'missing-timeline': '.copy-timeline', 'wrong-version': '.copy-clock',
        'wrong-timeline': '.copy-timeline', 'wrong-master': 'index.m3u8',
        'wrong-init': '360p/init.mp4', 'wrong-first': '360p/segment-00000.m4s'}[name]
    path = directory / target
    if name.startswith('missing-'):
        path.unlink()
    elif name == 'wrong-version':
        value = json.loads(bounded_bytes(path, 4096, 'compat_certificate_bound'))
        value['version'] = 2
        path.write_text(json.dumps(value, separators=(',', ':')))
    else:
        data = bytearray(bounded_bytes(path, 2 << 20, 'compat_fault_bound'))
        check(len(data) > 16, 'compat_fault_minimum')
        data[len(data) // 2] ^= 1
        path.write_bytes(data)


def diagnostics(owner, replies):
    request_ids = [next((v for k, v in headers.items() if k.lower() == 'x-request-id'), '')
        for _, _, headers in replies]
    rows = []
    for line in bounded_bytes(owner.log_path, 2 << 20, 'compat_private_log_bound').splitlines():
        try:
            entry = json.loads(line)
        except json.JSONDecodeError:
            continue
        if entry.get('request_id') not in request_ids or entry.get('msg') != 'HLS copied playlist rejected':
            continue
        valid = (set(entry) == {'time', 'level', 'msg', 'request_id', 'playback_session', 'failure_class'}
            and entry.get('level') == 'WARN' and entry.get('playback_session') == ''
            and entry.get('failure_class') in ['invalid-source-binding', 'invalid-generation-or-manifest',
                'invalid-legacy-generation', 'invalid-start'])
        rows.append({'boundedFieldsAndCorrelationValid': valid})
    return {'requestIDsValid': all(re.fullmatch(r'[a-zA-Z0-9_-]{8,96}', v) for v in request_ids),
        'rows': rows}


def require_diagnostics(result, count):
    check(result['requestIDsValid'] and len(result['rows']) == count
        and all(v['boundedFieldsAndCorrelationValid'] for v in result['rows']), 'compat_rejection_diagnostic')


def query_controls():
    return [
        ('start-valid', 'start=3', 200),
        ('start-last-integer', 'start=31', 200),
        ('session-start', 'playSessionId=PublicCompatSession0123&start=3', 200),
        ('start-empty', 'start=', 400),
        ('start-zero', 'start=0', 400),
        ('start-negative', 'start=-1', 400),
        ('start-duplicate', 'start=1&start=2', 400),
        ('start-nonnumeric', 'start=abc', 400),
        ('start-oversized', 'start=1234567890', 400),
        ('start-at-source-end', 'start=32', 400),
        ('start-above-source-end', 'start=33', 400),
    ]


def seed_prefix(cache, witness):
    timelines = sorted(cache.glob('*/.copy-timeline'))
    check(len(timelines) == 1, 'compat_prefix_indexed_generation_count')
    directory = timelines[0].parent
    timeline = json.loads(bounded_bytes(timelines[0], 256 << 10, 'compat_prefix_timeline_bound'))
    data = bounded_bytes(directory / '360p/index.m3u8', 256 << 10, 'compat_prefix_manifest_bound')
    witness['physicalManifestSHA256'] = hashlib.sha256(data).hexdigest()
    facts, fragments = manifest_facts(data)
    raw_event = b'#EXT-X-PLAYLIST-TYPE:EVENT' in data.splitlines()
    names = ['segment-' + str(n).zfill(5) + '.m4s' for n in range(10)]
    physical = [name for name in ['init.mp4', *names] if (directory / '360p' / name).is_file()]
    witness.update(physicalManifestCuts=len(fragments), physicalManifestEndlist=facts['endlist'],
        physicalPlaylistType=facts['playlistType'], rawEventTypeLine=raw_event, certifiedCuts=len(timeline['Keys']),
        orderedPhysicalPrefix=[name for name, _ in fragments], physicalAssetNames=physical,
        allCertifiedAssetsPhysical=len(physical) == 11)
    check(raw_event and facts['playlistType'] == 'EVENT' and not facts['endlist']
        and 0 < len(fragments) < len(timeline['Keys']) == 10, 'compat_real_physical_event_prefix')
    check([name for name, _ in fragments] == names[:len(fragments)], 'compat_physical_prefix_order')
    check(len(physical) == 11, 'compat_prefix_all_certified_assets_physical')
    check(timeline['Policy'] == bounded_bytes(directory / '.source', 16 << 10,
        'compat_prefix_binding_bound').decode(), 'compat_prefix_whole_binding')
