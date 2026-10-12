"""Public request faults for a held, real Subtitles encoder."""
import json
import os
import signal
import threading
import time
from hls_timeline_fixture import check


def held_fault(api, path, run, source, fault, receipt):
    statuses = []
    request = threading.Thread(target=lambda: statuses.append(
        api.http(path.removesuffix('index.m3u8') + 'segment-00006.m4s')[0]))
    request.start()
    time.sleep(.25)
    check(request.is_alive(), 'future_request_must_wait')
    if fault == 'source':
        stat = source.stat()
        os.utime(source, ns=(stat.st_atime_ns, stat.st_mtime_ns + 1_000_000))
        receipt['intentionalSourceVersionChange'] = True
    elif fault == 'policy':
        settings = api.call('/api/v1/settings')
        quality = 'quality' if settings['transcoder'] == 'speed' else 'speed'
        api.call('/api/v1/settings/transcoder', 'PUT',
                 {'quality': quality, 'accelerator': 'none', 'toneMap': False})
        check(api.call('/api/v1/settings')['transcoder'] == quality, 'policy_not_changed')
    else:
        os.kill(int((run / 'held').read_text()), signal.SIGKILL)
    (run / 'release').touch()
    request.join(timeout=40)
    check(not request.is_alive(), 'fault_request_unsettled')
    receipt.update(heldFault=fault, heldFaultStatuses=statuses)
    check(statuses == [404], 'held_fault_must_not_deliver_media')
    events = [json.loads(line) for line in (run / 'server-private.log').read_text().splitlines()
              if line.startswith('{')]
    message = 'HLS transcode failed' if fault == 'producer' else 'HLS generation rejected'
    failures = [event for event in events if event.get('msg') == message]
    receipt['heldFaultDiagnostics'] = len(failures) == 1 and bool(failures[0].get('request_id')) and (
        failures[0]['level'] == ('ERROR' if fault == 'producer' else 'WARN'))
    receipt['noReplacementEncoder'] = len([event for event in events if event.get('msg') == 'HLS transcode started']) == 1
    check(receipt['heldFaultDiagnostics'] and receipt['noReplacementEncoder'], 'held_fault_ownership_and_diagnostics')
