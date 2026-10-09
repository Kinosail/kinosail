"""Strict fixed-source public AAC observations; every delivered packet remains."""
import hashlib
import json
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
        'timelineKeys': {'count': len(timeline['Keys']), 'first': timeline['Keys'][0],
                         'last': timeline['Keys'][-1]}}


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
