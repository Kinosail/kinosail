#!/usr/bin/env python3
"""Keep only bounded HLS phase facts from the private synthetic E2E server log."""
import hashlib
import json
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[3]
PHASES = {'queued', 'admission_wait', 'admitted', 'admission_rejected',
          'process_started', 'process_start_failed', 'media_ready'}
NUMBERS = {'encoder_cost': (0, 64), 'capacity': (0, 64), 'segment_start': (0, 99_999),
           'input_seek_ms': (-1, 604_800_000), 'elapsed_ms': (0, 604_800_000),
           'queue_wait_ms': (0, 604_800_000)}
STRINGS = {'mode': {'unknown', 'remux', 'audio-transcode', 'transcode'},
           'work_class': {'unknown', 'playback', 'background'},
           'outcome': {'failed', 'canceled', 'deadline'}}
ALLOWED = {'time', 'level', 'msg', 'request_id', 'phase'} | NUMBERS.keys() | STRINGS.keys()


def project(entry):
    if not isinstance(entry, dict) or not isinstance(entry.get('msg'), str) or entry['msg'] not in {'HLS encode phase', 'HLS transcode started'}:
        return None
    if set(entry) - ALLOWED or not isinstance(entry.get('phase'), str) or entry['phase'] not in PHASES:
        raise ValueError('invalid_phase_schema')
    if not isinstance(entry.get('request_id'), str) or not re.fullmatch(r'[A-Za-z0-9_-]{1,64}', entry['request_id']):
        raise ValueError('invalid_request_correlation')
    for key, (low, high) in NUMBERS.items():
        if type(entry.get(key)) is not int or not low <= entry[key] <= high:
            raise ValueError('invalid_phase_number')
    for key, values in STRINGS.items():
        if key != 'outcome' or key in entry:
            if not isinstance(entry.get(key), str) or entry[key] not in values:
                raise ValueError('invalid_phase_enum')
    value = {key: entry[key] for key in ('phase', *NUMBERS, 'mode', 'work_class')}
    if 'outcome' in entry:
        value['outcome'] = entry['outcome']
    return (entry['request_id'], entry['mode'], entry['segment_start']), value


def evidence(run, revision):
    receipt = json.loads((run / 'receipt.json').read_text())
    if receipt.get('revision') != revision:
        raise ValueError('stale_source_receipt')
    log = run / 'server.log'
    if log.stat().st_size > 20 * 1024 * 1024:
        raise ValueError('private_log_exceeds_bound')
    events, groups, counts = [], {}, dict.fromkeys(sorted(PHASES), 0)
    with log.open() as stream:
        for line in stream:
            if len(line) > 16_384:
                continue
            try:
                entry = json.loads(line)
            except json.JSONDecodeError:
                continue
            projected = project(entry)
            if projected is None:
                continue
            if len(events) >= 512:
                raise ValueError('phase_count_exceeds_bound')
            key, value = projected
            events.append(value)
            counts[value['phase']] += 1
            prior = groups.setdefault(key, set())
            required = {'admission_wait': 'queued', 'admitted': 'admission_wait',
                        'admission_rejected': 'admission_wait', 'process_start_failed': 'admitted',
                        'process_started': 'admitted', 'media_ready': 'process_started'}
            if value['phase'] in required and required[value['phase']] not in prior:
                raise ValueError('phase_order_invalid')
            prior.add(value['phase'])
    if not counts['process_started'] or not counts['media_ready']:
        raise ValueError('media_phase_evidence_missing')
    if not any(value['input_seek_ms'] > 0 for value in events if value['phase'] == 'process_started'):
        raise ValueError('resume_seek_evidence_missing')
    return {'revision': revision, 'result': 'passed', 'phaseCounts': counts, 'events': events,
            'command': 'python3 apps/player/scripts/hls-phase-evidence.py',
            'scriptSHA256': hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
            'boundary': 'Same synthetic real-server moving-media journey; server publication readiness is separate from device decoding. No paths, URLs, credentials, request or playback-session identifiers are retained.'}


def main():
    revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    if not re.fullmatch(r'[0-9a-f]{40}', revision):
        raise SystemExit('invalid_source_revision')
    runs = sorted((ROOT / '.verification/startup').glob('*/receipt.json'))
    if not runs:
        raise SystemExit('startup_receipt_missing')
    run = runs[-1].parent
    try:
        result = evidence(run, revision)
    except (OSError, json.JSONDecodeError):
        result = {'revision': revision, 'result': 'failed', 'reason': 'private_evidence_unavailable'}
    except ValueError as failure:
        # Every ValueError above is a fixed local code, never remote input.
        result = {'revision': revision, 'result': 'failed', 'reason': str(failure)}
    (run / 'hls-phases.json').write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps({key: value for key, value in result.items() if key != 'events'}))
    raise SystemExit(0 if result['result'] == 'passed' else 1)


if __name__ == '__main__':
    main()
