"""Use measured full CLI sample clocks; retain raw ffprobe clocks independently."""
from fractions import Fraction
import hashlib
import subprocess
from hls_remaining_nonkey_aac_clock import frame_md5, filter_clock
from hls_remaining_nonkey_evidence import native_pcm


def require(value, name):
    if not value:
        raise RuntimeError(name)


def measure_cli_clock(source, result):
    facts, pcm = native_pcm(source)
    result.update(sourcePCM=facts, result='in-flight')
    require(facts['completeEOFAccounted'], 'aac_normalized_source_eof')
    args = ['ffmpeg', '-nostdin', '-v', 'info', '-xerror', '-threads', '2', '-i', str(source),
        '-map', '0:a:0', '-vn', '-sn', '-dn', '-frames:a', '4097', '-af', 'ashowinfo',
        '-c:a', 'pcm_s16le', '-f', 'framemd5', 'pipe:1']
    process = subprocess.run(args, capture_output=True, timeout=45)
    require(process.returncode == 0 and len(process.stdout) <= 2 << 20 and
        len(process.stderr) <= 2 << 20, 'aac_normalized_cli_bound')
    result.update(frameMD5OutputSHA256=hashlib.sha256(process.stdout).hexdigest(),
        generatedMediaLogSHA256=hashlib.sha256(process.stderr).hexdigest())
    result['outputPCMClock'] = frame_md5(process.stdout.decode(), pcm)
    result['preTrimUserFilterClock'] = filter_clock(process.stderr.decode())
    rows = result['preTrimUserFilterClock']['completeRows']
    require(len(rows) == len(facts['decodedFrameRows']) and
        [r['samples'] for r in rows] == [r['nb_samples'] for r in facts['decodedFrameRows']] and
        sum(r['samples'] for r in rows) == facts['samples'] and
        all(abs(Fraction(r['ptsTime']) - Fraction(r['pts'], 48000)) <= Fraction(1, 100000)
            for r in rows), 'aac_normalized_complete_frame_association')
    result.update(result='observed', fullSourceClockQualified=True, usedReferencePCMToChooseClock=False)


def derive_normalized_audio_edit(clock, measurement, first_pts, offset, current):
    require(measurement.get('fullSourceClockQualified') is True and
        measurement.get('result') == 'observed', 'aac_normalized_measurement_unqualified')
    rows = clock['completeRows']
    filtered = measurement['preTrimUserFilterClock']['completeRows']
    require(0 < len(rows) == len(filtered) <= 4096 and
        [r['samples'] for r in rows] == [r['samples'] for r in filtered],
        'aac_normalized_native_frame_association')
    requested = Fraction(str(offset)) * 48000
    require(requested.denominator == 1 and requested >= 0 and type(current) is int,
            'aac_normalized_requested_clock')
    first = [n for n, row in enumerate(rows) if row['pts'] == first_pts]
    target = [n for n, row in enumerate(filtered) if row['pts'] <= requested < row['pts'] + row['samples']]
    require(len(first) == len(target) == 1, 'aac_normalized_unique_frame_identity')
    selected = target[0]
    wanted = rows[selected]['nativeStartSample'] + int(requested) - filtered[selected]['pts']
    media_time = wanted - rows[first[0]]['nativeStartSample']
    delta = media_time - current
    require(media_time >= 32 and -32 <= delta <= 32, 'aac_normalized_edit_bound')
    return {'requestedTimestampSamples': int(requested), 'desiredNativeStartSample': wanted,
        'firstCopiedNativeFrame': rows[first[0]], 'targetNativeFrame': rows[selected],
        'targetNormalizedFilterFrame': filtered[selected], 'originalMediaTime': current,
        'derivedMediaTime': media_time, 'deltaSamples': delta,
        'sourcePCM_SHA256': measurement['sourcePCM']['sha256'],
        'filterMeasurementSHA256': measurement['generatedMediaLogSHA256'],
        'usedReferencePCMToChooseEdit': False, 'productionAcceptance': False,
        'boundary': 'Measured complete fixed-fixture CLI clock; no generic source-origin/codec/renderer certificate'}
