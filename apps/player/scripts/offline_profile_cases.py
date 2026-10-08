"""Exact offline identities and a measured OPFS worker capability disposition."""
import base64
import hashlib
import json

PROJECTS = ('chromium', 'firefox', 'webkit')
CASES = (
    ('test-instance-large-offline-a.spec.ts', 'large offline transfers › offline download stores and verifies every transfer chunk'),
    ('test-instance-large-offline-a.spec.ts', 'large offline transfers › offline download rejects a mismatched manifest before storage changes'),
    ('test-instance-large-offline-b.spec.ts', 'large offline transfers › offline resume does not count an orphaned OPFS write twice against quota'),
    ('test-instance-large-offline-b.spec.ts', 'large offline transfers › offline resume accounts for replaced IndexedDB chunks near quota'),
    ('test-instance-large-offline-c.spec.ts', 'large offline transfers › offline playback cleanup removes an unchanged copy with a missing later chunk'),
    ('test-instance-large-offline-c.spec.ts', 'large offline transfers › explicit offline removal cancels a transfer across tabs without letting an old source delete its replacement'),
    ('test-instance-offline.spec.ts', 'two browser clients synchronize and reconnect in one Watch Together room'),
    ('test-instance-offline.spec.ts', 'offline download stops before transfer when device storage is full'),
    ('test-instance-offline.spec.ts', 'offline removal binds immediately, blocks repeat submits, and resumes live updates after BFCache'),
    ('test-instance-offline.spec.ts', 'offline surfaces replace pending text when IndexedDB initialization fails'),
    ('test-instance-offline.spec.ts', 'offline writes fail closed before local changes without Web Locks'),
    ('test-instance-production.spec.ts', 'fresh installation controls the populated app and serves its complete shell offline'),
    ('test-instance-production.spec.ts', 'populated library views stay accessible at desktop and phone sizes'),
    ('test-instance-production.spec.ts', 'saved video Compatibility keeps real music and audiobook playback on their pages'),
    ('test-instance-production.spec.ts', 'authenticated video reports its state to Home Assistant'),
    ('test-instance-production.spec.ts', 'a real offline download plays and seeks after the network disconnects'),
    ('test-instance-production.spec.ts', 'video and music open the receiver picker and restore focus'),
)
OPFS_CASE = ('test-instance-large-offline-b.spec.ts', 'large offline transfers › offline resume does not count an orphaned OPFS write twice against quota')
OPFS_REASON = 'This engine lacks the OPFS sync writer; IndexedDB quota resume is covered separately.'


def selected_cases(project):
    if project not in PROJECTS:
        raise ValueError('fixed offline project required')
    return CASES


def capability(value):
    """No engine-name inference: require the actual case's bounded inline witness."""
    from library_profile_admission import unique_object, invalid_number, finite_number
    pending = [(suite, (), 0) for suite in value['suites']]
    found, visited = None, 0
    while pending:
        suite, parents, depth = pending.pop()
        visited += 1
        if depth > 8 or visited > 64 or not isinstance(suite, dict):
            raise ValueError('bounded capability suite required')
        title = suite['title']
        if not isinstance(title, str) or not 1 <= len(title) <= 240:
            raise ValueError('bounded capability context required')
        context = parents if depth == 0 else (*parents, title)
        children, specs = suite.get('suites', []), suite.get('specs', [])
        if not isinstance(children, list) or len(children) > 64 or not isinstance(specs, list) or len(specs) > 17:
            raise ValueError('bounded capability cases required')
        pending.extend((child, context, depth+1) for child in children)
        for spec in specs:
            if (spec['file'], ' › '.join((*context, spec['title']))) != OPFS_CASE:
                continue
            if found is not None or len(spec['tests']) != 1 or len(spec['tests'][0]['results']) != 1:
                raise ValueError('one actual OPFS capability result required')
            attachments = spec['tests'][0]['results'][0]['attachments']
            if not isinstance(attachments, list) or len(attachments) > 16:
                raise ValueError('bounded capability attachments required')
            matches = [a for a in attachments if a.get('name') == 'offline-opfs-capability']
            if len(matches) != 1:
                raise ValueError('one named OPFS capability attachment required')
            attachment = matches[0]
            if set(attachment) != {'name', 'contentType', 'body'} or attachment['contentType'] != 'application/json':
                raise ValueError('inline JSON capability required')
            body = attachment['body']
            if not isinstance(body, str) or not 1 <= len(body) <= 2048:
                raise ValueError('bounded capability body required')
            raw = base64.b64decode(body, validate=True)
            if base64.b64encode(raw).decode() != body:
                raise ValueError('canonical capability encoding required')
            record = json.loads(raw.decode('utf-8'), object_pairs_hook=unique_object, parse_constant=invalid_number, parse_float=finite_number)
            if (not isinstance(record, dict) or set(record) != {'schemaVersion', 'writer', 'supported'}
                    or type(record['schemaVersion']) is not int or record['schemaVersion'] != 1
                    or record['writer'] != 'opfs-sync-worker' or type(record['supported']) is not bool):
                raise ValueError('closed worker capability required')
            found = record['supported'], hashlib.sha256(raw).hexdigest()
    if found is None:
        raise ValueError('actual OPFS capability required')
    return found
