#!/usr/bin/env python3
"""Prove early Subtitles VOD cuts match the source keyframes and completed media.

The installation FFmpeg seam paces and temporarily stops the real encoder after
its first published fragment. No output is mocked or rewritten. Completed-file
browser journeys miss speculative segment URLs exposed during this interval.
"""
import argparse
import json
import os
from pathlib import Path
import re
import shlex
import sys
import socket
import subprocess
import time
from hls_copied_prefix_faults import held_fault
from hls_copied_prefix_environment import held_encoder, fault_probe
from hls_completed_suffix import completed_suffix, completed_suffix_receipt
from hls_timeline_fixture import check, fixture
from hls_long_resumed_prefix import long_fixture, long_window, long_window_final, long_window_join, long_window_rejection
from hls_timeline_http import PublicServer, sha, source_state

ROOT = Path(__file__).resolve().parents[3]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--long-resume', nargs='?', const='previous', choices=['previous', 'origin', 'on-key', 'after-key'],
                    help='Hold a long copied window at this source keyframe boundary')
parser.add_argument('--completed-suffix', choices=['success', 'producer', 'timeout', 'missing-plan', 'mismatched-plan'])
parser.add_argument('--native', action='store_true', help='Release the held encoder only after a native WebKit future-fragment request')
parser.add_argument('--plan-fault', choices=['missing', 'malformed', 'policy', 'cuts', 'downgrade'])
parser.add_argument('--held-source-change', action='store_true')
parser.add_argument('--held-fault', choices=['policy', 'producer'])
parser.add_argument('--index-fault', choices=['unsupported', 'timeout'])
args = parser.parse_args()
if (args.long_resume and args.completed_suffix) or ((args.long_resume or args.completed_suffix)
        and (args.native or args.plan_fault or args.held_source_change or args.held_fault or (args.index_fault and args.completed_suffix))):
    parser.error('long resumed prefix and completed suffix controls must run independently')
RUN = ROOT / '.verification/hls-copied-prefix' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip(),
    'command': shlex.join(['python3', 'apps/player/scripts/test-subtitles-copied-prefix.py', *sys.argv[1:]]), 'result': 'failed', 'planFault': args.plan_fault,
    'boundary': 'Authenticated disposable Subtitles Server; real FFmpeg held at a public prefix; synthetic media.'}


class HeldFaultComplete(Exception):
    pass


def wait_for(predicate, failure, seconds=25):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        if predicate():
            return
        time.sleep(.025)
    raise RuntimeError(failure)


def cuts(manifest):
    return [(float(duration), name) for duration, name in re.findall(
        rb'#EXTINF:([0-9.]+),\n(segment-[0-9]{5}\.m4s)', manifest)]


try:
    media = RUN / 'media'
    media.mkdir()
    if args.long_resume:
        source, receipt['fixture'] = long_fixture(media, args.long_resume, receipt)
    else:
        source, receipt['fixture'] = fixture(media, 'Prefix Sample', 24 * 8, '0,4,10,14,21,27,31',
                                             frames=24 * 32, extension='.mp4')
    before = source_state(source)
    build = ['go', 'build', '-p', '2', '-o', str(RUN / 'server'), './apps/subtitles/cmd/kinosail']
    receipt['build'] = build
    with (RUN / 'build-private.log').open('w') as log:
        subprocess.run(build, cwd=ROOT, check=True, stdout=log, stderr=log, timeout=180)
    wrapper = held_encoder(RUN, source, args.completed_suffix, args.long_resume)
    receipt['wrapperSHA256'] = sha(wrapper)
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    api = PublicServer(f'http://localhost:{port}')
    env = dict(os.environ, KINOSAIL_LISTEN=f'127.0.0.1:{port}', KINOSAIL_AUTH_URL=api.url,
        KINOSAIL_TLS_ENABLED='false', KINOSAIL_DATA_DIR=str(RUN / 'config'),
        KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(RUN / 'cache'),
        KINOSAIL_BACKUP_DIR=str(RUN / 'backups'), KINOSAIL_BACKUP_KEY='synthetic-prefix-key',
        KINOSAIL_FFMPEG=str(wrapper))
    fault_probe(RUN, env, args.index_fault)
    with (RUN / 'server-private.log').open('w') as log:
        server = subprocess.Popen([str(RUN / 'server')], cwd=ROOT, env=env, stdout=log, stderr=log)
        try:
            api.authorize()
            item = next(i for i in api.call('/api/v1/library')['items'] if i['title'] == 'Prefix Sample')
            plan = api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264&audioCodecs=aac')
            check(plan['compatiblePlan']['mode'] == 'remux', 'fixture_must_copy')
            if args.long_resume:
                plan['compatible'] = plan['compatible'].replace('/index.m3u8', f"-o{receipt['longResumeOffset']}/index.m3u8")
            if args.completed_suffix:
                state = [server]
                try:
                    completed_suffix(api, plan, RUN, source, receipt, args.completed_suffix, state, env, log)
                finally:
                    server = state[0]
                raise HeldFaultComplete()
            if args.long_resume and args.index_fault:
                long_window_rejection(api, plan['compatible'], RUN, receipt)
                raise HeldFaultComplete()
            status, master, _ = api.http(plan['compatible'])
            receipt['masterStatus'] = status
            check(status == 200, 'master_status')
            rendition = re.findall(rb'^([1-9][0-9]*p/index\.m3u8)$', master, re.M)
            check(len(rendition) == 1, 'one_variant')
            path = plan['compatible'].removesuffix('index.m3u8') + rendition[0].decode()
            wait_for(lambda: (RUN / 'held').exists(), 'producer_not_held')
            physical = next((RUN / 'cache').glob('*/' + rendition[0].decode()))
            prefix = physical.read_bytes()
            check(len(cuts(prefix)) == 1 and abs(cuts(prefix)[0][0] - receipt.get('longExpectedFirstCut', 4)) < .1
                  and b'#EXT-X-ENDLIST' not in prefix, 'held_prefix_shape')
            held_pid = (RUN / 'held').read_text()
            joined = RUN / (held_pid + '.joined')
            receipt['heldProcessState'] = subprocess.check_output(['ps', '-o', 'state=', '-p',
                (RUN / 'held').read_text()], text=True).strip()
            status, public, _ = api.http(path)
            receipt['prefixStatus'] = status
            receipt['prefixCounts'] = {'physical': len(cuts(prefix)), 'public': len(cuts(public))}
            receipt['inventedEnd'] = b'#EXT-X-ENDLIST' in public
            expected = None if args.long_resume else [4, 6, 4, 7, 6, 4, 1]
            receipt['accurateFutureCuts'] = None if expected is None else [round(x, 6) for x, _ in cuts(public)] == expected
            receipt['stableTargetDuration'] = None if expected is None else b'#EXT-X-TARGETDURATION:7\n' in public
            receipt['producerHeld'] = not joined.exists()
            receipt['heldPrefixUnchanged'] = physical.read_bytes() == prefix
            (RUN / 'public-prefix.m3u8').write_bytes(public)
            if args.held_source_change or args.held_fault:
                held_fault(api, path, RUN, source, args.held_fault or 'source', receipt)
                # Rejection can precede encoder exit. Let the fixture wrapper
                # reap its real child before stopping the owning Server.
                wait_for(lambda: joined.exists(), 'fault_producer_not_joined')
                raise HeldFaultComplete()
            if args.native:
                child = dict(os.environ, QA_RESUME_ORIGIN=api.url, QA_RESUME_TOKEN=api.token,
                    QA_RESUME_ITEM=item['id'], QA_RESUME_OUTPUT=str(RUN / 'native.json'),
                    QA_RESUME_ENGINE='webkit-native', QA_RESUME_APP='subtitles', QA_RESUME_SECONDS='12.5',
                    QA_RESUME_EXPECTED_MODE='remux', QA_RESUME_FRAME_RATE='24', QA_RESUME_DURATION='32',
                    QA_RESUME_RELEASE_ENCODER=str(RUN / 'release'))
                with (RUN / 'native-private.log').open('w') as output:
                    browser = subprocess.run(['node', str(ROOT / 'apps/player/scripts/hls-resume-browser.mjs')],
                        cwd=ROOT, env=child, stdout=output, stderr=subprocess.STDOUT, timeout=150)
                receipt['nativePassed'] = browser.returncode == 0
                receipt['nativeFutureFragment'] = (RUN / 'release').read_text() if (RUN / 'release').exists() else None
            (RUN / 'release').touch()
            wait_for(lambda: joined.exists(), 'producer_not_joined', seconds=65 if args.long_resume else 25)
            wait_for(lambda: b'#EXT-X-ENDLIST' in physical.read_bytes(), 'cache_not_complete')
            status, completed, _ = api.http(path)
            receipt['completedCuts'] = [duration for duration, _ in cuts(completed)]
            receipt['completedMatchesCache'] = cuts(completed) == cuts(physical.read_bytes())
            check(status == 200 and b'#EXT-X-ENDLIST' in completed, 'completed_status')
            receipt['completedKeyframeCuts'] = None if expected is None else [round(x, 6) for x in receipt['completedCuts']] == expected
            receipt['immutablePlaylist'] = cuts(public) == cuts(completed)
            # A completed URI must retain its exact VOD map across warm reads
            # and process restart, without starting another encoder.
            encoded = len(list(RUN.glob('*.joined')))
            status, warm, _ = api.http(path)
            receipt['warmStable'] = status == 200 and warm == public
            server.terminate()
            server.wait(timeout=10)
            receipt['firstServerSettled'] = server.poll() is not None
            server = subprocess.Popen([str(RUN / 'server')], cwd=ROOT, env=env, stdout=log, stderr=log)
            def restarted():
                try:
                    return api.http('/healthz', authenticated=False)[0] == 200
                except OSError:
                    return False
            wait_for(restarted, 'restarted_server_not_ready')
            status, restarted_master, _ = api.http(plan['compatible'])
            check(status == 200 and restarted_master.startswith(b'#EXTM3U'), 'restarted_master_status')
            status, restarted_manifest, _ = api.http(path)
            receipt['restartStable'] = status == 200 and restarted_manifest == public
            receipt['noWarmEncode'] = len(list(RUN.glob('*.joined'))) == encoded
            (RUN / 'public-completed.m3u8').write_bytes(completed)
            (RUN / 'public-restarted.m3u8').write_bytes(restarted_manifest)
            if args.plan_fault:
                plan_file = physical.parent.parent / '.copied-cuts'
                original = plan_file.read_bytes() if plan_file.exists() else None
                malformed = {'Version': 1, 'Policy': 'wrong', 'Strategy': 'certified', 'Cuts': [0, 4, 32]}
                if args.plan_fault == 'missing':
                    plan_file.unlink(missing_ok=True)
                elif args.plan_fault == 'malformed':
                    plan_file.write_bytes(b'{')
                else:
                    record = json.loads(original) if original else malformed
                    if args.plan_fault == 'policy': record['Policy'] = 'wrong'
                    elif args.plan_fault == 'downgrade': record.update(Strategy='legacy', Cuts=None)
                    else: record['Cuts'] = [0, 5, 32]
                    plan_file.write_text(json.dumps(record, separators=(',', ':')))
                requests = [path, path.removesuffix('index.m3u8') + 'segment-00000.m4s']
                receipt['faultStatuses'] = [api.http(url)[0] for url in requests]
                receipt['faultStartsNoEncoder'] = len(list(RUN.glob('*.joined'))) == encoded
                if original is not None: plan_file.write_bytes(original)
                check(receipt['faultStatuses'] == [503, 404] and receipt['faultStartsNoEncoder'], 'invalid_plan_must_not_deliver_media')
            elif args.index_fault:
                saved = json.loads((physical.parent.parent / '.copied-cuts').read_text())
                pid = (RUN / 'index-probe-pid').read_text()
                receipt['optionalIndexFault'] = args.index_fault
                receipt['legacyPlan'] = saved['Strategy'] == 'legacy' and saved['Cuts'] is None
                receipt['indexProbeSettled'] = subprocess.run(['ps', '-p', pid], stdout=subprocess.DEVNULL, check=False).returncode != 0
                check(receipt['legacyPlan'] and receipt['indexProbeSettled'] and status == 200, 'optional_index_fallback')
                events = [json.loads(line) for line in (RUN / 'server-private.log').read_text().splitlines() if line.startswith('{')]
                warnings = [event for event in events if event.get('msg') == 'HLS copied index unavailable']
                receipt['fallbackDiagnostics'] = len(warnings) == 1 and warnings[0]['level'] == 'WARN' and bool(warnings[0]['request_id']) and warnings[0]['outcome'] == 'legacy-before-encoding'
                receipt['noFalseLifecycleError'] = not any(event.get('msg') == 'copied probe completion failed' for event in events)
                check(receipt['fallbackDiagnostics'] and receipt['noFalseLifecycleError'], 'optional_index_diagnostics')
            elif args.long_resume:
                long_window(api, path, RUN, source, physical, receipt)
            else:
                check(receipt['prefixStatus'] == 200 and receipt['accurateFutureCuts'] and receipt['stableTargetDuration']
                  and receipt['inventedEnd'] and receipt['immutablePlaylist'] and receipt['completedKeyframeCuts'] and receipt['heldPrefixUnchanged']
                  and receipt['producerHeld'] and receipt['completedMatchesCache'] and receipt['warmStable']
                  and receipt['restartStable'] and receipt['noWarmEncode'] and receipt['firstServerSettled'], 'copied_timeline_must_match_source_keys')
            check(not args.native or receipt['nativePassed'] and receipt['nativeFutureFragment'], 'native_future_resume')
        finally:
            (RUN / 'release').touch()
            if args.long_resume:
                long_window_join(RUN, receipt)
            server.terminate()
            try:
                server.wait(timeout=10)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait()
            receipt.update(sourceUnchanged=source_state(source) == before,
                           serverSettled=server.poll() is not None,
                           encoderJoined=not (RUN / 'held').exists() or (RUN / ((RUN / 'held').read_text()+'.joined')).exists())
            if args.completed_suffix:
                completed_suffix_receipt(api, RUN, receipt)
            if args.long_resume and not args.index_fault:
                long_window_final(api, RUN, receipt, require_success=sys.exc_info()[0] is None)
    check(receipt['sourceUnchanged'] and receipt['serverSettled'] and receipt['encoderJoined'], 'fixture_cleanup')
    receipt['result'] = 'passed'
except HeldFaultComplete:
    receipt['sourceContentUnchanged'] = sha(source) == before['sha256']
    if receipt['serverSettled'] and receipt['encoderJoined'] and receipt['sourceContentUnchanged'] and (args.held_source_change or receipt['sourceUnchanged']) and (not args.completed_suffix or receipt.get('completedFinalProof')):
        receipt['result'] = 'passed'
    else:
        receipt['failureClass'] = 'held_fault_cleanup'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    receipt['artifactsSHA256'] = {p.name: sha(p) for p in RUN.iterdir() if p.is_file() and p.name != 'server'}
    (RUN / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps({'receipt': str(RUN / 'receipt.json'), 'result': receipt['result']}), flush=True)
raise SystemExit(0 if receipt['result'] == 'passed' else 1)
