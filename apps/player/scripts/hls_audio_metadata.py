"""Reject forged audio-clock fields through the real missing-fragment endpoint."""
import copy
import hashlib
import json
import re

from hls_followon_public import check, encoder_count
from hls_timeline_http import sha, source_state


def reject_audio_metadata(api, base, root, rendition, server, source, log_path):
    timeline_path, certificate_path = root / '.copy-timeline', root / '.copy-clock'
    original = timeline_path.read_bytes(), certificate_path.read_bytes()
    timeline, certificate = (json.loads(data) for data in original)
    check(timeline.get('AudioSource') and timeline.get('AudioGrid'), 'metadata_requires_certified_audio')
    target = sorted(rendition.glob('segment-*.m4s'))[-1]
    fragment = target.read_bytes()
    baseline = {str(p.relative_to(root)): sha(p) for p in root.rglob('*') if p.is_file()}
    source_before = source_state(source)
    cases = [('AudioSource', 'Rate', 0), ('AudioSource', 'Rate', 2**63 - 1),
             ('AudioSource', 'Quantum', 2048), ('AudioSource', 'Priming', -1),
             ('AudioSource', 'Priming', 1536), ('AudioSource', 'End', timeline['End'] + 2),
             ('AudioGrid', 'Rate', 7350), ('AudioGrid', 'Phase', -1), ('AudioGrid', 'Phase', 1024),
             ('AudioGrid', 'Phase', (timeline['AudioGrid']['Phase'] + 512) % 1024)]
    rows = []
    try:
        for section, field, value in cases:
            damaged = copy.deepcopy(timeline)
            damaged[section][field] = value
            raw = json.dumps(damaged, separators=(',', ':')).encode()
            binding = copy.deepcopy(certificate)
            # Rebind the hash to exercise semantic validation independently of
            # an ordinary checksum mismatch. These are owned fixture files.
            binding['timeline'] = list(hashlib.sha256(raw).digest())
            timeline_path.write_bytes(raw)
            certificate_path.write_text(json.dumps(binding))
            target.unlink()
            position = log_path.stat().st_size
            status, _, headers = api.http(base + target.name)
            request_id = next((v for k, v in headers.items() if k.lower() == 'x-request-id'), '')
            check(re.fullmatch(r'[a-zA-Z0-9_-]{1,80}', request_id), 'metadata_request_id')
            check(status == 404 and not target.exists() and encoder_count(server, source) == 0, 'invalid_audio_metadata_encoded')
            with log_path.open('rb') as log:
                log.seek(position)
                private = log.read(64 << 10).decode()
            records = []
            for line in private.splitlines():
                try:
                    entry = json.loads(line)
                except ValueError:
                    continue
                if entry.get('msg') == 'HLS segment preparation failed' and entry.get('request_id') == request_id:
                    check(entry.get('level') == 'WARN' and len(line) <= 2048, 'metadata_diagnostic_level_or_bound')
                    check(api.token not in line and str(source) not in line and 'synthetic-splice-key' not in line,
                          'metadata_diagnostic_secret')
                    records.append({'level': entry['level'], 'requestID': request_id, 'message': entry['msg']})
            check(len(records) == 1, 'metadata_diagnostic_count')
            timeline_path.write_bytes(original[0])
            certificate_path.write_bytes(original[1])
            target.write_bytes(fragment)
            current = {str(p.relative_to(root)): sha(p) for p in root.rglob('*') if p.is_file()}
            check(current == baseline and source_state(source) == source_before, 'metadata_rejection_changed_assets')
            rows.append({'section': section, 'field': field, 'value': value, 'status': status,
                         'encoderCount': 0, 'assetsUnchanged': True, 'diagnostics': records, 'passed': True})
    finally:
        timeline_path.write_bytes(original[0])
        certificate_path.write_bytes(original[1])
        target.write_bytes(fragment)
    return rows
