"""Complete CLI filter/output audio clocks; no edit derivation or production claim."""
from fractions import Fraction
import hashlib
import re


def require(value, name):
    if not value:
        raise RuntimeError(name)


def frame_md5(document, pcm):
    require(len(document.encode()) <= 2 << 20 and len(pcm) % 4 == 0, 'aac_clock_output_bound')
    headers = [line.strip() for line in document.splitlines() if line.startswith('#')]
    require('#tb 0: 1/48000' in headers and '#media_type 0: audio' in headers and
            '#sample_rate 0: 48000' in headers, 'aac_clock_output_format')
    rows, ordinal = [], 0
    for line in document.splitlines():
        if not line.strip() or line.startswith('#'):
            continue
        fields = [v.strip() for v in line.split(',')]
        require(len(fields) == 6 and all(re.fullmatch(r'-?[0-9]{1,17}', v)
                for v in fields[:5]) and re.fullmatch(r'[a-f0-9]{32}', fields[5]), 'aac_clock_packet_shape')
        stream, dts, pts, samples, size = [int(v) for v in fields[:5]]
        require(stream == 0 and dts == pts and 0 < samples <= 8192 and size == samples * 4,
                'aac_clock_packet_extent')
        data = pcm[ordinal * 4:(ordinal + samples) * 4]
        require(len(data) == size and hashlib.md5(data, usedforsecurity=False).hexdigest() == fields[5],
                'aac_clock_packet_pcm_identity')
        rows.append({'number': len(rows), 'pts': pts, 'dts': dts, 'samples': samples,
            'bytes': size, 'md5': fields[5], 'nativeStartSample': ordinal,
            'clockMinusOrdinalSamples': pts - ordinal})
        ordinal += samples
        require(len(rows) <= 4096, 'aac_clock_packet_row_bound')
    require(rows and ordinal * 4 == len(pcm), 'aac_clock_complete_pcm_extent')
    return {'timeBase': '1/48000', 'sampleRate': 48000, 'channels': 2,
        'completeRows': rows, 'samples': ordinal, 'pcmSHA256': hashlib.sha256(pcm).hexdigest(),
        'allOutputPacketPCMBytesBound': True,
        'clockGapsSamples': [b['pts'] - a['pts'] - a['samples'] for a, b in zip(rows, rows[1:])],
        'residualSamples': sorted({r['clockMinusOrdinalSamples'] for r in rows})}


def filter_clock(stderr):
    require(len(stderr.encode()) <= 2 << 20, 'aac_clock_filter_log_bound')
    rows = []
    for line in stderr.splitlines():
        if 'Parsed_ashowinfo_' not in line or ' n:' not in line:
            continue
        match = re.search(r' n:([0-9]+) pts:(-?[0-9]+) pts_time:([^ ]+)', line)
        fields = {key: re.search(r'\b' + key + r':([^ ]+)', line)
                  for key in ['fmt', 'channels', 'rate', 'nb_samples', 'checksum']}
        require(match and all(fields.values()), 'aac_clock_filter_row_shape')
        number, pts = int(match[1]), int(match[2])
        value = {key: found[1] for key, found in fields.items()}
        require(number == len(rows) and int(value['channels']) == 2 and
            int(value['rate']) == 48000 and 0 < int(value['nb_samples']) <= 8192 and
            re.fullmatch(r'[A-Fa-f0-9]{8}', value['checksum']), 'aac_clock_filter_extent')
        Fraction(match[3])
        rows.append({'number': number, 'pts': pts, 'ptsTime': match[3],
            'format': value['fmt'], 'channels': 2, 'sampleRate': 48000,
            'samples': int(value['nb_samples']), 'checksum': value['checksum']})
        require(len(rows) <= 4096, 'aac_clock_filter_row_bound')
    require(rows, 'aac_clock_filter_empty')
    return {'completeRows': rows, 'samples': sum(r['samples'] for r in rows),
        'timeBaseInferred': False, 'framingIndependentOfPCMEncoder': True}
