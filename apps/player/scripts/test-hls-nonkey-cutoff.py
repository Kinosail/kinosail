#!/usr/bin/env python3
"""Real hosted SIGTERM/internal-deadline cleanup receipts for disposable groups."""
import json
import os
from pathlib import Path
import subprocess
import sys
import threading
import time
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_remaining_process import finish_processes
from hls_followon_public import check

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-nonkey-cutoff'
RUN.mkdir(parents=True, exist_ok=True)
mode = sys.argv[1] if len(sys.argv) > 1 else 'parent'


def owned(kind):
    facts = {'result': 'failed', 'cutoff': kind, 'failureClass': None}
    guard = DiagnosticDeadline(0.25 if kind == 'internal' else 20)
    stop, server = threading.Event(), None
    guard.__enter__()
    try:
        server = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(20)'],
                                  start_new_session=True)
        facts['ownedGroup'] = server.pid
        while True:
            guard.check()
            time.sleep(0.02)
    except RuntimeError as error:
        facts['failureClass'] = str(error)
    finally:
        with guard.cleanup():
            if server is not None:
                facts.update(finish_processes(server, Path('synthetic-cutoff'), stop, None))
            facts['handledTerminationSignals'] = guard.signals
            (RUN / (kind + '.json')).write_text(json.dumps(facts, separators=(',', ':')) + '\n')
        guard.__exit__()
    raise SystemExit(1)


if mode in ['internal', 'term']:
    owned(mode)

receipt = {'result': 'failed', 'revision': subprocess.check_output(
    ['git', 'rev-parse', 'HEAD'], text=True).strip(), 'cases': [],
    'boundary': 'Disposable Python owned-group cutoff regression; no media or production acceptance'}
try:
    for kind in ['internal', 'term']:
        command = [sys.executable, str(Path(__file__)), kind]
        if kind == 'term':
            command = ['timeout', '--kill-after=30s', '1s', *command]
        case = {'cutoff': kind, 'result': 'in-flight'}
        receipt['cases'].append(case)
        before = time.monotonic()
        process = subprocess.run(command, capture_output=True, timeout=35)
        path = RUN / (kind + '.json')
        with path.open('rb') as stream:
            data = stream.read(65537)
        check(0 < len(data) <= 65536, 'cutoff_receipt_bound')
        case.update(json.loads(data))
        case.update(exitCode=process.returncode, elapsedSeconds=time.monotonic() - before)
        join = case['ownedProcessJoin']
        check(process.returncode == (124 if kind == 'term' else 1), 'cutoff_expected_exit')
        check(case['failureClass'] == 'bounded_diagnostic_deadline', 'cutoff_explicit_failure')
        check(case['handledTerminationSignals'] == (1 if kind == 'term' else 0), 'cutoff_handler')
        check(join['confirmedZeroSamples'] == 2 and join['remainingOwnedPIDs'] == []
              and not join['forcedOwnedGroupStop'] and not join['qualificationFailures']
              and not case['cleanupFailures'] and case['elapsedSeconds'] < 10, 'cutoff_unforced_join')
        case['result'] = 'expected-failure-preserved-and-joined'
    receipt['result'] = 'passed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    (RUN / 'receipt.json').write_text(json.dumps(receipt, separators=(',', ':')) + '\n')
    print(json.dumps(receipt, separators=(',', ':')))
raise SystemExit(0 if receipt['result'] == 'passed' else 1)
