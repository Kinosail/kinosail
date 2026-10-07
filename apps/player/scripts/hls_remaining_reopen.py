"""Bounded read-through cold reopen of the disposable corrected AAC cache."""
import shutil
import subprocess
import threading
import time
from hls_followon_public import bounded_bytes, check, sample_resources
from hls_remaining_audio import audio_output
from hls_remaining_process import asset_snapshot, finish_processes, physical, source_snapshot
from hls_timeline_packets import safe_encoder_lifecycle


def cold_reopen(run, binary, directory, source, before, api, hls, environment, deadline, case):
    result = {'result': 'unqualified', 'rounds': [], 'qualificationFailures': [],
        'boundary': 'Two fresh disposable Server processes, same corrected cache and in-memory synthetic auth.'}
    case['installationColdReopen'] = result
    roots = list((directory / 'cache').glob('*-plan-*'))
    check(len(roots) == 1, 'installation_reopen_generation')
    root, identity = roots[0], roots[0].stat()
    names = ['index.m3u8', 'init.mp4', *[v['segment'] for v in case['publicPackets']]]
    snapshot = lambda: [asset_snapshot(root / 'audio' / n, 8 << 20)[1] for n in names]
    original = snapshot()
    for number in [1, 2]:
        row = {'round': number, 'failures': [], 'fixture': case['fixture'], 'planDurationSeconds': 10}
        result['rounds'].append(row)
        stage = directory / ('cold-reopen-' + str(number))
        stage.mkdir()
        server, sampler, stop = None, None, threading.Event()
        resources = {'samples': 0, 'peakOwnedFFmpeg': 0, 'samplingErrors': 0}
        row['resources'] = resources
        wrapper = stage / 'paced-ffmpeg'
        code = bounded_bytes(directory / 'paced-ffmpeg', 8192, 'installation_wrapper_bound').decode()
        # Keep every original private argv/certificate/invocation file immutable.
        for name in ['pacing-invocations.jsonl', 'refill-recipe-private.json']:
            code = code.replace(repr(str(directory / name)), repr(str(stage / name)))
        wrapper.write_text(code)
        wrapper.chmod(0o700)
        env = dict(environment, KINOSAIL_FFMPEG=str(wrapper), KINOSAIL_INSTALLATION_RECEIPT_DIR=str(stage))
        log_path = stage / 'server.log'
        try:
            check(deadline - time.monotonic() > 45, 'installation_reopen_deadline')
            with log_path.open('w') as log:
                try:
                    server = subprocess.Popen([str(binary)], env=env, stdout=log, stderr=log, start_new_session=True)
                    sampler = threading.Thread(target=sample_resources, args=(server, source, stop, resources), daemon=True)
                    sampler.start()
                    ready = time.monotonic() + 30
                    while True:
                        try:
                            if api.http('/healthz', authenticated=False)[0] == 200:
                                break
                        except OSError:
                            pass
                        check(time.monotonic() < ready, 'installation_reopen_health')
                        time.sleep(0.1)
                    check(api.http('/api/v1/me')[0] == 200, 'installation_reopen_synthetic_auth')
                    row['physicalBeforeFirstGET'] = physical(directory / 'cache', hls.split('/')[2])
                    shutil.copyfile(directory / 'physical.mp4', stage / 'physical.mp4')
                    audio_output(api, hls, stage, row, source, run, deadline)
                    row['physicalAfterPublicDelivery'] = physical(directory / 'cache', hls.split('/')[2])
                finally:
                    if server is not None:
                        row.update(finish_processes(server, source, stop, sampler))
            private = bounded_bytes(log_path, 2 << 20, 'installation_reopen_log_bound').decode()
            row['encoderLifecycle'] = safe_encoder_lifecycle(private)
            row['sourceUnchanged'] = source_snapshot(source) == before
            row['assetsUnchanged'] = snapshot() == original
            current = root.stat()
            row['generationUnchanged'] = (identity.st_dev, identity.st_ino) == (current.st_dev, current.st_ino)
            row['exactPublicPacketRowsMatch'] = row['joinedPublicPacketPayloads'] == case['joinedPublicPacketPayloads']
            row['exactPublicPCMSHA256Match'] = row['fullEOFNativeSamples']['publicSHA256'] == case['fullEOFNativeSamples']['publicSHA256']
            joined = row['ownedProcessJoin']
            check(row['sourceUnchanged'] and row['assetsUnchanged'] and row['generationUnchanged'] and
                row['exactPublicPacketRowsMatch'] and row['exactPublicPCMSHA256Match'] and not row['failures'] and
                resources['samples'] > 0 and resources['peakOwnedFFmpeg'] == 0 and not resources['samplingErrors'] and
                row['encoderLifecycle']['starts'] == row['encoderLifecycle']['ends'] == 0 and
                not row['ownedFFmpegBeforeTeardown'] and not row['cleanupFailures'] and
                joined['confirmedZeroSamples'] == 2 and not joined['forcedOwnedGroupStop'] and not joined['qualificationFailures'],
                'installation_reopen_cache_or_process_changed')
            row['result'] = 'qualified'
        except (RuntimeError, OSError, subprocess.SubprocessError, ValueError, KeyError) as error:
            row['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
            result['qualificationFailures'].append(row['failureClass'])
            return
    result['result'] = 'qualified'
