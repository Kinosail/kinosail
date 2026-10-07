"""Closed disposable AAC installation counterfactual; never production eligibility."""
import hashlib
import json
from pathlib import Path
import re
from hls_followon_public import bounded_bytes, check
from hls_remaining_warmup import fixed_windows


def installed_refill(arguments, source, directory, output_directory=None):
    source, directory = Path(source), Path(directory)
    eligibility = json.loads(bounded_bytes(directory / 'installation-eligibility-private.json', 4096, 'installation_eligibility_bound'))
    check(eligibility['audio'] == {'codec': 'flac', 'sampleRate': 48000, 'channels': 2, 'index': 0}
        and hashlib.sha256(bounded_bytes(source, 2 << 20, 'installation_source_bound')).hexdigest() == eligibility['sourceSHA256'],
        'installation_selected_source')
    root = Path(arguments[-1]).parents[2]
    check(root.parent == directory / 'cache' and re.fullmatch(r'[a-f0-9]{16}-plan-a-[a-zA-Z0-9-]+', root.name), 'installation_root')
    expected = ['-hide_banner', '-loglevel', 'error', '-y', '-avoid_negative_ts', 'disabled', '-max_delay', '5000000',
        '-ss', '8.000', '-readrate', '0.75', '-i', str(source), '-map', '0:a:0', '-vn', '-sn', '-dn',
        '-c:a', 'aac', '-ac', '2', '-b:a', '192000', '-output_ts_offset', '8.000', '-f', 'hls', '-hls_time', '2',
        '-hls_playlist_type', 'event', '-hls_segment_type', 'fmp4', '-hls_segment_options', 'movflags=+frag_discont+skip_sidx',
        '-hls_flags', 'temp_file', '-hls_fmp4_init_filename', 'init.mp4', '-start_number', '4', '-hls_segment_filename',
        str(root / 'audio/segment-%05d.m4s'), str(root / '.seek-4/audio/index.m3u8')]
    check(arguments == expected, 'installation_closed_actual_refill')
    changed = list(arguments)
    changed[changed.index('-ss') + 1] = '7.936'
    changed[changed.index('-output_ts_offset') + 1] = str(8 - 2048 / 48000)
    changed[changed.index('-f'):changed.index('-f')] = ['-bsf:a', 'noise=amount=0:drop=lt(pts\\,2048)']
    raw, altered = json.dumps(arguments).encode(), json.dumps(changed).encode()
    check(len(raw) <= 8192 and len(altered) <= 8192, 'installation_argv_bound')
    target = Path(output_directory) if output_directory is not None else directory
    check(target == directory or target.parent == directory and target.name in ['cold-reopen-1', 'cold-reopen-2'], 'installation_private_output')
    (target / 'installed-recipe-private.json').write_bytes(altered)
    (target / 'installation-certificate.json').write_text(json.dumps({'result': 'closed-transformation',
        'sourceSHA256': eligibility['sourceSHA256'], 'selectedAudio': eligibility['audio'],
        'originalArgvSHA256': hashlib.sha256(raw).hexdigest(), 'installedArgvSHA256': hashlib.sha256(altered).hexdigest(),
        'sourceSeekSeconds': 7.936, 'outputOffsetSeconds': 8 - 2048 / 48000,
        'droppedPTSBelowSamples': 2048, 'startNumber': 4, 'onlyChangedOptions': ['ss', 'output_ts_offset', 'bsf:a']}))
    return changed


def installation_cases(run, journey, directory, receipt, deadline):
    generated = "aevalsrc='0.1*sin(2*PI*(440*t+20*t*t))+0.4*between(t,7.995,8.005)|0.1*sin(2*PI*(670*t+31*t*t))+0.35*between(t,8.002,8.012)':s=48000:d=10"
    source = directory / 'independent-stereo.flac'
    run(['ffmpeg', '-nostdin', '-v', 'error', '-f', 'lavfi', '-i', generated, '-c:a', 'flac', str(source)], 60)
    probe = json.loads(run(['ffprobe', '-v', 'error', '-show_entries',
        'format=duration:stream=codec_name,sample_rate,channels', '-of', 'json', str(source)]))
    check(len(probe['streams']) == 1 and probe['streams'][0]['codec_name'] == 'flac' and
        probe['streams'][0]['sample_rate'] == '48000' and probe['streams'][0]['channels'] == 2,
        'installation_independent_probe')
    metadata = {'probedDurationSeconds': float(probe['format']['duration']), 'markerNear8Seconds': True,
        'fixtureFilter': generated, 'independentSourceProbe': probe}
    for name, pace, installed in [('installation-original', 0.75, False), ('installation-candidate', 0.75, True),
            ('installation-control', 0.9, False)]:
        journey(name, source, metadata, pacing=pace, installation=installed)
    result = {'result': 'unqualified', 'qualificationFailures': [], 'boundary':
        'Actual disposable Server plus closed installation transform; no production, arbitrary-cut or audible acceptance.'}
    receipt['aacInstallationCounterfactual'] = result
    try:
        baseline, candidate, control = receipt['cases']
        result['originalFailures'] = baseline['failures']
        result['candidateFailures'] = candidate['failures']
        check(control['result'] == 'passed' and baseline['failures'] ==
            ['audio_preparation_not_ready', 'audio_packet_order', 'audio_required_new_encoder'] and
            candidate['failures'] == ['audio_preparation_not_ready', 'audio_required_new_encoder'], 'installation_unchanged_control_contract')
        for case in [baseline, candidate, control]:
            check(not case.get('failureClass') and not case.get('diagnosticFailureClass') and case['workerBound']
                and case['sourceUnchanged'] and not case['cleanupFailures'] and not case['ownedProcessJoin']['forcedOwnedGroupStop'],
                'installation_owned_source')
        check(len({v['fixture']['sha256'] for v in [baseline, candidate, control]}) == 1 and
            len({v['initializationSHA256'] for v in [baseline, candidate, control]}) == 1, 'installation_source_init_pair')
        prefix = lambda c: [(v['name'], v['sha256']) for v in c['physicalBeforeFirstGET']['assets'] if v['name'] != 'index.m3u8']
        check(prefix(baseline) == prefix(candidate), 'installation_retained_prefix_pair')
        native = candidate['nativePCMQualification']['public']
        check(len(candidate['joinedPublicPacketPayloads']) == native['frames'] == 470 and native['samples'] == 481280
            and native['completeEOFAccounted'] and native['decodedClockOrderValid'], 'installation_full_native_eof')
        reference = bounded_bytes(directory / 'installation-candidate/Fixture.flac.pcm', 2 << 20, 'installation_pcm_bound')
        channel = lambda n: b''.join(reference[v + n:v + n + 2] for v in range(0, len(reference), 4))
        result['sourceChannelSHA256'] = [hashlib.sha256(channel(n)).hexdigest() for n in [0, 2]]
        check(result['sourceChannelSHA256'][0] != result['sourceChannelSHA256'][1], 'installation_independent_channels')
        result['fixedWindows'] = {}
        for label in ['original', 'candidate', 'control']:
            pcm = bounded_bytes(directory / ('installation-' + label) / 'public.mp4.pcm', 2 << 20, 'installation_pcm_bound')
            result['fixedWindows'][label] = fixed_windows(reference, pcm)
        result['fixedContentComparison'] = []
        for a, b in zip(result['fixedWindows']['candidate'], result['fixedWindows']['control']):
            check(a['complete'] and b['complete'] and a['referenceSHA256'] == b['referenceSHA256'], 'installation_complete_reference_window')
            for x, y in zip(a['channels'], b['channels']):
                row = {'label': a['label'], 'channel': x['channel'],
                    'normalizedRMSErrorDelta': x['normalizedRMSError'] - y['normalizedRMSError'],
                    'correlationDeltaAtDeclaredPhase': x['correlationAtDeclaredPhase'] - y['correlationAtDeclaredPhase']}
                result['fixedContentComparison'].append(row)
                check(max(x['normalizedRMSError'], y['normalizedRMSError']) <= 0.05 and
                    min(x['correlationAtDeclaredPhase'], y['correlationAtDeclaredPhase']) >= 0.998 and
                    row['normalizedRMSErrorDelta'] <= 0.005 and row['correlationDeltaAtDeclaredPhase'] >= -0.001,
                    'installation_fixed_content_criteria')
        check(candidate['installationColdReopen']['result'] == 'qualified', 'installation_reopen_unqualified')
        result['result'] = 'qualified-fixed-installation'
    except (RuntimeError, OSError, ValueError, KeyError) as error:
        result['qualificationFailures'].append(str(error) if isinstance(error, RuntimeError) else type(error).__name__)
