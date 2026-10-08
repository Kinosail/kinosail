#!/usr/bin/env python3
"""Strict public non-key regression and complete raw timestamp observations."""
import hashlib
import json
import math
import os
from pathlib import Path
import re
import shutil
import socket
import subprocess
import sys
import threading
import time
from hls_timeline_fixture import fixture
from hls_timeline_http import PublicServer, sha, source_state
from hls_timeline_packets import safe_encoder_lifecycle, safe_seek_phases
from hls_timeline_preparation import prepare_scene
from hls_followon_public import check, bounded_bytes, encoder_count, sample_resources, prepare_once, measure
from hls_followon_frames import stream_metadata
from hls_remaining_nonkey_evidence import observed_media
from hls_remaining_process import finish_processes
from hls_remaining_nonkey_deadline import DiagnosticDeadline

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-nonkey' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
binary = RUN / 'player'
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], text=True).strip(),
    'trackedSourceClean': not subprocess.check_output(
        ['git', 'status', '--porcelain', '--untracked-files=no'], text=True).strip(),
    'result': 'failed', 'cases': [], 'productionAcceptance': False,
    'boundary': 'Synthetic authenticated public diagnostic; all raw data retained, no discard map applied.',
    'originalFailuresPreserved': True, 'nativeSafariNoxAcceptance': False}

guard = DiagnosticDeadline(650)
guard.__enter__()


class CaptureServer(PublicServer):
    def __init__(self, url, directory):
        super().__init__(url)
        self.directory, self.initialization = directory, None

    def http(self, path, *args, **kwargs):
        guard.check(40)
        status, data, headers = super().http(path, *args, **kwargs)
        name = path.rsplit('/', 1)[-1]
        if status == 200 and name == 'init.mp4':
            if self.initialization is not None:
                check(data == self.initialization, 'captured_initialization_changed')
            self.initialization = data
        if status == 200 and re.fullmatch(r'segment-[0-9]{5}\.m4s', name):
            target = self.directory / name
            if target.exists():
                check(bounded_bytes(target, 2 << 20, 'captured_fragment_bound') == data,
                      'captured_physical_fragment_changed')
            check(0 < len(data) <= 2 << 20, 'captured_fragment_bound')
            target.write_bytes(data)
        return status, data, headers


def convert(original, metadata):
    path = RUN / 'regular-copy.mp4'
    subprocess.run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(original),
        '-map', '0:v:0', '-map', '0:a:0', '-c', 'copy', str(path)], check=True, timeout=45,
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    facts = stream_metadata(path)
    return path, reprobe_source(path, dict(metadata, sha256=sha(path),
        durationSeconds=float(facts['format']['duration'])))


def reprobe_source(path, metadata):
    facts = stream_metadata(path)
    data = subprocess.check_output(['ffprobe', '-v', 'error', '-select_streams', 'v:0', '-show_frames',
        '-show_entries', 'frame=best_effort_timestamp_time,key_frame', '-of', 'json', str(path)], timeout=30)
    check(len(data) <= 256 * 1024, 'source_frame_clock_bound')
    frames = json.loads(data)['frames']
    times = [float(v['best_effort_timestamp_time']) for v in frames]
    keys = [float(v['best_effort_timestamp_time']) for v in frames if v['key_frame']]
    origin = float(facts['format']['start_time'])
    check(len(times) == 768 and math.isfinite(origin) and all(math.isfinite(v) for v in times)
          and all(a < b for a, b in zip(times, times[1:])), 'source_presentation_clock')
    video = next(v for v in facts['streams'] if v['codec_type'] == 'video')
    numerator, denominator = video['avg_frame_rate'].split('/')
    rate = float(numerator) / float(denominator)
    check(0 < len(keys) <= 100 and abs(rate - 24) < 0.001, 'source_key_rate')
    return dict(metadata, keyframesSeconds=keys, frameRate=rate, streamOrigins=facts,
                sourceFramePTS=times, sourceTimeOriginSeconds=origin)


def journey(name, original, metadata, offset=0, cold=False, one_shot=False, audio_conversion=False):
    case = {'name': name, 'result': 'failed', 'fixture': metadata, 'resumeOffsetSeconds': offset, 'failures': []}
    receipt['cases'].append(case)
    guard.check(40)
    directory, media = RUN / name, RUN / name / 'media'
    media.mkdir(parents=True)
    source = media / ('Fixture' + original.suffix)
    shutil.copy2(original, source)
    before = source_state(source)
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    url = f'http://localhost:{port}'
    env = dict(os.environ, KINOSAIL_LISTEN=f'127.0.0.1:{port}', KINOSAIL_AUTH_URL=url,
        KINOSAIL_TLS_ENABLED='false', KINOSAIL_DATA_DIR=str(directory / 'config'),
        KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(directory / 'cache'),
        KINOSAIL_BACKUP_DIR=str(directory / 'backups'), KINOSAIL_BACKUP_KEY='synthetic-followon-key')
    api = CaptureServer(url, directory)
    resources = {'samples': 0, 'peakOwnedFFmpeg': 0, 'samplingErrors': 0}
    case['resources'] = resources
    log_path = directory / 'server.log'
    with log_path.open('w') as log:
        server, sampler = None, None
        stop = threading.Event()
        try:
            server = subprocess.Popen([str(binary)], cwd=ROOT, env=env,
                stdout=log, stderr=log, start_new_session=True)
            sampler = threading.Thread(target=sample_resources,
                args=(server, source, stop, resources), daemon=True)
            sampler.start()
            api.authorize()
            item_id = next(i['id'] for i in api.call('/api/v1/library')['items'] if i['title'] == 'Fixture')
            plan = api.call('/api/v1/items/' + item_id + '/playback?videoCodecs=h264&audioCodecs=aac')
            case['mode'] = plan['compatiblePlan']['mode']
            check(case['mode'] == ('audio-transcode' if audio_conversion else 'remux'), 'expected_compatibility_mode')
            hls = plan['compatible']
            origin = metadata['sourceTimeOriginSeconds']
            case['referenceClock'] = {'originSeconds': origin, 'requestedSourcePTS': origin + offset,
                'expectedFrames': sum(point >= origin + offset - 0.000001 for point in metadata['sourceFramePTS']),
                'method': 'independent ffprobe decoded source PTS minus demux format start_time'}
            expected = [v for v in metadata['sourceFramePTS'] if v >= origin + offset - 0.000001]
            check(len(expected) == {0: 768, 12: 480, 12.5: 468}[offset], 'literal_reference_frame_count')
            expected_starts_at_key = any(abs(k - expected[0]) <= 0.0000011
                                        for k in metadata['keyframesSeconds'])
            check(expected_starts_at_key == (offset != 12.5), 'independent_key_control')
            case['referenceClock']['firstExpectedFrameIsKey'] = expected_starts_at_key
            if offset and offset % 2:
                check(all(abs(k - origin - offset) > 0.05 for k in metadata['keyframesSeconds']), 'fixture_offset_is_key')
            if offset:
                hls = hls.replace('/index.m3u8', '-o' + str(round(offset * 1000)) + '/index.m3u8')
            check(re.fullmatch(r'/hls/[a-f0-9]{16}/p/[ra]-[a-zA-Z0-9-]+/index\.m3u8', hls), 'planned_recipe_route')
            case['expectedTimelineSeconds'] = plan['duration'] - offset
            check(abs(plan['duration'] - metadata['durationSeconds']) < 0.1, 'source_duration')
            check(plan['media']['fileVersion'] == str(before['sizeBytes']) + ':' + before['mtimeNanoseconds'],
                  'source_snapshot_binding')
            prepare, cache = '/api/v1/items/' + item_id + '/playback-prepare', directory / 'cache'
            roots = lambda: sorted(p.name for p in cache.iterdir() if p.name.startswith(item_id))
            check(not roots() and encoder_count(server, source) == 0, 'planning_side_effect')
            status, _, _ = api.http(prepare, 'POST', {'source': hls}, authenticated=False)
            check(status == 401 and not roots() and encoder_count(server, source) == 0, 'unauthenticated_side_effect')
            case['unauthenticatedPreparation'] = {'status': status, 'cacheUnchanged': True}
            prepared_init = None
            if one_shot:
                prepare_once(api, prepare, hls, log_path, server, source, case)
                case['preparationReady'] = False
            else:
                _, prepared_init = prepare_scene(api, prepare, hls, cache, item_id, server, source, case,
                    encoder_count, check, bounded_bytes, cold)
            measure(api, hls, directory, source, metadata, offset, prepared_init, case)
            case['originalPublicAssertions'] = list(case['failures'])
            physical = sorted(directory.glob('segment-*.m4s'))
            case['observations'] = {}
            observed_media(source, directory / 'public.mp4', api.initialization,
                           physical, metadata, offset, case['observations'])
            if not case['observations']['packetClockQualification']['qualified']:
                case['failures'].append('nonkey_packet_clock')
            if not case['observations']['mapping']['exactRequestedSequence']:
                case['failures'].append('exact_requested_source_sequence')
            if not case['observations']['nativePCM']['publicAndSourceCompleteEOFAccounted']:
                case['failures'].append('complete_native_pcm_eof_accounting')
            if not case['observations']['nativePCM']['wholePublicEqualsReference']:
                case['failures'].append('full_native_pcm_reference')
            case['result'] = 'passed' if not case['failures'] else 'failed'
        except Exception as error:
            case['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
        finally:
            try:
                if server is not None:
                    with guard.cleanup():
                        case.update(finish_processes(server, source, stop, sampler))
            except (OSError, subprocess.SubprocessError) as error:
                case['cleanupFailureClass'] = type(error).__name__
                case['failures'].append('owned_process_cleanup_failed')
                case['result'] = 'failed'
            log.flush()
            private = bounded_bytes(log_path, 2 * 1024 * 1024, 'private_log_bound').decode()
            case.update(safe_seek_phases(private))
            lifecycle = safe_encoder_lifecycle(private)
            case['encoderLifecycle'] = lifecycle
            joined = case.get('ownedProcessJoin', {})
            case['workerBound'] = (joined.get('confirmedZeroSamples') == 2
                and joined.get('remainingOwnedPIDs') == [] and not joined.get('forcedOwnedGroupStop')
                and not joined.get('qualificationFailures') and not case.get('cleanupFailures')
                and not case.get('cleanupFailureClass') and (sampler is None or not sampler.is_alive())
                and resources['samples'] > 0 and resources['peakOwnedFFmpeg'] <= 1
                and resources['samplingErrors'] == 0 and lifecycle['validSequence'] and lifecycle['peakActive'] == 1
                and case.get('ownedFFmpegBeforeTeardown') == 0)
            if not case['workerBound']:
                case['failures'].append('owned_encoder_bound')
                case['result'] = 'failed'
            case['sourceUnchanged'] = before == source_state(source)
            case['sourceBefore'], case['sourceAfter'] = before, source_state(source)
            if not case['sourceUnchanged']:
                case['failures'].append('source_mutated')
                case['result'] = 'failed'


try:
    check(receipt['trackedSourceClean'], 'nonkey_dirty_tracked_source')
    subprocess.run(['go', '-C', 'apps/player', 'build', '-p=1', '-o', str(binary), './cmd/kinosail'],
        cwd=ROOT, check=True, timeout=180)
    receipt['binarySHA256'] = sha(binary)
    receipt['encoderVersions'] = {t: subprocess.check_output([t, '-version'], text=True).splitlines()[0]
                                 for t in ['ffmpeg', 'ffprobe']}
    regular, metadata = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
    metadata = reprobe_source(regular, metadata)
    mp4, mp4_metadata = convert(regular, metadata)
    for name, original, facts in [('mkv', regular, metadata), ('mp4', mp4, mp4_metadata)]:
        for offset in [0, 12, 12.5]:
            journey(name + '-' + str(offset), original, facts, offset, cold=offset == 0, one_shot=offset > 0)
    check(len(receipt['cases']) == 6, 'nonkey_exact_case_count')
    receipt['result'] = 'passed' if all(c['result'] == 'passed' for c in receipt['cases']) else 'failed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    with guard.cleanup():
        receipt['handledTerminationSignals'] = guard.signals
        target = RUN / 'receipt.json'
        files = {Path(__file__)} | {Path(m.__file__).resolve() for m in list(sys.modules.values())
            if getattr(m, '__file__', None) and Path(m.__file__).resolve().parent == Path(__file__).resolve().parent}
        receipt['executedScriptSHA256'] = {str(p.relative_to(ROOT)): sha(p) for p in sorted(files)}
        raw = json.dumps(receipt, separators=(',', ':'), allow_nan=False) + '\n'
        check(0 < len(raw.encode()) <= 16 << 20, 'nonkey_lossless_receipt_bound')
        target.write_text(raw)
        modules = sorted(files)
        (RUN / 'SHA256SUMS').write_text(''.join(sha(p) + '  ' + str(p.relative_to(ROOT)) + '\n'
            for p in modules) + sha(target) + '  receipt.json\n')
        print(json.dumps({'result': receipt['result'], 'revision': receipt['revision'],
            'receiptSHA256': sha(target), 'failureClass': receipt.get('failureClass'),
            'cases': [{k: c.get(k) for k in ['name', 'result', 'failureClass', 'failures',
                'workerBound', 'sourceUnchanged', 'presentation', 'encoderLifecycle']}
                for c in receipt['cases']]}))
    guard.__exit__()
raise SystemExit(0 if receipt['result'] == 'passed' else 1)
