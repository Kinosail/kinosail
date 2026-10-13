"""Bounded process and completion observations for cold-suffix E2E evidence."""
import json
import os

from hls_followon_public import check


def count(directory, name):
    path = directory / name
    return len(path.read_text().splitlines()) if path.exists() else 0


def settled(directory, name='wrapper-pids'):
    path = directory / name
    for pid in path.read_text().splitlines() if path.exists() else []:
        try:
            os.kill(int(pid), 0)
            return False
        except ProcessLookupError:
            pass
    return True


def first_encoder_completed(directory):
    for line in (directory / 'server-private.log').read_text().splitlines():
        try:
            row = json.loads(line)
        except json.JSONDecodeError:
            continue
        if row.get('msg') == 'HLS transcode completed':
            return True
    return False


def replacement_completed(directory, request_id, session, observation):
    check(isinstance(request_id, str) and 0 < len(request_id) <= 128, 'replacement_request_id_missing')
    counts = {'completed': 0, 'failed': 0, 'info': 0}
    raw = (directory / 'server-private.log').read_text()
    check(len(raw) <= 2 * 1024 * 1024, 'private_log_bound')
    for line in raw.splitlines():
        try:
            row = json.loads(line)
        except json.JSONDecodeError:
            continue
        if row.get('request_id') != request_id or row.get('playback_session') != session:
            continue
        if row.get('msg') == 'HLS transcode completed':
            counts['completed'] += 1
            counts['info'] += row.get('level') == 'INFO'
        elif row.get('msg') == 'HLS transcode failed':
            counts['failed'] += 1
    observation['terminalOutcomes'] = counts
    check(counts['failed'] == 0 and counts['completed'] <= 1 and counts['info'] == counts['completed'],
          'replacement_terminal_outcome_invalid')
    return counts['completed'] == 1 and settled(directory, 'encoder-pids')
