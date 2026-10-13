"""Public completed-suffix controls; the real producer remains held after one fragment.

The browser tail journey catches truncation, but cannot prove failed ENDLIST or
warm/restarted admission. This fixture uses the existing real-encoder wrapper.
"""
import json
from pathlib import Path
import re
import subprocess
import sys
import threading
import time
from hls_timeline_fixture import check


def wait_for(predicate, failure, seconds=35):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        if predicate():
            return
        time.sleep(.025)
    raise RuntimeError(failure)


def completed_suffix(api, plan, run, source, receipt, scenario, state, env, log):
    path = plan['compatible'].replace('/index.m3u8', '-o30000/index.m3u8')
    receipt['completedSuffix'] = scenario
    responses, failures = [], []

    def request():
        try:
            responses.append(api.http(path))
        except Exception as error:
            failures.append(type(error).__name__)

    pending = threading.Thread(target=request)
    pending.start()
    try:
        wait_for(lambda: (run / 'held').exists(), 'completed_producer_not_held')
        variants = list((run / 'cache').glob('*/*p/index.m3u8'))
        check(len(variants) == 1, 'completed_one_physical_variant')
        physical = variants[0]
        root = physical.parent.parent
        prefix = physical.read_bytes()
        check(b'#EXT-X-ENDLIST' not in prefix and b'segment-00000.m4s' in prefix, 'completed_real_prefix')
        time.sleep(.25)
        receipt['prefixWithheld'] = pending.is_alive() and not responses and not failures
        receipt['masterAbsentWhileHeld'] = not (root / 'index.m3u8').exists()
        check(receipt['prefixWithheld'] and receipt['masterAbsentWhileHeld'], 'completed_prefix_must_not_publish_master')
        if scenario == 'timeout':
            pending.join(timeout=35)
            receipt['timeoutStatus'] = responses[0][0] if responses else None
            check(not pending.is_alive() and not failures and receipt['timeoutStatus'] == 503, 'completed_timeout_must_reject')
            events = [json.loads(line) for line in (run / 'server-private.log').read_text().splitlines() if line.startswith('{')]
            receipt['timeoutDiagnostic'] = any(e.get('msg') == 'HLS generation rejected' and e.get('outcome') == 'readiness-timeout'
                and e.get('level') == 'WARN' and e.get('request_id') for e in events)
            check(receipt['timeoutDiagnostic'], 'completed_timeout_diagnostic')
        (run / 'release').touch()
        wait_for(lambda: (run / 'encoded').exists(), 'completed_endlist_producer_not_held')
        receipt['genuineEndlistBeforeExit'] = b'#EXT-X-ENDLIST' in physical.read_bytes()
        check(receipt['genuineEndlistBeforeExit'], 'completed_real_endlist_required')
        time.sleep(.25)
        receipt['masterAbsentBeforeProducerExit'] = not (root / 'index.m3u8').exists()
        receipt['endlistStillPending'] = pending.is_alive() if scenario != 'timeout' else True
        check(receipt['masterAbsentBeforeProducerExit'] and receipt['endlistStillPending'], 'endlist_must_not_replace_successful_join')
        (run / 'finish').touch()
        pending.join(timeout=35)
        check(not pending.is_alive() and not failures and len(responses) == 1, 'completed_request_must_settle')
        joined = run / ((run / 'held').read_text() + '.joined')
        wait_for(joined.exists, 'completed_producer_not_joined')
        if scenario == 'producer':
            receipt['failedEndlistStatus'] = responses[0][0]
            receipt['masterAbsentAfterFailure'] = not (root / 'index.m3u8').exists()
            check(receipt['failedEndlistStatus'] == 503 and receipt['masterAbsentAfterFailure'], 'failed_endlist_must_not_publish_master')
            receipt['failedWarmStatus'] = api.http(path)[0]
            check(receipt['failedWarmStatus'] == 503 and not (root / 'index.m3u8').exists(), 'failed_endlist_warm_rejected')
            restart(api, run, state, env, log)
            receipt['completedExpectedAdmissions'] = 3
            receipt['failedRestartStatus'] = api.http(path)[0]
            check(receipt['failedRestartStatus'] == 503 and not (root / 'index.m3u8').exists(), 'failed_endlist_restart_rejected')
            return
        status, master, _ = api.http(path) if scenario == 'timeout' else responses[0]
        check(status == 200 and master.startswith(b'#EXTM3U'), 'completed_master_status')
        rendition = re.findall(rb'^([1-9][0-9]*p/index\.m3u8)$', master, re.M)
        check(len(rendition) == 1, 'completed_master_one_variant')
        variant = path.removesuffix('index.m3u8') + rendition[0].decode()
        status, public, _ = api.http(variant)
        saved = physical.read_bytes()
        expected = saved.replace(b'#EXT-X-PLAYLIST-TYPE:EVENT', b'#EXT-X-PLAYLIST-TYPE:VOD', 1)
        receipt['exactCompletedPlaylist'] = status == 200 and public == expected and b'#EXT-X-ENDLIST' in public
        check(receipt['exactCompletedPlaylist'], 'completed_playlist_must_preserve_actual_cuts')
        data = json.loads((root / '.copied-cuts').read_text())
        check(data['Strategy'] == 'completed', 'explicit_completed_strategy')
        encoded = len(list(run.glob('*.started')))
        receipt['completedExpectedAdmissions'] = encoded
        receipt['warmStable'] = api.http(variant)[:2] == (200, public)
        restart(api, run, state, env, log)
        receipt['restartStable'] = api.http(path)[0] == 200 and api.http(variant)[:2] == (200, public)
        receipt['noWarmEncode'] = len(list(run.glob('*.started'))) == encoded
        check(receipt['warmStable'] and receipt['restartStable'] and receipt['noWarmEncode'], 'completed_cache_stability')
        if scenario in ('missing-plan', 'mismatched-plan'):
            metadata = root / '.copied-cuts'
            if scenario == 'missing-plan':
                metadata.unlink()  # Only this disposable fixture's completion policy.
            else:
                data['Policy'] = 'mismatched-test-policy'
                metadata.write_text(json.dumps(data, separators=(',', ':')))
            restart(api, run, state, env, log)
            receipt['metadataFaultStatuses'] = [api.http(variant)[0], api.http(variant.removesuffix('index.m3u8') + 'segment-00000.m4s')[0]]
            receipt['metadataFaultStartsNoEncoder'] = len(list(run.glob('*.started'))) == encoded
            check(receipt['metadataFaultStatuses'] == [503, 404] and receipt['metadataFaultStartsNoEncoder'], 'completed_metadata_must_reject')
    finally:
        (run / 'release').touch()
        (run / 'finish').touch()
        pending.join(timeout=40)
        original_failed = sys.exc_info()[0] is not None
        def joined():
            admissions = list(run.glob('*.started'))
            return all(p.read_text().isdigit() and (run / (p.read_text() + '.joined')).exists() for p in admissions)
        try:
            wait_for(joined, 'completed_producer_cleanup', seconds=15)
            receipt['completedCleanupVerified'] = not pending.is_alive()
        except RuntimeError:
            receipt['completedCleanupVerified'] = False
        if not original_failed:
            check(receipt['completedCleanupVerified'], 'completed_request_cleanup')


def restart(api, run, state, env, log):
    state[0].terminate()
    state[0].wait(timeout=10)
    state[0] = subprocess.Popen([str(run / 'server')], cwd=Path(__file__).resolve().parents[3], env=env, stdout=log, stderr=log)
    def ready():
        try:
            return api.http('/healthz', authenticated=False)[0] == 200
        except OSError:
            return False
    wait_for(ready, 'completed_restart_not_ready')


def completed_suffix_receipt(api, run, receipt):
    """Final side-effect and diagnostic proof after the owning Server has exited."""
    admissions = list(run.glob('*.started'))
    receipt['completedNoLateEncoder'] = len(admissions) == receipt.get('completedExpectedAdmissions')
    receipt['completedAllProducersJoined'] = 0 < len(admissions) <= 16 and all(
        p.read_text().isdigit() and (run / (p.read_text() + '.joined')).exists() for p in admissions)
    raw = (run / 'server-private.log').read_text()
    events = [json.loads(line) for line in raw.splitlines() if line.startswith('{')]
    failed = receipt.get('completedSuffix') == 'producer'
    message = 'HLS transcode failed' if failed else 'HLS transcode completed'
    outcomes = [e for e in events if e.get('msg') == message]
    receipt['completedOutcomeDiagnostics'] = len(outcomes) == len(admissions) and all(
        e.get('level') == ('ERROR' if failed else 'INFO') and e.get('request_id') for e in outcomes)
    receipt['completedDiagnosticSecretsAbsent'] = all(value not in raw for value in
        (api.token, 'synthetic-timeline-password', 'synthetic-prefix-key') if value)
    receipt['completedFinalProof'] = all(receipt.get(key) for key in ('completedNoLateEncoder',
        'completedAllProducersJoined', 'completedOutcomeDiagnostics', 'completedDiagnosticSecretsAbsent', 'completedCleanupVerified'))
