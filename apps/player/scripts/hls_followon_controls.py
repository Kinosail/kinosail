"""Public preparation regressions outside the H264 copied-video admission."""
import hashlib
import json
import os
import re
import socket
import subprocess
import threading
import time
from hls_timeline_http import PublicServer, source_state
from hls_timeline_packets import manifest_facts, safe_encoder_lifecycle, safe_seek_phases
from hls_timeline_preparation import prepare_scene
from hls_followon_public import check, bounded_bytes, encoder_count, sample_resources, prepare_once
from hls_followon_hevc import evidence, fragment_evidence, seek_diagnostics, validate_public_output
from hls_followon_cancel import interrupted_preparation, preparation_prefix, preparation_states


def controls(root, run, binary, receipt, include_hevc=True, include_audio=True):
    for name, extension, codec in [('audio-only', '.flac', 'flac'),
                                   ('audiobook', '.m4b', 'alac'), ('hevc-video', '.mkv', 'ac3'),
                                   ('hevc-cold', '.mkv', 'ac3'), ('hevc-interrupted-preparation', '.mkv', 'ac3'),
                                   ('hevc-adopted-preparation', '.mkv', 'ac3')]:
        hevc = name.startswith('hevc-')
        if hevc and not include_hevc:
            continue  # Full manual proof retains strict HEVC counter/control cases.
        if not hevc and not include_audio:
            continue  # The separate HEVC gate leaves audio6 in its existing gate.
        case = {'name': name, 'result': 'failed', 'failures': [],
                'boundary': 'Public complete output for matched zero-offset synthetic media; general HEVC seeks not certified'}
        receipt['cases'].append(case)
        directory = run / name
        media = directory / 'media'
        media.mkdir(parents=True)
        source = media / ('Fixture' + extension)
        command = ['ffmpeg', '-nostdin', '-v', 'error']
        if hevc:
            command += ['-f', 'lavfi', '-i', 'testsrc2=s=320x180:r=24:d=10']
        audio = ('aevalsrc=0.2*sin(2*PI*(440+110*floor(t/2))*t):s=48000:d=10' if hevc
                 else 'sine=frequency=440:sample_rate=48000:duration=10')
        command += ['-f', 'lavfi', '-i', audio]
        if hevc:
            command += ['-c:v', 'libx265', '-preset', 'ultrafast', '-threads', '1',
                        '-x265-params', 'pools=1:frame-threads=1:keyint=48:min-keyint=48:scenecut=0']
        command += ['-c:a', codec, '-ac', '2', str(source)]
        subprocess.run(command, check=True, timeout=60, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        before = source_state(source)
        case['fixture'] = {'command': command, **before}
        with socket.socket() as listener:
            listener.bind(('127.0.0.1', 0))
            port = listener.getsockname()[1]
        url = f'http://localhost:{port}'
        env = dict(os.environ, KINOSAIL_LISTEN=f'127.0.0.1:{port}', KINOSAIL_AUTH_URL=url,
            KINOSAIL_TLS_ENABLED='false', KINOSAIL_DATA_DIR=str(directory / 'config'),
            KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(directory / 'cache'),
            KINOSAIL_BACKUP_DIR=str(directory / 'backups'), KINOSAIL_BACKUP_KEY='synthetic-control-key')
        api = PublicServer(url)
        resources = {'samples': 0, 'peakOwnedFFmpeg': 0, 'samplingErrors': 0}
        case['resources'] = resources
        log_path = directory / 'server.log'
        with log_path.open('w') as log:
            server = subprocess.Popen([str(binary)], cwd=root, env=env, stdout=log, stderr=log)
            stop = threading.Event()
            sampler = threading.Thread(target=sample_resources, args=(server, source, stop, resources), daemon=True)
            sampler.start()
            try:
                api.authorize()
                item = next(i for i in api.call('/api/v1/library')['items'] if i['title'] == 'Fixture')
                query = '?videoCodecs=hevc&audioCodecs=aac' if hevc else '?videoCodecs=h264&audioCodecs=aac'
                plan = api.call('/api/v1/items/' + item['id'] + '/playback' + query)
                check(plan['compatiblePlan']['mode'] == 'audio-transcode', 'control_compatibility_mode')
                case['itemKind'], case['mode'] = item['kind'], plan['compatiblePlan']['mode']
                hls = plan['compatible']
                check(re.fullmatch(r'/hls/[a-f0-9]{16}/p/a-[a-zA-Z0-9-]+/index\.m3u8', hls), 'control_route')
                prepare = '/api/v1/items/' + item['id'] + '/playback-prepare'
                cache = directory / 'cache'
                roots = lambda: sorted(p.name for p in cache.iterdir() if p.name.startswith(item['id']))
                check(not roots() and encoder_count(server, source) == 0, 'control_planning_side_effect')
                status, _, _ = api.http(prepare, 'POST', {'source': hls}, authenticated=False)
                check(status == 401 and not roots() and encoder_count(server, source) == 0, 'control_unauthenticated_side_effect')
                case['unauthenticatedPreparation'] = {'status': status, 'cacheUnchanged': True}
                if name == 'hevc-interrupted-preparation':
                    interrupted_preparation(api, prepare, hls, cache, item['id'], server, source, log_path, case)
                elif name == 'hevc-adopted-preparation':
                    preparation_prefix(api, prepare, hls, cache, item['id'], server, source, case, 'adoptedPreparation')
                elif name == 'hevc-cold':
                    prepare_scene(api, prepare, hls, cache, item['id'], server, source, case,
                                  encoder_count, check, bounded_bytes, cold=True)
                else:
                    prepare_once(api, prepare, hls, log_path, server, source, case)
                    check(case['preparationAttempt']['completionState'] == 'ready', 'control_preparation_not_ready')
                    value = api.call(prepare, 'POST', {'source': hls}, 202)
                    check(value['state'] == 'ready', 'control_public_preparation_not_ready')
                if hevc and name not in ['hevc-interrupted-preparation', 'hevc-adopted-preparation']:
                    cache_roots = roots()
                    check(len(cache_roots) == 1, 'hevc_prepared_cache_root')
                    initializations = list((cache / cache_roots[0]).glob('*/init.mp4'))
                    check(len(initializations) == 1, 'hevc_prepared_initialization')
                    prepared_init = bounded_bytes(initializations[0], 1024 * 1024, 'hevc_initialization_bound')
                    case['preparedInitializationSHA256'] = hashlib.sha256(prepared_init).hexdigest()
                    raw_variant = bounded_bytes(initializations[0].with_name('index.m3u8'), 64 * 1024,
                                                'hevc_prepared_manifest_bound')
                    case['rawPreparedManifest'] = {'sha256': hashlib.sha256(raw_variant).hexdigest(),
                        'endlist': b'#EXT-X-ENDLIST' in raw_variant}
                if name == 'hevc-adopted-preparation':
                    check(encoder_count(server, source) == 1, 'hevc_adoption_not_active')
                status, master, _ = api.http(hls)
                check(status == 200, 'control_master_http_' + str(status))
                renditions = re.findall(r'^(?:[1-9][0-9]{2,3}p|audio)/index\.m3u8$', master.decode(), re.M)
                check(len(renditions) == 1, 'control_rendition')
                base = hls.removesuffix('index.m3u8') + renditions[0].removesuffix('index.m3u8')
                status, variant, _ = api.http(base + 'index.m3u8')
                case['controlTimelineResponse'] = {'status': status, 'endlist': b'#EXT-X-ENDLIST' in variant,
                    'sha256': hashlib.sha256(variant).hexdigest(),
                    'playlistPrefix': variant[:4096].decode(errors='replace')}
                check(status == 200 and b'#EXT-X-ENDLIST' in variant, 'control_complete_timeline')
                status, init, _ = api.http(base + 'init.mp4')
                check(status == 200 and init, 'control_initialization')
                facts, advertised = manifest_facts(variant)
                segments = [filename for filename, _ in advertised]
                check(0 < len(segments) <= 16, 'control_fragment_bound')
                case['publicVariant'] = facts
                fragments = [init]
                case['fragmentAudioPackets'] = []
                if hevc:
                    case['fragmentPacketEvidence'] = []
                for filename, advertised_seconds in advertised:
                    status, data, _ = api.http(base + filename)
                    check(status == 200 and data, 'control_fragment_http_' + str(status))
                    fragments.append(data)
                    fragment = directory / 'fragment.mp4'
                    fragment.write_bytes(init + data)
                    packets = subprocess.check_output(['ffprobe', '-v', 'error', '-select_streams', 'a:0',
                        '-read_intervals', '%+#4096', '-show_packets', '-show_entries', 'packet=pts_time,duration_time',
                        '-of', 'json', str(fragment)], timeout=30)
                    check(len(packets) <= 1024 * 1024, 'control_audio_packet_bound')
                    rows = json.loads(packets).get('packets', [])
                    check(len(rows) < 4096, 'control_audio_packet_bound')
                    case['fragmentAudioPackets'].append({'segment': filename, 'packets': len(rows),
                        'first': rows[0] if rows else None, 'last': rows[-1] if rows else None})
                    if hevc:
                        case['fragmentPacketEvidence'].append(
                            fragment_evidence(fragment, rows, filename, advertised_seconds))
                joined = directory / 'public.mp4'
                joined.write_bytes(b''.join(fragments))
                pcm = subprocess.check_output(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(joined),
                    '-map', '0:a:0', '-t', '12', '-ac', '1', '-ar', '16000', '-f', 's16le', '-'], timeout=30)
                case['decodedAudioSamples'], case['publicFragments'] = len(pcm) // 2, len(segments)
                source_pcm = subprocess.check_output(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(source),
                    '-map', '0:a:0', '-ac', '1', '-ar', '16000', '-f', 's16le', '-'], timeout=30)
                check(0 < len(source_pcm) <= 512 * 1024 and len(source_pcm) % 2 == 0, 'control_source_audio_bound')
                case['sourceAudioSamples'] = len(source_pcm) // 2
                if hevc:
                    status, final_init, _ = api.http(base + 'init.mp4')
                    check(status == 200 and final_init, 'hevc_final_initialization')
                    case['preparedInitializationRetained'] = (hashlib.sha256(init).hexdigest() ==
                        case['preparedInitializationSHA256'])
                    case['initializationStable'] = (init == final_init and
                        (name == 'hevc-interrupted-preparation' or case['preparedInitializationRetained']))
                    evidence(source, joined, case)
                    validate_public_output(case)
                    if name == 'hevc-adopted-preparation':
                        states = preparation_states(log_path, case['adoptedPreparation']['requestID'])
                        case['adoptedPreparation']['completionState'] = states[-1] if states else None
                        check(states and states[-1] == 'adopted', 'hevc_adoption_not_correlated')
                if name == 'hevc-cold':
                    try:
                        seek_diagnostics(source, joined, directory, case)
                    except Exception as error:
                        case.setdefault('offlineSeekDiagnostic', {})['failureClass'] = (
                            str(error) if isinstance(error, RuntimeError) else type(error).__name__)
                check(9.8 * 16000 * 2 <= len(pcm) <= 10.2 * 16000 * 2, 'control_audio_duration')
                check(abs(len(pcm) - len(source_pcm)) <= 0.1 * 16000 * 2, 'control_audio_sample_count')
                check(all(p['packets'] for p in case['fragmentAudioPackets']), 'control_fragment_audio_missing')
                case['result'] = 'passed' if not case['failures'] else 'failed'
            except Exception as error:
                case['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
            finally:
                limit = time.monotonic() + 15
                while encoder_count(server, source) and time.monotonic() < limit:
                    time.sleep(0.05)
                case['ownedFFmpegBeforeTeardown'] = encoder_count(server, source)
                stop.set()
                sampler.join(timeout=5)
                server.terminate()
                try:
                    server.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    server.kill()
                    server.wait(timeout=5)
                log.flush()
                private = bounded_bytes(log_path, 2 * 1024 * 1024, 'control_private_log_bound').decode()
                case.update(safe_seek_phases(private))
                lifecycle = safe_encoder_lifecycle(private)
                case['encoderLifecycle'] = lifecycle
                case['workerBound'] = (resources['samples'] > 0 and resources['samplingErrors'] == 0
                    and resources['peakOwnedFFmpeg'] <= 1 and lifecycle['validSequence']
                    and lifecycle['peakActive'] == 1 and lifecycle['activeAtTeardown'] == 0
                    and case['ownedFFmpegBeforeTeardown'] == 0)
                if name == 'hevc-adopted-preparation' and (lifecycle['starts'] != 1 or lifecycle['ends'] != 1):
                    case['workerBound'] = False
                    case['failures'].append('hevc_adoption_restarted_encoder')
                case['sourceUnchanged'] = before == source_state(source)
                if not case['workerBound'] or not case['sourceUnchanged']:
                    case['result'] = 'failed'
                    case['failures'].append('control_worker_or_source')
