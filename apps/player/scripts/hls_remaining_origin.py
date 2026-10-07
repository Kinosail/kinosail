"""Actual production origin-refill proof; retain separate startup/lifecycle failures."""
import hashlib
import json
import math
import subprocess
from pathlib import Path
from hls_followon_public import bounded_bytes, check
from hls_remaining_installation import unfiltered_installation_control
from hls_remaining_warmup import fixed_windows
from hls_remaining_process import source_snapshot


def origin_source_witness(root):
    check(not subprocess.check_output(['git', 'status', '--porcelain'], cwd=root, timeout=5).strip(), 'origin_worktree_dirty')
    paths = ['apps/player/internal/server/' + name for name in ['hls.go', 'hls_remaining_arguments.go',
        'hls_audio.go', 'hls_presentation.go', 'hls_job.go', 'hls_seek.go', 'hls_copied_startup.go']]
    paths += ['packages/playback/hls_delivery.go', 'packages/playback/hls_recipe.go']
    return {'trackedAndUntrackedWorktreeClean': True,
        'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], cwd=root, timeout=5).decode().strip(),
        'productionSHA256': {name: hashlib.sha256(bounded_bytes(root / name, 65536, 'origin_production_source_bound')).hexdigest()
            for name in paths}}


def origin_selected_audio(plan):
    selected = next(v for v in plan['media']['audio'] if v['Index'] == plan['compatiblePlan'].get('audioIndex', 0))
    return {k: selected[v] for k, v in [('codec', 'Codec'), ('sampleRate', 'SampleRate'), ('channels', 'Channels'),
        ('layout', 'ChannelLayout'), ('index', 'Index'), ('sourceIndex', 'SourceIndex')]}


def origin_refill_witness(directory, source, case):
    raw = bounded_bytes(directory / 'refill-recipe-private.json', 8192, 'origin_actual_argv_bound')
    arguments = json.loads(raw)
    roots = list((directory / 'cache').glob('*-plan-*'))
    check(len(roots) == 1, 'origin_single_generation')
    root = roots[0]
    offset = str(8 - 382976 / 48000)
    expected = ['-hide_banner', '-loglevel', 'error', '-y', '-avoid_negative_ts', 'disabled', '-max_delay', '5000000',
        '-ss', '0.000', '-readrate', '0.75', '-i', str(source), '-map', '0:a:0', '-vn', '-sn', '-dn',
        '-c:a', 'aac', '-ac', '2', '-b:a', '192000', '-output_ts_offset', offset,
        '-bsf:a', 'noise=amount=0:drop=lt(pts\\,382976)', '-f', 'hls', '-hls_time', '2',
        '-hls_playlist_type', 'event', '-hls_segment_type', 'fmp4', '-hls_segment_options',
        'movflags=+frag_discont+skip_sidx', '-hls_flags', 'temp_file', '-hls_fmp4_init_filename', 'init.mp4',
        '-start_number', '4', '-hls_segment_filename', str(root / 'audio/segment-%05d.m4s'),
        str(root / '.seek-4/audio/index.m3u8')]
    check(arguments == expected, 'origin_actual_production_argv')
    selected = case['originSelectedAudio']
    check(selected == {'codec': 'flac', 'sampleRate': 48000, 'channels': 2, 'layout': 'stereo', 'index': 0,
        'sourceIndex': 0}, 'origin_actual_selected_track')
    check(source_snapshot(source)['sha256'] == case['fixture']['sha256'], 'origin_actual_source')
    case['installationCertificate'] = {'result': 'observed-production-origin-options-with-test-only-pacing',
        'installedArgvSHA256': hashlib.sha256(raw).hexdigest(), 'outputOffsetSeconds': float(offset),
        'actualSourceSeekSeconds': 0, 'logicalRefillCutSeconds': 8, 'selectedAudio': selected,
        'testOnlyReadrate': 0.75, 'originOptionsTransformedByHarness': False}
    # The existing raw control consumes observed argv, without changing the
    # production worker or its retained artifacts. Only that later replay drops
    # the packet filter so all raw encoded packets remain independently visible.
    with (directory / 'installed-recipe-private.json').open('xb') as output:
        output.write(raw)


def origin_cases(run, journey, directory, receipt, deadline):
    generated = "aevalsrc='0.1*sin(2*PI*(440*t+20*t*t))+0.4*between(t,7.995,8.005)|0.1*sin(2*PI*(670*t+31*t*t))+0.35*between(t,8.002,8.012)':s=48000:d=10"
    source = directory / 'independent-stereo.flac'
    run(['ffmpeg', '-nostdin', '-v', 'error', '-f', 'lavfi', '-i', generated, '-c:a', 'flac', str(source)], 60)
    probe = json.loads(run(['ffprobe', '-v', 'error', '-select_streams', 'a:0', '-read_intervals', '%+#1',
        '-show_packets', '-show_entries', 'format=duration,start_time:stream=codec_name,sample_rate,channels,channel_layout,start_time,time_base:packet=pts,dts',
        '-of', 'json', str(source)]))
    stream = probe['streams'][0]
    check(len(probe['streams']) == 1 and stream['codec_name'] == 'flac' and stream['sample_rate'] == '48000'
        and stream['channels'] == 2 and stream['channel_layout'] == 'stereo' and stream['time_base'] == '1/48000'
        and float(stream['start_time']) == float(probe['format']['start_time']) == 0
        and probe['packets'] == [{'pts': 0, 'dts': 0}], 'origin_independent_source_start')
    metadata = {'probedDurationSeconds': float(probe['format']['duration']), 'markerNear8Seconds': True,
        'fixtureFilter': generated, 'independentSourceProbe': probe}
    for name, pace in [('origin-refill', 0.75), ('origin-control', 0.9)]:
        journey(name, source, metadata, pacing=pace)
    result = {'result': 'unqualified', 'qualificationFailures': [],
        'boundary': 'Integrated production arguments on fixed rawFLAC source; preparation/lifecycle/all50 separate.'}
    receipt['originProductionQualification'] = result
    try:
        candidate, control = receipt['cases']
        check(control['result'] == 'passed' and candidate['failures'] ==
            ['audio_preparation_not_ready', 'audio_required_new_encoder'], 'origin_unchanged_failure_boundary')
        for case, count in [(candidate, 2), (control, 1)]:
            joined = case['ownedProcessJoin']
            check(not case.get('failureClass') and not case.get('diagnosticFailureClass') and not case.get('originWitnessFailure')
                and case['workerBound'] and case['sourceUnchanged'] and not case['cleanupFailures']
                and joined['confirmedZeroSamples'] == 2 and joined['remainingOwnedPIDs'] == []
                and not joined['forcedOwnedGroupStop'] and not joined['qualificationFailures']
                and case['encoderLifecycle']['starts'] == case['encoderLifecycle']['ends'] == count,
                'origin_source_and_owned_workers')
            starts = [(v['segment_start'], v['input_seek_ms']) for v in case['encoderStarts']]
            check(starts == ([(0, 0), (4, 8000)] if case is candidate else [(0, 0)]), 'origin_exact_logical_worker_sequence')
        before, after = candidate['physicalBeforeFirstGET'], candidate['physicalAfterPublicDelivery']
        check(before['segments'] == [f'segment-{n:05d}.m4s' for n in range(4)] and
            (before['generationInode'], before['generationDevice']) == (after['generationInode'], after['generationDevice']),
            'origin_prepared_prefix_generation')
        retained = {v['name']: v for v in after['assets']}
        check(all(retained.get(v['name']) == v for v in before['assets'] if v['name'] != 'index.m3u8'),
            'origin_prepared_prefix_init_bytes_and_file_identity')
        result['preparedPrefixAndInitializationUnchanged'] = True
        check(candidate['installationCertificate']['result'] == 'observed-production-origin-options-with-test-only-pacing'
            and candidate['fixture']['sha256'] == control['fixture']['sha256']
            and candidate['initializationSHA256'] == control['initializationSHA256'], 'origin_source_init_pair')
        packets = candidate['joinedPublicPacketPayloads']
        gaps = [float(b['pts_time']) - float(a['pts_time']) - float(a['duration_time']) for a, b in zip(packets, packets[1:])]
        check(len(packets) == 470 and packets == control['joinedPublicPacketPayloads']
            and all(v['pts_time'] == v['dts_time'] and not v.get('side_data_list') and
                math.isfinite(float(v['pts_time'])) and 0 < float(v['duration_time']) <= 1024 / 48000 + 0.000001 for v in packets)
            and all(float(a['pts_time']) < float(b['pts_time']) for a, b in zip(packets, packets[1:]))
            and all(math.isfinite(v) and abs(v) <= 1 / 48000 + 0.000001 for v in gaps), 'origin_complete_packet_clock')
        native = candidate['nativePCMQualification']['public']
        check(native['frames'] == 470 and native['samples'] == 481280 and native['completeEOFAccounted']
            and native['decodedClockOrderValid'] and candidate['fullEOFNativeSamples']['publicSHA256'] ==
            control['fullEOFNativeSamples']['publicSHA256'], 'origin_full_native_pcm_identity')
        unfiltered_installation_control(run, directory / 'origin-refill', candidate, control, deadline)
        reference = bounded_bytes(directory / 'origin-refill/Fixture.flac.pcm', 2 << 20, 'origin_reference_bound')
        reference_sha = hashlib.sha256(reference).hexdigest()
        check(len(reference) == 480000 * 4 and all(case['fullEOFNativeSamples']['source'] == 480000
            and case['fullEOFNativeSamples']['sourceSHA256'] == reference_sha
            and case['fullEOFNativeSamples']['channels'] == 2 and case['fullEOFNativeSamples']['sampleRate'] == 48000
            and case['nativePCMQualification']['source']['completeEOFAccounted']
            for case in [candidate, control]), 'origin_independent_full_reference_binding')
        result['fixedWindowReference'] = {'sha256': reference_sha, 'bytes': len(reference), 'nativeSourceSamples': 480000}
        channel = lambda n: b''.join(reference[v + n:v + n + 2] for v in range(0, len(reference), 4))
        check(hashlib.sha256(channel(0)).digest() != hashlib.sha256(channel(2)).digest(), 'origin_independent_channels')
        result['fixedWindows'] = {label: fixed_windows(reference, bounded_bytes(
            directory / ('origin-' + label) / 'public.mp4.pcm', 2 << 20, 'origin_public_pcm_bound'))
            for label in ['refill', 'control']}
        result['fixedContentComparison'] = []
        for a, b in zip(result['fixedWindows']['refill'], result['fixedWindows']['control']):
            check(a['complete'] and b['complete'] and a['referenceSHA256'] == b['referenceSHA256'], 'origin_full_reference_window')
            for x, y in zip(a['channels'], b['channels']):
                rms, corr = x['normalizedRMSError'] - y['normalizedRMSError'], x['correlationAtDeclaredPhase'] - y['correlationAtDeclaredPhase']
                result['fixedContentComparison'].append({'label': a['label'], 'channel': x['channel'], 'rmsDelta': rms, 'correlationDelta': corr})
                check(max(x['normalizedRMSError'], y['normalizedRMSError']) <= 0.05 and
                    min(x['correlationAtDeclaredPhase'], y['correlationAtDeclaredPhase']) >= 0.998 and rms <= 0.005 and corr >= -0.001,
                    'origin_unchanged_content_criteria')
        check(candidate['installationColdReopen']['result'] == 'qualified', 'origin_public_cold_reopens')
        result.update(result='qualified-fixed-production', exactPublicPacketRows=True, exactWholeNativePCM=True,
            maximumPacketGapSeconds=max([0] + gaps), maximumPacketOverlapSeconds=max([0] + [-v for v in gaps]),
            actualSourceSeekSeconds=0, logicalRefillCutSeconds=8)
    except (RuntimeError, OSError, ValueError, KeyError) as error:
        result['qualificationFailures'].append(str(error) if isinstance(error, RuntimeError) else type(error).__name__)
