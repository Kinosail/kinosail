#!/usr/bin/env python3
"""Strict public audio completion and non-key diagnostics; never production proof of preroll."""
import argparse
import json
import math
import os
from pathlib import Path
import re
import shutil
import signal
import socket
import subprocess
import threading
import time
from hls_remaining_process import annotate_case, finish_processes, physical, source_snapshot
from hls_remaining_audio import audio_output, replay_refill
from hls_remaining_mux import counterfactuals
from hls_remaining_warmup import warmup_counterfactual
from hls_remaining_installation import installation_cases
from hls_remaining_origin import origin_cases, origin_refill_witness, origin_source_witness, origin_selected_audio
from hls_remaining_reopen import cold_reopen
from hls_timeline_fixture import fixture
from hls_timeline_http import PublicServer, sha
from hls_followon_frames import decode_frames, stream_metadata
from hls_followon_public import bounded_bytes, check, encoder_count, measure, prepare_once, sample_resources

ROOT = Path(__file__).resolve().parents[3]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--suite', choices=['remaining', 'audio-timing', 'audio-installation', 'audio-origin'], default=os.environ.get('KINOSAIL_HLS_REMAINING_SUITE', 'remaining'))
SUITE = parser.parse_args().suite
if SUITE not in ['remaining', 'audio-timing', 'audio-installation', 'audio-origin']:
    parser.error('unsupported diagnostic suite')
RUN = ROOT / '.verification/hls-followon' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
BINARY = RUN / 'player'
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'result': 'failed', 'cases': [], 'suite': SUITE,
    'command': 'python3 apps/player/scripts/test-hls-remaining.py --suite ' + SUITE,
    'boundary': 'Disposable authenticated public Server. Raw negative frames retained; browser/native/audible qualification separate.',
    'productionMediaOrCacheModified': False, 'expectedCases': {'remaining': 6, 'audio-timing': 5, 'audio-installation': 3, 'audio-origin': 2}[SUITE]}
RUN_DEADLINE = time.monotonic() + 500


def alarm(_signal, _frame):
    raise RuntimeError('bounded_diagnostic_deadline')


signal.signal(signal.SIGALRM, alarm)
signal.signal(signal.SIGTERM, alarm)


def remaining_alarm():
    signal.setitimer(signal.ITIMER_REAL, max(0.001, RUN_DEADLINE - time.monotonic()))


def run(command, seconds=30):
    remaining = RUN_DEADLINE - time.monotonic()
    check(remaining > 0, 'bounded_run_deadline')
    value = subprocess.run(command, capture_output=True, timeout=min(seconds, remaining))
    check(value.returncode == 0, 'diagnostic_command_failed')
    check(len(value.stdout) <= 2 * 1024 * 1024, 'diagnostic_output_bound')
    return value.stdout


def reprobe(path, metadata):
    facts = stream_metadata(path)
    frames = json.loads(run(['ffprobe', '-v', 'error', '-select_streams', 'v:0', '-show_frames',
        '-show_entries', 'frame=best_effort_timestamp_time,key_frame', '-of', 'json', str(path)]))['frames']
    times = [float(row['best_effort_timestamp_time']) for row in frames]
    keys = [float(row['best_effort_timestamp_time']) for row in frames if row['key_frame']]
    origin = float(facts['format']['start_time'])
    check(len(times) == 768 and math.isfinite(origin) and all(math.isfinite(v) for v in times)
        and all(a < b for a, b in zip(times, times[1:])), 'source_independent_clock')
    return dict(metadata, sourceFramePTS=times, keyframesSeconds=keys, sourceTimeOriginSeconds=origin,
        durationSeconds=float(facts['format']['duration']), streamOrigins=facts)


def journey(name, original, metadata, offset=0, pacing=None, installation=False):
    case = {'name': name, 'result': 'failed', 'failures': [], 'resumeOffsetSeconds': offset}
    receipt['cases'].append(case)
    remaining = RUN_DEADLINE - time.monotonic()
    check(remaining > 30, 'bounded_run_deadline')
    directory = RUN / name
    media = directory / 'media'
    media.mkdir(parents=True)
    source = media / ('Fixture' + original.suffix)
    shutil.copy2(original, source)
    before = source_snapshot(source)
    case['fixture'] = dict(metadata, **before)
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    api = PublicServer(f'http://localhost:{port}')
    env = dict(os.environ, KINOSAIL_LISTEN=f'127.0.0.1:{port}', KINOSAIL_AUTH_URL=api.url,
        KINOSAIL_TLS_ENABLED='false', KINOSAIL_DATA_DIR=str(directory / 'config'),
        KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(directory / 'cache'),
        KINOSAIL_BACKUP_DIR=str(directory / 'backups'), KINOSAIL_BACKUP_KEY='synthetic-remaining-key')
    if pacing is not None:
        real = shutil.which('ffmpeg')
        check(real is not None, 'ffmpeg_unavailable')
        wrapper = directory / 'paced-ffmpeg'
        invocation = directory / 'pacing-invocations.jsonl'
        wrapper.write_text('#!/usr/bin/env python3\nimport os,sys,json\na=sys.argv[1:]\n'
            "if '-hls_time' in a:\n"
            " if '-readrate' in a: a[a.index('-readrate')+1]=" + repr(str(pacing)) + '\n'
            " else: a[a.index('-i'):a.index('-i')]=['-readrate'," + repr(str(pacing)) + ']\n'
            + ' with open(' + repr(str(invocation)) + ", 'a') as f: f.write(json.dumps({'pid':os.getpid(),'parent':os.getppid(),'sourceMatched':" + repr(str(source)) + " in a,'readrate':a[a.index('-readrate')+1]})+'\\n')\n"
            + " if '-start_number' in a and " + repr(str(source)) + " in a:\n"
            + '  value=json.dumps(a)\n  if len(value.encode())<=8192:\n'
            + '   with open(' + repr(str(directory / 'refill-recipe-private.json')) + ", 'w') as f: f.write(value)\n"
            + ("  if a[a.index('-start_number')+1]=='4':\n   sys.path.insert(0," + repr(str(Path(__file__).parent)) + ")\n   from hls_remaining_installation import installed_refill\n"
                + "   a=installed_refill(a," + repr(str(source)) + ',' + repr(str(directory))
                + ",os.environ.get('KINOSAIL_INSTALLATION_RECEIPT_DIR'))\n" if installation else '')
            + 'os.execv(' + repr(real) + ',[' + repr(real) + ']+a)\n')
        wrapper.chmod(0o700)
        env['KINOSAIL_FFMPEG'] = str(wrapper)
        case['testOnlyRealCodecPacing'] = {'readrate': pacing, 'wrapperSHA256': sha(wrapper), 'executableSHA256': sha(Path(real))}
    resources = {'samples': 0, 'peakOwnedFFmpeg': 0, 'samplingErrors': 0}
    case['resources'] = resources
    log_path = directory / 'server.log'
    item, server, sampler = None, None, None
    stop = threading.Event()
    with log_path.open('w') as log:
        try:
            signal.setitimer(signal.ITIMER_REAL, min(90, remaining - 20))
            if 'sourceFramePTS' in metadata:
                check(abs(metadata['sourceTimeOriginSeconds']) <= 0.000001, 'fixture_zero_origin')
                keys = metadata['keyframesSeconds']
                check(any(abs(v - offset) <= 0.000001 for v in keys) if name.startswith('exact-key') else all(abs(v - offset) > 0.05 for v in keys), 'fixture_key_eligibility')
            server = subprocess.Popen([str(BINARY)], cwd=ROOT, env=env, stdout=log, stderr=log, start_new_session=True)
            sampler = threading.Thread(target=sample_resources, args=(server, source, stop, resources), daemon=True)
            sampler.start()
            api.authorize()
            item = next(i for i in api.call('/api/v1/library')['items'] if i['title'] == 'Fixture')
            check(re.fullmatch(r'[a-f0-9]{16}', item['id']), 'item_id_shape')
            plan = api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264&audioCodecs=aac')
            hls = plan['compatible']
            if offset:
                hls = hls.replace('/index.m3u8', '-o' + str(round(offset * 1000)) + '/index.m3u8')
            check(re.fullmatch(r'/hls/[a-f0-9]{16}/p/[ra]-[a-zA-Z0-9-]+/index\.m3u8', hls), 'planned_recipe_route')
            check(plan['media']['fileVersion'] == str(before['sizeBytes']) + ':' + before['mtimeNanoseconds'], 'source_snapshot_binding')
            case['mode'] = plan['compatiblePlan']['mode']
            if SUITE == 'audio-origin':
                case['originSelectedAudio'] = origin_selected_audio(plan)
            check(case['mode'] == ('audio-transcode' if item['kind'] == 'audio' else 'remux'), 'expected_compatibility_mode')
            case['planDurationSeconds'] = plan['duration']
            if installation:
                selected = next(v for v in plan['media']['audio'] if v['Index'] == plan['compatiblePlan'].get('audioIndex', 0))
                audio = {k: selected[v] for k, v in [('codec', 'Codec'), ('sampleRate', 'SampleRate'), ('channels', 'Channels'), ('index', 'Index')]}
                (directory / 'installation-eligibility-private.json').write_text(json.dumps({'audio': audio, 'sourceSHA256': before['sha256']}))
            check(hls.split('/')[2] == item['id'], 'route_item_binding')
            check(abs(plan['duration'] - metadata.get('probedDurationSeconds', metadata.get('durationSeconds'))) < 0.1, 'plan_probe_duration')
            prepare = '/api/v1/items/' + item['id'] + '/playback-prepare'
            cache = directory / 'cache'
            check(encoder_count(server, source) == 0 and not list(cache.glob(item['id'] + '-plan-*')), 'planning_side_effect')
            status, _, _ = api.http(prepare, 'POST', {'source': hls}, authenticated=False)
            check(status == 401 and encoder_count(server, source) == 0 and not list(cache.glob(item['id'] + '-plan-*')), 'unauthenticated_side_effect')
            case['unauthenticatedPreparation'] = {'status': status, 'cacheUnchanged': True}
            prepare_once(api, prepare, hls, log_path, server, source, case)
            if item['kind'] == 'audio':
                if case['preparationAttempt']['completionState'] != 'ready':
                    case['failures'].append('audio_preparation_not_ready')
                case['physicalBeforeFirstGET'] = physical(cache, item['id'])
                check(case['physicalBeforeFirstGET'] is not None, 'physical_prefix_missing')
                audio_output(api, hls, directory, case, source, run, RUN_DEADLINE)
                case['physicalAfterPublicDelivery'] = physical(cache, item['id'])
            else:
                full, source_rows = decode_frames(source)
                origin = metadata['sourceTimeOriginSeconds']
                expected = [n for n, point in enumerate(metadata['sourceFramePTS']) if point >= origin + offset - 0.000001]
                case['referenceClock'] = {'originSeconds': origin, 'requestedSourcePTS': origin + offset,
                    'expectedFrames': len(expected)}
                case['expectedTimelineSeconds'] = plan['duration'] - offset
                measure(api, hls, directory, source, metadata, offset, None, case)
                public_rows = case['timestampedFrameEvidence']
                index = {v: n for n, (_, v) in enumerate(source_rows)}
                mapped = [index.get(row['md5']) for row in public_rows]
                case['rawSourceCorrespondence'] = {'expectedSourceIndices': expected, 'publicSourceIndices': mapped,
                    'sourceHashesUnique': len(index) == len(source_rows), 'negativeFramesRetained': True}
                if len(index) != len(source_rows) or mapped != expected:
                    case['failures'].append('exact_requested_source_sequence')
            case['result'] = 'passed' if not case['failures'] else 'failed'
        except Exception as error:
            case['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
        finally:
            signal.setitimer(signal.ITIMER_REAL, 0)
            signal.signal(signal.SIGTERM, signal.SIG_IGN)
            try:
                if server is not None:
                    case.update(finish_processes(server, source, stop, sampler))
            except Exception as error:
                case['cleanupFailureClass'] = type(error).__name__
                case['failures'].append('owned_process_join_failed')
                case['result'] = 'failed'
            signal.signal(signal.SIGTERM, alarm)
            log.flush()
            try:
                annotate_case(case, log_path, source, before, server, resources, offset, pacing,
                    invocation if pacing is not None else None, item is not None and item['kind'] == 'audio')
            except Exception as error:
                case['diagnosticFailureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
                case['failures'].append('diagnostic_evidence_incomplete')
                case['result'] = 'failed'
            remaining_alarm()
            if installation and case.get('retainedRefillFragments'):
                try:
                    certificate = directory / 'installation-certificate.json'
                    case['installationCertificate'] = json.loads(bounded_bytes(certificate, 4096, 'installation_certificate_bound'))
                    facts = case['installationCertificate']
                    check(sha(directory / 'refill-recipe-private.json') == facts['originalArgvSHA256'] and
                        sha(directory / 'installed-recipe-private.json') == facts['installedArgvSHA256'], 'installation_actual_argv_certificate')
                    cold_reopen(run, BINARY, directory, source, before, api, hls, env, RUN_DEADLINE, case)
                except (RuntimeError, OSError, subprocess.SubprocessError, ValueError, KeyError) as error:
                    case['installationQualificationFailure'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
            if SUITE == 'audio-origin' and case.get('retainedRefillFragments'):
                try:
                    origin_refill_witness(directory, source, case)
                    cold_reopen(run, BINARY, directory, source, before, api, hls, env, RUN_DEADLINE, case)
                except (RuntimeError, OSError, subprocess.SubprocessError, ValueError, KeyError) as error:
                    case['originWitnessFailure'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
            if case.get('retainedRefillFragments') and SUITE not in ['audio-installation', 'audio-origin']:
                try:
                    check(pacing is not None, 'fresh_refill_actual_argv_unavailable')
                    value = replay_refill(run, RUN_DEADLINE, directory, source, case, Path(real))
                    case['isolatedFreshRefillReplay'] = value
                    value['matchesCanonicalPCM'] = value['pcmSHA256'] == case['refillNativeEOF']['pcmSHA256']
                except (RuntimeError, OSError, subprocess.SubprocessError, ValueError, KeyError) as error:
                    policy = 'outputPolicyCounterfactual' in case.get('isolatedFreshRefillReplay', {})
                    case['outputPolicyQualificationFailure' if policy else 'replayQualificationFailure'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
                    case['failures'].append('aac_output_policy_unqualified' if policy else 'fresh_refill_replay_unqualified')
                    case['result'] = 'failed'
                    if isinstance(error, RuntimeError) and str(error) in ['bounded_diagnostic_deadline', 'bounded_run_deadline']:
                        raise


try:
    remaining_alarm()
    if SUITE == 'audio-origin':
        receipt['productionSourceWitness'] = origin_source_witness(ROOT)
    subprocess.run(['go', '-C', 'apps/player', 'build', '-p=1', '-o', str(BINARY), './cmd/kinosail'], cwd=ROOT, check=True, timeout=180)
    receipt['binarySHA256'] = sha(BINARY)
    receipt['encoderVersions'] = {t: run([t, '-version']).decode().splitlines()[0] for t in ['ffmpeg', 'ffprobe']}
    cases = [] if SUITE in ['audio-installation', 'audio-origin'] else ([(10, 0.75, False), (10, 0.9, False), (8, 0.75, False), (10, 0.75, True), (10, 0.9, True)] if SUITE == 'audio-timing' else [(10, None, False), (10, 1.25, False), (8, 1.25, False)])
    for duration, paced, marker in cases:
        name = f'{"marker" if marker else "audio"}-{duration}-{paced}'
        source = RUN / (name + '.flac')
        generated = "aevalsrc='0.1*sin(2*PI*(440*t+20*t*t))+0.4*between(t,7.995,8.005)':s=48000:d=10" if marker else f'sine=frequency=440:sample_rate=48000:duration={duration}'
        run(['ffmpeg', '-nostdin', '-v', 'error', '-f', 'lavfi', '-i', generated, '-c:a', 'flac', '-ac', '2', str(source)], 60)
        probe = json.loads(run(['ffprobe', '-v', 'error', '-show_entries', 'format=duration', '-of', 'json', str(source)]))
        journey(name, source, {'probedDurationSeconds': float(probe['format']['duration']),
            'markerNear8Seconds': marker, 'fixtureFilter': generated}, pacing=paced)
    if SUITE == 'audio-installation':
        installation_cases(run, journey, RUN, receipt, RUN_DEADLINE)
    if SUITE == 'audio-origin':
        origin_cases(run, journey, RUN, receipt, RUN_DEADLINE)
        check(origin_source_witness(ROOT) == receipt['productionSourceWitness'], 'origin_production_source_changed')
        receipt['productionSourceWitness']['unchangedAfterPublicProof'] = True
    if SUITE == 'remaining':
        source, metadata = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
        metadata = reprobe(source, metadata)
        journey('exact-key-control12', source, metadata, offset=12)
        journey('nonkey-mkv12.5', source, metadata, offset=12.5)
        mp4 = RUN / 'copy.mp4'
        run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(source), '-map', '0:v:0', '-map', '0:a:0', '-c', 'copy', str(mp4)], 60)
        mp4_metadata = reprobe(mp4, dict(metadata, sha256=sha(mp4)))
        journey('nonkey-mp4-12.5', mp4, mp4_metadata, offset=12.5)
        for kind, path, facts in [('mkv', source, metadata), ('mp4', mp4, mp4_metadata)]:
            result = {'cases': [], 'qualificationFailures': []}
            receipt[kind + 'MuxCounterfactual'] = result
            try:
                counterfactuals(run, path, facts, RUN / (kind + '-mux'), result)
            except (RuntimeError, OSError, subprocess.SubprocessError, ValueError, KeyError) as error:
                result['qualificationFailures'].append(str(error) if isinstance(error, RuntimeError) else type(error).__name__)
                if isinstance(error, RuntimeError) and str(error) in ['bounded_diagnostic_deadline', 'bounded_run_deadline']:
                    raise
    if len(receipt['cases']) == receipt['expectedCases'] and all(c['result'] == 'passed' for c in receipt['cases']):
        receipt['result'] = 'passed'
    if SUITE == 'audio-timing':
        result = {'cases': [], 'qualificationFailures': []}
        receipt['aacWarmupCounterfactual'] = result
        try:
            pair = [next(v for v in receipt['cases'] if v['name'] == name) for name in ['marker-10-0.75', 'marker-10-0.9']]
            warmup_counterfactual(run, RUN_DEADLINE, RUN, result, *pair, Path(shutil.which('ffmpeg')))
        except (RuntimeError, OSError, subprocess.SubprocessError, ValueError, KeyError) as error:
            result['qualificationFailures'].append(str(error) if isinstance(error, RuntimeError) else type(error).__name__)
            if isinstance(error, RuntimeError) and str(error) in ['bounded_diagnostic_deadline', 'bounded_run_deadline']:
                raise
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    signal.setitimer(signal.ITIMER_REAL, 0)
    encoded = json.dumps(receipt, separators=(',', ':')) + '\n'
    check(len(encoded.encode()) <= 4 << 20, 'receipt_budget')
    (RUN / 'receipt.json').write_text(encoded)
    helpers = [Path(__file__), *(ROOT / 'apps/player/scripts').glob('hls_*.py')]
    check(len(helpers) <= 32, 'helper_manifest_budget')
    checksums = {'receipt.json': sha(RUN / 'receipt.json'), **{str(p.relative_to(ROOT)): sha(p) for p in helpers}}
    (RUN / 'SHA256SUMS').write_text(''.join(f'{value}  {name}\n' for name, value in checksums.items()))
    print(json.dumps({'revision': receipt['revision'], 'result': receipt['result'], 'cases': [{'name': c['name'], 'result': c['result'], 'failures': c['failures'], 'failureClass': c.get('failureClass')} for c in receipt['cases']]}))
raise SystemExit(0 if receipt['result'] == 'passed' else 1)
