"""Existing pinned-codec readers; raw producer evidence is never trimmed."""
from fractions import Fraction
import hashlib
import json
import re
from hls_followon_frames import parse_frames, frame_facts, stream_metadata
from hls_followon_public import bounded_bytes, check


def hex_dump(text):
    chunks = []
    position = 0
    for line in text.splitlines():
        if not line.strip():
            continue
        check(re.match(r'^[0-9a-f]{8}: ', line) is not None, 'capability_packet_hex_shape')
        value = line.split(': ', 1)[1].split('  ', 1)[0].replace(' ', '')
        check(re.fullmatch(r'(?:[0-9a-f]{2})+', value) is not None, 'capability_packet_hex_bytes')
        data = bytes.fromhex(value)
        check(int(line[:8], 16) == position and 0 < len(data) <= 16, 'capability_packet_hex_extent')
        chunks.append(data)
        position += len(data)
    return b''.join(chunks)


def nal_evidence(run, path, first_only=False):
    command = ['ffprobe', '-v', 'error', '-select_streams', 'v:0',
        '-read_intervals', '%+#4097', '-show_packets', '-show_streams',
        '-show_data', '-show_data_hash', 'sha256', '-show_entries',
        'stream=index,codec_type,codec_name,extradata,time_base:packet=pts,pts_time,dts,flags,size,data,data_hash',
        '-of', 'json', str(path)]
    result = json.loads(run(command, 30, 8 << 20))
    streams, packets = result.get('streams', []), result.get('packets', [])
    check(len(streams) == 1 and 0 < len(packets) <= 4096, 'capability_nal_probe_shape')
    check(streams[0].get('index') == 0 and streams[0].get('codec_type') == 'video'
          and streams[0].get('codec_name') == 'h264', 'capability_nal_h264_identity')
    config = hex_dump(streams[0]['extradata'])
    check(len(config) >= 5 and config[0] == 1, 'capability_avcc_configuration')
    width = (config[4] & 3) + 1
    rows = []
    for number, packet in enumerate(packets[:1] if first_only else packets):
        data, position, types = hex_dump(packet['data']), 0, []
        check(len(data) == int(packet['size']), 'capability_packet_payload_size')
        check('SHA256:' + hashlib.sha256(data).hexdigest() == packet['data_hash'], 'capability_packet_payload_hash')
        while position < len(data):
            check(position + width <= len(data), 'capability_nal_length_bound')
            size = int.from_bytes(data[position:position + width], 'big')
            position += width
            check(size > 0 and position + size <= len(data), 'capability_nal_payload_bound')
            types.append(data[position] & 31)
            position += size
        row = {key: packet[key] for key in ['pts', 'pts_time', 'dts', 'flags', 'size', 'data_hash'] if key in packet}
        row.update(packetNumber=number, payloadSHA256=hashlib.sha256(data).hexdigest(),
                   nalTypes=types, containsIDR=5 in types)
        rows.append(row)
    return {'command': command, 'timeBase': streams[0]['time_base'],
            'nalLengthBytes': width, 'completeVideoPacketCount': len(packets),
            'accessUnitRowsInspected': len(rows),
            'rows': rows, 'allEncodedPayloadsInspected': not first_only}


def expected_sequence(source_rows, requested):
    return [digest for point, digest in source_rows if Fraction(str(point)) >= requested - Fraction(1, 1000000)]


def reader_case(run, path, label, input_options, source_rows, requested, reference_pcm, row, copyts=True):
    row.update(label=label, result='in-flight', inputOptions=input_options,
               copytsApplied=copyts, rawProducerAcceptance=False)
    command = ['ffmpeg', '-nostdin', '-v', 'error', '-xerror', '-threads', '2',
               *(['-copyts'] if copyts else []), *input_options, '-i', str(path)]
    video = command + ['-an', '-frames:v', '4097', '-fps_mode', 'passthrough',
                       '-enc_time_base', '1:1000000', '-f', 'framemd5', 'pipe:1']
    row['videoCommand'] = video
    try:
        data = run(video, 30, 2 << 20)
        row['videoOutputBytes'] = len(data)
        row['videoOutputSHA256'] = hashlib.sha256(data).hexdigest()
        # Retain every actual PTS/hash row before any acceptance comparison.
        frames = parse_frames(data)
        row['completeFrameRows'] = frames
        row['frameRowScope'] = 'All decoded framemd5 PTS/hash rows; not frame side-data'
        row['frameTimeBase'] = re.search(rb'^#tb 0: ([0-9]+/[0-9]+)$', data, re.M)[1].decode()
        row['video'] = frame_facts(frames)
        row['seekedConsumerExact'] = [digest for _, digest in frames] == expected_sequence(source_rows, requested)
        row['videoEOFAccounted'] = len(frames) < 4097
        audio = command + ['-map', '0:a:0', '-vn', '-sn', '-dn', '-frames:a', '4097',
                           '-c:a', 'pcm_s16le', '-f', 's16le', 'pipe:1']
        row['audioCommand'] = audio
        pcm = run(audio, 30, 8 << 20)
        check(0 < len(pcm) <= 8 << 20 and len(pcm) % 4 == 0, 'capability_reader_pcm_shape')
        row['audio'] = {'samples': len(pcm) // 4, 'sha256': hashlib.sha256(pcm).hexdigest(),
            'sampleRate': 48000, 'channels': 2, 'format': 's16le',
            'completeEqualsIndependentReference': pcm == reference_pcm,
            'scope': 'Complete command output; input EOF bound independently established by baseline frame count',
            'rateConversionApplied': False, 'channelConversionApplied': False}
        row['result'] = 'observed'
    except Exception as error:
        row.update(result='command-or-observation-failed',
                   failureClass=str(error) if isinstance(error, RuntimeError) else type(error).__name__)
        if isinstance(error, RuntimeError) and str(error) == 'bounded_diagnostic_deadline':
            raise


def read_options(public_rows, source_rows, requested, metadata):
    index = {digest: n for n, (_, digest) in enumerate(source_rows)}
    check(len(index) == len(source_rows) == 768, 'capability_unique_source_association')
    check(public_rows and all(digest in index for _, digest in public_rows), 'capability_public_association')
    matched = source_rows[index[public_rows[0][1]]][0]
    first = Fraction(str(public_rows[0][0]))
    origin = Fraction(str(metadata['format']['start_time']))
    target = first + requested - Fraction(str(matched))
    return {'firstPublicVideoPTS': str(first), 'matchedFirstSourceVideoPTS': str(matched),
        'requestedSourcePTS': str(requested), 'containerStartTime': str(origin),
        'absoluteLocalTarget': str(target), 'relativeLocalTarget': str(target - origin),
        'formula': 'firstPublicVideoPTS + requestedSourcePTS - matchedFirstSourceVideoPTS'}, target, target - origin


def consumers(run, directory, source_rows, requested, public_rows, reference_pcm, result):
    path = directory / 'joined.mp4'
    metadata = stream_metadata(path)
    clock, absolute, relative = read_options(public_rows, source_rows, requested, metadata)
    result.update(readerClock=clock, containerPresentation=metadata, readers=[])
    options = [('advanced-edit-enabled', ['-advanced_editlist', '1']),
        ('advanced-edit-disabled', ['-advanced_editlist', '0']),
        ('ignore-edit-list', ['-ignore_editlist', '1']),
        ('accurate-absolute', ['-seek_timestamp', '1', '-ss', str(float(absolute)), '-accurate_seek']),
        ('accurate-relative', ['-ss', str(float(relative)), '-accurate_seek']),
        ('inaccurate-absolute-control', ['-seek_timestamp', '1', '-ss', str(float(absolute)), '-noaccurate_seek']),
        ('accurate-absolute-zero', ['-seek_timestamp', '1', '-ss', '0', '-accurate_seek']),
        ('accurate-relative-zero', ['-ss', '0', '-accurate_seek']),
        ('accurate-absolute-normal-clock', ['-seek_timestamp', '1', '-ss', str(float(absolute)), '-accurate_seek']),
        ('accurate-absolute-zero-normal-clock', ['-seek_timestamp', '1', '-ss', '0', '-accurate_seek'])]
    for label, argv in options:
        row = {}
        result['readers'].append(row)
        reader_case(run, path, label, argv, source_rows, requested, reference_pcm, row,
                    copyts=not label.endswith('-normal-clock'))
    media = [directory / 'init.mp4', *sorted(directory.glob('segment-*.m4s'))]
    before = {p.name: hashlib.sha256(bounded_bytes(p, 2 << 20, 'capability_reader_asset_bound')).hexdigest() for p in media}
    result['readerMediaSHA256Before'] = before
    manifest = (directory / 'index.m3u8').read_text()
    check('#EXT-X-ENDLIST' in manifest and '#EXT-X-START' not in manifest, 'capability_finalized_hls_control')
    source_origin_candidate = requested - Fraction(clock['matchedFirstSourceVideoPTS'])
    result['playlistOffsetQualification'] = 'Unqualified origin candidates; no certified HLS playlist/source origin'
    for candidate, offset in [('container-relative', relative), ('first-video-source', source_origin_candidate)]:
        for precise in ['NO', 'YES']:
            label = 'finalized-hls-start-' + candidate + '-' + precise.lower()
            playlist = directory / (label + '.m3u8')
            content = manifest.replace('#EXTM3U\n', '#EXTM3U\n#EXT-X-START:TIME-OFFSET='
                + str(float(offset)) + ',PRECISE=' + precise + '\n', 1)
            playlist.write_text(content)
            row = {'playlistSHA256': hashlib.sha256(content.encode()).hexdigest(),
                   'playlistComplete': True, 'playlistOffsetCandidate': candidate,
                   'playlistOffsetSeconds': str(offset), 'playlistOriginCertified': False}
            result['readers'].append(row)
            reader_case(run, playlist, label, ['-allowed_extensions', 'mp4,m4s', '-prefer_x_start', '1'],
                        source_rows, requested, reference_pcm, row)
    result['readerMediaSHA256After'] = {p.name: hashlib.sha256(bounded_bytes(
        p, 2 << 20, 'capability_reader_asset_bound')).hexdigest() for p in media}
    result['readerMediaUnchanged'] = result['readerMediaSHA256After'] == before
    check(result['readerMediaUnchanged'], 'capability_reader_media_changed')
