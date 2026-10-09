"""Bounded controls for disposable Version1 public HTTP cache fixtures."""
import hashlib
import json
import re
import urllib.error
import urllib.request
from hls_followon_public import bounded_bytes, check


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
    directories = [p for p in cache.iterdir() if p.is_dir()]
    check(len(directories) == 1, 'compat_generation_count')
    directory = directories[0]
    target = {'missing-source': '.source', 'wrong-source': '.source',
        'missing-clock': '.copy-clock', 'wrong-version': '.copy-clock',
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
                'invalid-legacy-generation'])
        rows.append({'boundedFieldsAndCorrelationValid': valid})
    return {'requestIDsValid': all(re.fullmatch(r'[a-zA-Z0-9_-]{8,96}', v) for v in request_ids),
        'rows': rows}


def require_diagnostics(result, count):
    check(result['requestIDsValid'] and len(result['rows']) == count
        and all(v['boundedFieldsAndCorrelationValid'] for v in result['rows']), 'compat_rejection_diagnostic')
