"""Written-first actual copied-master controls for disposable V1 and V2 caches."""
import hashlib
import re
import shutil
from hls_aac_v2_compat_public import diagnostics, requests, responses, require_diagnostics, snapshot
from hls_aac_v2_lazy_public import clone_arm, selected_path
from hls_aac_v2_public_http import diagnostic_producer_rows, idle
from hls_followon_public import bounded_bytes, check
from hls_timeline_http import source_state

FAULTS = ['damaged-marker', 'missing-stream', 'malformed-bandwidth', 'duplicate-bandwidth',
    'duplicate-stream', 'unknown-tag', 'wrong-codec', 'wrong-resolution',
    'wrong-range', 'unknown-attribute', 'missing-codecs', 'duplicate-rendition']


def master_path(cache):
    paths = list(cache.glob('*/.copy-timeline'))
    check(len(paths) == 1, 'master_single_indexed_generation')
    return paths[0].with_name('index.m3u8')


def damage(cache, fault):
    path = master_path(cache)
    data = bounded_bytes(path, 256 << 10, 'master_fault_bound')
    text = data.decode('ascii')
    lines = text.splitlines(keepends=True)
    selected = [n for n, line in enumerate(lines) if line.startswith('#EXT-X-STREAM-INF:')]
    check(len(selected) == 1, 'master_real_single_stream_control')
    index = selected[0]
    original = lines[index]
    if fault == 'damaged-marker':
        check('#KINOSAIL-BANDWIDTH:2\n' in lines, 'master_bandwidth_marker_control')
        lines[lines.index('#KINOSAIL-BANDWIDTH:2\n')] = '#KINOSAIL-BANDWIDTH:3\n'
    elif fault == 'missing-stream':
        lines[index] = ''
    elif fault == 'malformed-bandwidth':
        lines[index], count = re.subn(r'^#EXT-X-STREAM-INF:BANDWIDTH=[1-9][0-9]*',
            '#EXT-X-STREAM-INF:BANDWIDTH=oops', original)
        check(count == 1, 'master_bandwidth_fault_control')
    elif fault == 'duplicate-bandwidth':
        lines[index] = original.rstrip('\n') + ',BANDWIDTH=1\n'
    elif fault == 'duplicate-stream':
        lines.insert(index, original)
    elif fault == 'unknown-tag':
        lines.insert(index, '#EXT-X-UNKNOWN:1\n')
    elif fault == 'wrong-codec':
        lines[index], count = re.subn(r'CODECS="avc1\.', 'CODECS="avc3.', original)
        check(count == 1, 'master_codec_fault_control')
    elif fault == 'wrong-resolution':
        lines[index], count = re.subn(r'RESOLUTION=([1-9][0-9]*)x([1-9][0-9]*)',
            lambda match: 'RESOLUTION=' + str(int(match[1]) + 1) + 'x' + match[2], original)
        check(count == 1, 'master_geometry_fault_control')
    elif fault == 'wrong-range':
        check('VIDEO-RANGE=SDR' in original, 'master_range_fault_control')
        lines[index] = original.replace('VIDEO-RANGE=SDR', 'VIDEO-RANGE=HLG')
    elif fault == 'unknown-attribute':
        lines[index] = original.rstrip('\n') + ',X-UNKNOWN=1\n'
    elif fault == 'missing-codecs':
        lines[index], count = re.subn(r',CODECS="[^"]+"', '', original)
        check(count == 1, 'master_missing_codec_fault_control')
    elif fault == 'duplicate-rendition':
        lines.insert(index + 2, lines[index + 1])
    else:
        raise RuntimeError('master_unknown_fault')
    mutated = ''.join(lines).encode('ascii')
    check(mutated != data and 0 < len(mutated) <= 256 << 10, 'master_fault_not_applied')
    path.write_bytes(mutated)
    return {'fault': fault, 'originalSHA256': hashlib.sha256(data).hexdigest(),
        'mutatedSHA256': hashlib.sha256(mutated).hexdigest(), 'bytes': len(mutated)}


def reject(root, run, label, binary, source, seed, selected, owners, before_source, row):
    directory = run / label
    directory.mkdir()
    shutil.copytree(seed, directory / 'cache')
    row['faultControl'] = damage(directory / 'cache', row['fault'])
    # clone_arm also snapshots before Server start. Use the damaged immutable clone.
    owner = clone_arm(root, run, label + '-owned', binary, source, directory / 'cache', owners)
    cache = owner.directory / 'cache'
    row.update(cacheUnchangedAfterStartup=snapshot(cache) == owner.clone_snapshot,
        startupSourceCalls=len(owner.source_invocation_rows()))
    check(row['cacheUnchangedAfterStartup'] and row['startupSourceCalls'] == 0,
        'master_invalid_startup_mutation')
    row['requestIdentityMatched'] = selected_path(owner)[1] == selected
    row.update(cacheUnchangedAfterPlanning=snapshot(cache) == owner.clone_snapshot,
        planningSourceCalls=len(owner.source_invocation_rows()))
    check(row['requestIdentityMatched'] and row['cacheUnchangedAfterPlanning']
        and row['planningSourceCalls'] == 0, 'master_invalid_planning_mutation')
    steps = [*requests('master', 'GET', selected), *requests('master', 'HEAD', selected),
        *requests('rendition', 'GET', selected), *requests('rendition', 'HEAD', selected)]
    replies = responses(owner, steps)
    row.update(statuses=[v[0] for v in replies],
        immediateCacheUnchanged=snapshot(cache) == owner.clone_snapshot,
        sourceCalls=len(owner.source_invocation_rows()))
    row['idle'] = idle(owner.api, owner.process, source)
    row['diagnostics'] = diagnostics(owner, replies)
    at_idle = snapshot(cache)
    owner.stop()
    row.update(finalCacheUnchanged=snapshot(cache) == owner.clone_snapshot,
        joinedCacheUnchangedFromIdle=snapshot(cache) == at_idle,
        finalSourceCalls=len(owner.source_invocation_rows()), sourceUnchanged=source_state(source) == before_source,
        sessions=owner.sessions,
        sourceAudit=diagnostic_producer_rows(owner.invocation_rows(), source, owner.owned_pids))
    check(row['immediateCacheUnchanged'] and row['finalCacheUnchanged']
        and row['joinedCacheUnchangedFromIdle'] and row['sourceCalls'] == row['finalSourceCalls'] == 0
        and row['sourceUnchanged'], 'master_invalid_request_side_effect')
    check(row['statuses'] == [404] * 4, 'master_invalid_generation_admitted')
    require_diagnostics(row['diagnostics'], 4)
    row['result'] = 'observed'
