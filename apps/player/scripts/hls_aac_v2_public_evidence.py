"""Strict fixed-source public AAC observations; every delivered packet remains."""
import hashlib
import json
from decimal import Decimal
from hls_followon_public import bounded_bytes, check
from hls_remaining_nonkey_boundary import packet_tail
from hls_remaining_nonkey_evidence import observed_media
from hls_nonkey_browser_audio_boundary import retained_audio_boundary
from hls_nonkey_browser_packet_association import packet_association
from hls_nonkey_browser_packet_clock import packet_clock


def cache_state(cache):
    paths = list(cache.glob('*/.copy-timeline'))
    check(len(paths) == 1, 'v2_single_indexed_generation')
    directory = paths[0].parent
    timeline = json.loads(bounded_bytes(paths[0], 256 << 10, 'v2_timeline_bound'))
    certificate = json.loads(bounded_bytes(directory / '.copy-clock', 4096, 'v2_certificate_bound'))
    check(certificate['version'] == 2 and timeline.get('Clock') == 0
          and timeline.get('AudioOrigin') is not None, 'v2_bound_origin_required')
    rendition = certificate['rendition']
    check(rendition in ['360p', '480p', '720p', '1080p'], 'v2_rendition_allowlist')
    media = directory / rendition
    names = sorted(path.name for path in media.glob('segment-*.m4s'))
    bound = {name: hashlib.sha256(bounded_bytes(directory / name, 256 << 10, 'v2_metadata_bound')).hexdigest()
             for name in ['.source', '.copy-timeline', '.copy-clock']}
    assets = {name: hashlib.sha256(bounded_bytes(media / name, 2 << 20, 'v2_asset_bound')).hexdigest()
              for name in ['init.mp4', *names]}
    generation = directory.stat()
    return directory, media, {'metadataSHA256': bound, 'assetSHA256': assets,
        'generationDevice': generation.st_dev, 'generationInode': generation.st_ino,
        'segments': names, 'audioOrigin': timeline['AudioOrigin'],
        'certificateVersion': certificate['version'], 'clock': timeline['Clock'],
        'timelineEnd': timeline['End'], 'timelineGrid': [timeline['Numerator'], timeline['Denominator']],
        'timelineKeys': timeline['Keys']}


def assert_fixed_packets(observed):
    source = observed['sourcePacketRows']
    public = observed['publicPacketRows']
    left = [row for row in source if row['stream_index'] == 1]
    right = [row for row in public if row['stream_index'] == 1]
    check(len(left) == 1501 and len(right) == 942, 'v2_complete_aac_count')
    tail = packet_tail(source, public)
    check(tail['wholePublicPacketTail'] and tail['uniqueSourceStart'] == 559,
          'v2_complete_aac_payload_suffix')
    expected = left[559:]
    check([row['data_hash'] for row in right] == [row['data_hash'] for row in expected],
          'v2_every_aac_payload')
    for actual, reference in zip(right, expected):
        check(actual['duration'] == reference['duration'], 'v2_every_aac_duration')
        check(actual['pts'] - reference['pts'] == -576000
              and actual['dts'] - reference['dts'] == -576000,
              'v2_every_aac_clock')
    for previous, current in zip(right, right[1:]):
        check(current['pts'] == previous['pts'] + previous['duration']
              and current['dts'] == previous['dts'] + previous['duration'],
              'v2_adjacent_aac_gap_or_overlap')
    video = [row for row in source if row['stream_index'] == 0]
    output = [row for row in public if row['stream_index'] == 0]
    check(len(video) == 768 and len(output) == 480
          and [row['data_hash'] for row in output] == [row['data_hash'] for row in video[288:]],
          'v2_every_video_payload')
    check(all(a['dts'] < b['dts'] for a, b in zip(output, output[1:])),
          'v2_video_decode_order')
    boundary = retained_audio_boundary(observed)
    association = packet_association(tail)
    clock = packet_clock(tail, boundary)
    check(clock['observed'] and clock['clockOffsetRangeTicks'] == [-576000, -576000]
          and clock['clockChangeCount'] == 0 and clock['adjacentPTSChangeCount'] == 0
          and association['unknownPackets'] == 0, 'v2_complete_packet_clock_projection')
    return {'tail': tail, 'association': association, 'clock': clock, 'boundary': boundary}


def qualify(source, joined, assets, metadata, result):
    observed = result.setdefault('observations', {})
    observed_media(source, joined, assets[0].read_bytes(), assets[1:], metadata, 12, observed)
    result.update(assert_fixed_packets(observed))
    check(observed['mapping']['exactRequestedSequence']
          and observed['mapping']['actualSourceIndices'] == list(range(288, 768)),
          'v2_every_decoded_video_frame')
    pcm = observed['nativePCM']
    check(pcm['public']['samples'] == pcm['referenceSeek']['samples'] == 960008
          and pcm['wholePublicEqualsReference'] and pcm['publicAndSourceCompleteEOFAccounted'],
          'v2_full_native_pcm_and_eof')
    result['fixedPublicMediaAccepted'] = True


def assert_fixed_source_grid(data):
    streams, rows = data['streams'], data['packets']
    check(len(streams) == 1 and streams[0]['time_base'] == '1/16000' and len(rows) == 768,
          'v2_complete_source_video_grid')
    projected = []
    for row in rows:
        check(all(type(row[name]) is int for name in ['pts', 'dts', 'duration'])
              and 0 <= row['pts'] <= 512000 and -16000 <= row['dts'] <= 512000
              and 0 < row['duration'] <= 16000
              and type(row['flags']) is str and 0 < len(row['flags']) <= 16,
              'v2_source_video_packet_shape')
        projected.append({name: row[name] for name in ['pts', 'dts', 'duration', 'flags']})
    check(all(a['dts'] < b['dts'] for a, b in zip(projected, projected[1:])),
          'v2_source_video_decode_order')
    keys = [{'PTS': row['pts'], 'DTS': row['dts']} for row in projected if 'K' in row['flags']]
    end = max(row['pts'] + row['duration'] for row in projected)
    check([key['PTS'] for key in keys] == list(range(0, 512000, 32000)) and end == 511984,
          'v2_pinned_source_key_end_witness')
    return {'timeBase': [1, 16000], 'endTicks': end, 'keys': keys,
            'packetSHA256': hashlib.sha256(json.dumps(projected, sort_keys=True,
                separators=(',', ':'), allow_nan=False).encode()).hexdigest()}


def assert_fixed_cache_grid(cache, grid):
    check(cache['timelineGrid'] == grid['timeBase'] == [1, 16000]
          and cache['timelineKeys'] == grid['keys'][6:]
          and abs(cache['timelineEnd'] - grid['endTicks'] / 16000) <= 1e-12,
          'v2_cached_timeline_not_source_bound')


def assert_fixed_timeline(facts, fragments, grid):
    keys = grid['keys'][6:]
    check(grid['timeBase'] == [1, 16000] and grid['endTicks'] == 511984
          and len(keys) == 10 and keys[0]['PTS'] == 192000,
          'v2_pinned_resume_timeline')
    durations = [b['PTS'] - a['PTS'] for a, b in zip(keys, keys[1:])]
    durations.append(grid['endTicks'] - keys[-1]['PTS'])
    check(durations == [32000] * 9 + [31984]
          and facts['endlist'] and facts['playlistType'] == 'VOD'
          and len(fragments) == 10, 'v2_complete_public_timeline')
    check([name for name, _ in fragments] ==
          ['segment-' + format(n, '05d') + '.m4s' for n in range(10)],
          'v2_every_public_segment_name')
    for (_, duration), expected in zip(fragments, durations):
        check(Decimal(str(duration)) * 16000 == expected, 'v2_every_public_segment_duration')
    check(Decimal(str(facts['durationSeconds'])) * 16000 == grid['endTicks'] - keys[0]['PTS'],
          'v2_complete_public_source_span')
