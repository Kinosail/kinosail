#!/usr/bin/env python3
"""Change only stopped disposable E2E state to exercise time without waiting a year."""
import json
import hashlib
from pathlib import Path
import sqlite3
import sys
import time

path, operation, payload = sys.argv[1:]
path = Path(path).resolve()
assert any(part.startswith('kinosail-timeout-synthetic-') for part in path.parts)
assert path.name == 'config'
values = json.loads(payload)
connection = sqlite3.connect(path/'kinosail.db')
read = lambda name: json.loads(connection.execute('SELECT value FROM state WHERE name=?', (name,)).fetchone()[0])
settings, sessions = read('settings.json'), read('sessions.json')
now = int(time.time())
count = 0
if operation == 'snapshot':
    safe = [{'channel': x.get('channel',''), 'createdAt': x.get('createdAt'), 'expiresAt': x.get('expiresAt'), 'inactiveSeconds': x.get('inactiveSeconds')} for x in sessions.values() if x['profileId']==values['viewerID']]
    compatible=dict(settings)
    compatible.pop('publicSessionInactiveHours',None)
    compatible.pop('publicSessionAbsoluteHours',None)
    settings_digest=hashlib.sha256(json.dumps(compatible,sort_keys=True).encode()).hexdigest()
    data_digests={name:hashlib.sha256(bytes(value)).hexdigest() for name,value in connection.execute('SELECT name,value FROM state') if name in ['profiles.json','progress.json','lists.json','collections.json','playlists.json']}
    print(json.dumps({'compatibleSettingsSHA256':settings_digest,'unrelatedDataSHA256':data_digests,'requiredMFA':settings.get('requireMfa'),'sessions':safe,'policies':{k:v for k,v in settings.items() if 'Session' in k or 'session' in k}}))
    connection.close()
    raise SystemExit(0)
if operation == 'deny-write':
    connection.execute("CREATE TRIGGER timeout_test_deny BEFORE UPDATE ON state WHEN NEW.name='settings.json' BEGIN SELECT RAISE(ABORT,'synthetic policy write failure'); END")
    count = 1
elif operation == 'allow-write':
    connection.execute('DROP TRIGGER timeout_test_deny')
    count = 1
elif operation == 'legacy':
    settings.pop('publicSessionInactiveHours', None)
    settings.pop('publicSessionAbsoluteHours', None)
    settings['sessionInactiveHours'], settings['sessionAbsoluteHours'] = values['inactive'], values['absolute']
    for session in sessions.values():
        session.pop('inactiveSeconds', None)
        if session.get('channel') == 'public':
            session.update(createdAt=now, lastSeen=now, expiresAt=now+8*3600, strongAt=now)
else:
    for session in sessions.values():
        owner = session['profileId'] == values.get('ownerID')
        if operation in ('stale-owner', 'restore-owner'):
            if owner:
                count += 1
                session.update(lastSeen=now, strongAt=now-601 if operation == 'stale-owner' else now)
            continue
        if session['profileId'] != values['viewerID'] or session.get('channel', '') != values['channel']:
            continue
        count += 1
        inactive, absolute = values['inactive'], values['absolute']
        session.update(createdAt=now, lastSeen=now, strongAt=now)
        if not values.get('preserveCeilings'): session['expiresAt'] = now+int(absolute*3600)
        if values.get('legacy'):
            session.pop('inactiveSeconds', None)
        elif not values.get('preserveCeilings'):
            session['inactiveSeconds'] = int(inactive*3600)
        margin = values.get('margin', 0)
        mode = values['mode']
        if mode == 'idle': session['lastSeen'] = now-int(inactive*3600)+margin
        if mode == 'absolute': session['createdAt'] = now-int(absolute*3600)+margin
        if mode == 'expiry': session['expiresAt'] = now+margin
        if mode == 'age': session['createdAt'] = session['strongAt'] = now-33*24*3600
assert count > 0 or operation == 'legacy'
for name, value in ([] if operation in ('deny-write', 'allow-write') else [('settings.json', settings), ('sessions.json', sessions)]):
    connection.execute('UPDATE state SET value=? WHERE name=?', (json.dumps(value).encode(), name))
connection.commit()
connection.close()
print(json.dumps({'operation': operation, 'changedSessions': count, 'fixtureUnix': now,
                  'mode': values.get('mode'), 'channel': values.get('channel'), 'margin': values.get('margin')}))
