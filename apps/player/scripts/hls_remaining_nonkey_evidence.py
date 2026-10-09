"""Complete non-key observations; never trims frames, packets or public PCM."""
import hashlib
import json
import math
import re
import subprocess
from hls_followon_frames import decode_frames
from hls_followon_public import bounded_bytes, check
from hls_remaining_nonkey_init import initialization_metadata
from hls_remaining_nonkey_fragment import fragment_metadata


def packet_rows(path):
    command = ['ffprobe', '-v', 'error', '-read_intervals', '%+#4097',
        '-show_packets', '-show_data_hash', 'sha256', '-show_entries',
        'packet=stream_index,pts,pts_time,dts,dts_time,duration,duration_time,size,flags,data_hash,side_data_list',
        '-of', 'json', str(path)]
    process = subprocess.run(command, capture_output=True, timeout=30)
    check(process.returncode == 0 and len(process.stdout) <= 4 << 20, 'nonkey_packet_probe')
    rows = json.loads(process.stdout).get('packets', [])
    check(0 < len(rows) <= 4096, 'nonkey_complete_packet_bound')
    missing = []
    for number, row in enumerate(rows):
        check(re.fullmatch(r'SHA256:[a-f0-9]{64}', row.get('data_hash', '')) is not None,
              'nonkey_packet_hash')
        for field in ['pts_time', 'dts_time', 'duration_time']:
            if field not in row:
                missing.append({'packet': number, 'field': field})
            else:
                check(math.isfinite(float(row[field])), 'nonkey_packet_clock')
    return rows, missing


def audio_stream(path):
    process = subprocess.run(['ffprobe', '-v', 'error', '-select_streams', 'a:0',
        '-show_entries', 'stream=sample_rate,channels,codec_name,time_base,start_time,duration',
        '-of', 'json', str(path)], capture_output=True, timeout=30)
    check(process.returncode == 0 and len(process.stdout) <= 65536, 'nonkey_audio_stream_bound')
    streams = json.loads(process.stdout).get('streams', [])
    check(len(streams) == 1 and streams[0]['sample_rate'] == '48000'
          and streams[0]['channels'] == 2, 'nonkey_native_audio_format')
    return streams[0]


def native_pcm(path, offset=None):
    stream = audio_stream(path)
    command = ['ffmpeg', '-nostdin', '-v', 'error', '-xerror', '-threads', '2', '-i', str(path)]
    if offset is not None:
        command += ['-ss', str(offset)]
    command += ['-map', '0:a:0', '-vn', '-sn', '-dn', '-frames:a', '4097',
                '-c:a', 'pcm_s16le', '-f', 's16le', 'pipe:1']
    process = subprocess.run(command, capture_output=True, timeout=45)
    data = process.stdout
    check(process.returncode == 0 and 0 < len(data) <= 8 << 20 and len(data) % 4 == 0,
          'nonkey_native_pcm_bound')
    probe = subprocess.run(['ffprobe', '-v', 'error', '-select_streams', 'a:0',
        '-show_frames', '-show_entries',
        'frame=pts_time,best_effort_timestamp_time,pkt_dts_time,nb_samples,side_data_list',
        '-of', 'json', str(path)], capture_output=True, timeout=30)
    check(probe.returncode == 0 and len(probe.stdout) <= 2 << 20, 'nonkey_audio_frame_bound')
    frames = json.loads(probe.stdout).get('frames', [])
    check(0 < len(frames) <= 4096 and all(0 < f.get('nb_samples', 0) <= 8192 for f in frames),
          'nonkey_audio_frame_shape')
    return {'sampleRate': 48000, 'channels': 2, 'format': 's16le',
        'samples': len(data) // 4, 'sha256': hashlib.sha256(data).hexdigest(),
        'decodedFrameRows': frames, 'decodedFrameSampleSum': sum(f['nb_samples'] for f in frames),
        'rateConversionApplied': False, 'channelConversionApplied': False,
        'timeBudgetApplied': False, 'outputReferenceSeekSeconds': offset,
        'completeEOFAccounted': offset is None and len(data) // 4 == sum(f['nb_samples'] for f in frames),
        'decodedFrameRowsScope': 'Complete input decode before optional reference output seek',
        'stream': stream}, data


def pcm_tail_correspondence(source, public, requested):
    if len(source) % 4 or len(public) % 4 or not public or type(requested) is not int:
        raise RuntimeError('aac_pcm_correspondence_shape')
    count = len(public) // 4
    matches = [n for n in range(max(0, requested - 64), min(len(source) // 4, requested + 64) + 1)
        if source[n*4:n*4 + len(public)] == public]
    start = matches[0] if len(matches) == 1 else None
    return {'boundary': 'Whole public PCM diagnostic; no public sample trimmed, padded or rewritten',
        'searchRadiusSamples': 64, 'publicSamples': count, 'sequenceMatches': len(matches),
        'uniqueStartSample': start, 'requestedStartDeltaSamples': start - requested if start is not None else None,
        'uniqueSourceTailComplete': start is not None and start + count == len(source) // 4,
        'publicPCM_SHA256': hashlib.sha256(public).hexdigest()}



def frame_mapping(source_rows, public_rows, requested):
    index = {digest: number for number, (_, digest) in enumerate(source_rows)}
    check(len(index) == len(source_rows) == 768, 'nonkey_unique_source_frames')
    expected = [number for number, (pts, _) in enumerate(source_rows) if pts >= requested - 0.000001]
    mapped = [index.get(digest) for _, digest in public_rows]
    check(all(number is not None for number in mapped), 'nonkey_unknown_delivered_frame')
    return {'sourceColumns': ['sourcePTS', 'md5'], 'sourceRows': source_rows,
        'publicColumns': ['sourceIndex', 'sourcePTS', 'publicPTS', 'md5'],
        'publicRows': [[number, source_rows[number][0], pts, digest]
                       for number, (pts, digest) in zip(mapped, public_rows)],
        'expectedSourceIndices': expected, 'actualSourceIndices': mapped,
        'exactRequestedSequence': mapped == expected,
        'precedingSourceFrames': [number for number in mapped if number < expected[0]],
        'allRawFramesRetained': True, 'negativeFramesDiscarded': 0}


def observed_media(source, public, init, fragments, metadata, offset, result):
    result.update(boundary='All default-decoder facts retained by stage; edits never applied by oracle.',
                  completedStages=[])
    def stage(name):
        result['currentStage'] = name
    def completed(name):
        result['completedStages'].append(name)
    stage('source-decode')
    source_facts, source_rows = decode_frames(source)
    result.update(sourceDecode=source_facts, sourceFrameRows=source_rows)
    points = metadata['sourceFramePTS']
    check(len(points) == len(source_rows) and all(abs(row[0] - point) <= 0.0000011
          for row, point in zip(source_rows, points)), 'nonkey_independent_source_pts')
    result['independentSourcePTSBound'] = True
    completed('source-decode')
    stage('public-decode')
    public_facts, public_rows = decode_frames(public)
    result.update(publicDecode=public_facts, publicFrameRows=public_rows)
    completed('public-decode')
    stage('reference-decode')
    reference_facts, reference_rows = decode_frames(source, offset)
    result.update(referenceDecode=reference_facts, referenceFrameRows=reference_rows)
    mapping = frame_mapping(source_rows, public_rows, metadata['sourceTimeOriginSeconds'] + offset)
    result['mapping'] = mapping
    expected_hashes = [source_rows[n][1] for n in mapping['expectedSourceIndices']]
    check([digest for _, digest in reference_rows] == expected_hashes,
          'nonkey_independent_output_seek_reference')
    completed('reference-decode')
    stage('initialization')
    result['initializationSHA256'] = hashlib.sha256(init).hexdigest()
    result['initialization'] = initialization_metadata(init)
    completed('initialization')
    stage('physical-fragments')
    result['physicalFragments'] = []
    check(0 < len(fragments) <= 32, 'nonkey_physical_fragment_count')
    for path in fragments:
        data = bounded_bytes(path, 2 << 20, 'nonkey_fragment_bound')
        row = {'name': path.name, 'sha256': hashlib.sha256(data).hexdigest()}
        result['physicalFragments'].append(row)
        row['tracks'] = fragment_metadata(data)
    completed('physical-fragments')
    stage('source-packets')
    source_packets, source_missing = packet_rows(source)
    result['sourcePacketRows'] = source_packets
    result['packetClockQualification'] = {'sourceMissingFields': source_missing,
        'publicMissingFields': None, 'qualified': False, 'missingValuesInferred': False}
    completed('source-packets')
    stage('public-packets')
    public_packets, public_missing = packet_rows(public)
    result['publicPacketRows'] = public_packets
    result['packetClockQualification'].update(publicMissingFields=public_missing,
                                               qualified=not source_missing and not public_missing)
    public_video = [row for row in public_packets if row['stream_index'] == 0]
    result['firstPublicVideoPacketKeyFlag'] = bool(public_video and 'K' in public_video[0]['flags'])
    result['IDRNALQualificationPerformed'] = False
    check(result['firstPublicVideoPacketKeyFlag'], 'nonkey_first_packet_key')
    completed('public-packets')
    result['nativePCM'] = {}
    stage('source-native-pcm')
    source_pcm, source_bytes = native_pcm(source)
    result['nativePCM']['source'] = source_pcm
    completed('source-native-pcm')
    stage('public-native-pcm')
    public_pcm, public_bytes = native_pcm(public)
    result['nativePCM']['public'] = public_pcm
    completed('public-native-pcm')
    stage('reference-native-pcm')
    reference_pcm, _ = native_pcm(source, offset)
    result['nativePCM'].update(referenceSeek=reference_pcm,
        wholePublicCorrespondence=pcm_tail_correspondence(source_bytes, public_bytes, round(offset * 48000)),
        wholePublicEqualsReference=public_pcm['sha256'] == reference_pcm['sha256'],
        publicMinusReferenceSamples=public_pcm['samples'] - reference_pcm['samples'],
        publicAndSourceCompleteEOFAccounted=source_pcm['completeEOFAccounted'] and public_pcm['completeEOFAccounted'])
    completed('reference-native-pcm')
    stage('complete')
    return result
