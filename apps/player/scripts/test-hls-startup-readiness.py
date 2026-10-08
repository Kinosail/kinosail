#!/usr/bin/env python3
"""Separate public readiness proof; preserve the original strict origin failures."""
import hashlib
import json
from decimal import Decimal
from pathlib import Path
import subprocess
import time
from hls_followon_public import bounded_bytes, check

ROOT = Path(__file__).resolve().parents[3]
before = set((ROOT / '.verification/hls-followon').glob('*/receipt.json'))
started = time.monotonic()
process = subprocess.run(['python3', 'apps/player/scripts/test-hls-remaining.py', '--suite', 'audio-origin'],
    cwd=ROOT, capture_output=True, timeout=600)
created = set((ROOT / '.verification/hls-followon').glob('*/receipt.json')) - before
check(len(created) == 1, 'readiness_single_original_receipt')
path = created.pop()
raw = bounded_bytes(path, 4 << 20, 'readiness_original_receipt_bound')
original = json.loads(raw)
paths = ['apps/player/internal/server/' + name for name in
    ['startup_window.go', 'hls_remaining_startup.go', 'hls_remaining_completion_test.go']]
paths += ['apps/player/scripts/test-hls-startup-readiness.py', 'apps/player/scripts/test-hls-remaining.py',
    'apps/player/scripts/hls_remaining_origin.py', 'apps/player/scripts/hls_followon_public.py',
    'apps/player/scripts/hls_remaining_process.py']
result = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], cwd=ROOT, text=True).strip(),
    'result': 'failed', 'failures': [], 'command': 'python3 apps/player/scripts/test-hls-startup-readiness.py',
    'originalReceiptSHA256': hashlib.sha256(raw).hexdigest(), 'originalResult': original.get('result'),
    'originalExitCode': process.returncode, 'elapsedSeconds': round(time.monotonic() - started, 3),
    'boundary': 'Fixed FLAC48k stereo public preparation readiness and unchanged output; strict one-encoder contract remains failed.',
    'productionMediaOrCacheModified': False,
    'sourceSHA256': {name: hashlib.sha256(bounded_bytes(ROOT / name, 65536, 'readiness_source_bound')).hexdigest()
        for name in paths if (ROOT / name).is_file()}}
try:
    check(original['revision'] == result['revision'] and original['suite'] == 'audio-origin'
        and original['expectedCases'] == len(original['cases']) == 2
        and original['productionSourceWitness']['unchangedAfterPublicProof'], 'readiness_original_source_binding')
    candidate, control = original['cases']
    result['historicalFailureTagsRetained'] = candidate['failures']
    check(process.returncode == 1 and original['result'] == 'failed'
        and candidate['name'] == 'origin-refill' and candidate['failures'] == ['audio_required_new_encoder']
        and control['name'] == 'origin-control' and control['result'] == 'passed', 'readiness_strict_boundary_retained')
    check(original['originProductionQualification']['result'] == 'unqualified'
        and original['originProductionQualification']['qualificationFailures'] == ['origin_unchanged_failure_boundary'],
        'readiness_original_qualification_not_relabelled')
    for case, count in [(candidate, 2), (control, 1)]:
        preparation, joined = case['preparationAttempt'], case['ownedProcessJoin']
        check(preparation['posts'] == 1 and preparation['publicState'] == 'queued'
            and preparation['completionState'] == 'ready' and preparation['ownedFFmpeg'] == 0
            and preparation['joinedSamples'] >= 3, 'readiness_public_preparation_ready')
        check(not any(case.get(name) for name in ['failureClass', 'diagnosticFailureClass', 'originWitnessFailure',
                'cleanupFailureClass', 'cleanupFailures'])
            and case['workerBound'] and case['sourceUnchanged']
            and joined['confirmedZeroSamples'] == 2 and joined['remainingOwnedPIDs'] == []
            and not joined['forcedOwnedGroupStop'] and not joined['qualificationFailures']
            and case['encoderLifecycle']['starts'] == case['encoderLifecycle']['ends'] == count,
            'readiness_owned_worker_and_source')
    prefix = candidate['physicalBeforeFirstGET']
    check(prefix['segments'] == [f'segment-{n:05d}.m4s' for n in range(4)]
        and not prefix['manifest']['endlist'] and prefix['packetCount'] == 375
        and Decimal(prefix['firstPacket']['pts_time']) == 0
        and Decimal(prefix['lastPacket']['pts_time']) + Decimal(prefix['lastPacket']['duration_time']) == 8,
        'readiness_exact_physical_eight_seconds')
    after = candidate['physicalAfterPublicDelivery']
    retained = {v['name']: v for v in after['assets']}
    check((prefix['generationInode'], prefix['generationDevice']) ==
        (after['generationInode'], after['generationDevice'])
        and all(retained.get(v['name']) == v for v in prefix['assets'] if v['name'] != 'index.m3u8'),
        'readiness_prefix_init_and_generation_retained')
    packets = candidate['joinedPublicPacketPayloads']
    check(len(packets) == 470 and packets == control['joinedPublicPacketPayloads'], 'readiness_exact_packet_rows')
    gaps = [Decimal(b['pts_time']) - Decimal(a['pts_time']) - Decimal(a['duration_time'])
        for a, b in zip(packets, packets[1:])]
    check(all(p['pts_time'] == p['dts_time'] and not p.get('side_data_list')
            and Decimal(p['duration_time']).is_finite() and 0 < Decimal(p['duration_time']) <= Decimal('0.021334')
            for p in packets) and all(abs(gap) <= Decimal('0.000001') for gap in gaps),
        'readiness_complete_adjacent_packet_clock')
    check(candidate['fixture']['sha256'] == control['fixture']['sha256']
        and candidate['initializationSHA256'] == control['initializationSHA256']
        and candidate['fullEOFNativeSamples']['publicSHA256'] == control['fullEOFNativeSamples']['publicSHA256'],
        'readiness_source_init_and_whole_pcm_identity')
    for case in [candidate, control]:
        native = case['nativePCMQualification']['public']
        check(native['frames'] == 470 and native['samples'] == 481280
            and native['completeEOFAccounted'] and native['decodedClockOrderValid'],
            'readiness_complete_native_pcm')
    check(candidate['installationColdReopen']['result'] == 'qualified'
        and len(candidate['installationColdReopen']['rounds']) == 2, 'readiness_two_cold_reopens')
    result.update(result='passed', preparationState='ready', physicalPrefixPackets=375,
        exactPublicPackets=470, nativePCMSamples=481280,
        wholePublicPCMSHA256=candidate['fullEOFNativeSamples']['publicSHA256'],
        maximumPacketGapSeconds=str(max([Decimal(0)] + gaps)),
        maximumPacketOverlapSeconds=str(max([Decimal(0)] + [-gap for gap in gaps])),
        coldReopens=2, sequentialEncoderStarts=2, peakOwnedFFmpeg=1)
except (RuntimeError, KeyError, ValueError) as error:
    result['failures'].append(str(error) if isinstance(error, RuntimeError) else type(error).__name__)
target = path.with_name('startup-readiness.json')
target.write_text(json.dumps(result, separators=(',', ':')) + '\n')
with path.with_name('SHA256SUMS').open('a') as output:
    output.write(hashlib.sha256(target.read_bytes()).hexdigest() + '  startup-readiness.json\n')
print(json.dumps(result, separators=(',', ':')))
raise SystemExit(0 if result['result'] == 'passed' else 1)
