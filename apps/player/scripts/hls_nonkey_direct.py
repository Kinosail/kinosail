"""Bounded byte correspondence for the disposable Direct seek control."""
import hashlib
import re


def direct_delivery_matches(network, source):
    if type(source) is not bytes or not 0 < len(source) <= 8 * 1024 * 1024:
        return False
    if not isinstance(network, dict) or any(type(network.get(k)) is not int or network[k] != 0
            for k in ['unexpectedMediaRequests', 'failedMediaResponses']):
        return False
    rows = network.get('directResponses')
    if not isinstance(rows, list) or not 0 < len(rows) <= 32:
        return False
    for row in rows:
        if (not isinstance(row, dict) or set(row) != {'status', 'contentRange', 'bytes', 'sha256'}
                or type(row['status']) is not int or row['status'] not in [200, 206]
                or type(row['bytes']) is not int or not 0 < row['bytes'] <= len(source)
                or not isinstance(row['sha256'], str) or not re.fullmatch('[a-f0-9]{64}', row['sha256'])
                or not isinstance(row['contentRange'], str) or len(row['contentRange']) > 80):
            return False
        data = source
        if row['status'] == 200:
            if row['contentRange']:
                return False
        else:
            span = re.fullmatch(r'bytes ([0-9]{1,8})-([0-9]{1,8})/([0-9]{1,8})', row['contentRange'])
            if not span:
                return False
            first, last, total = map(int, span.groups())
            if total != len(source) or not 0 <= first <= last < total:
                return False
            data = source[first:last + 1]
        if row['bytes'] != len(data) or row['sha256'] != hashlib.sha256(data).hexdigest():
            return False
    return True
