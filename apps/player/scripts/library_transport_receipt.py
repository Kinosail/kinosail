"""Project fixed owned TCP facts; no raw relay output enters a public receipt."""
import json,os,stat,sys
from pathlib import Path
CODES=('ECONNRESET','ECONNREFUSED','ETIMEDOUT','EPIPE','ENETUNREACH','EHOSTUNREACH','unknown')
COUNTERS=('connections','connected','capacityRejected','connectDeadline','clientClosed','upstreamClosed','clientBytes','upstreamBytes')
def valid(value):
    if type(value) is not dict or set(value)!=set(COUNTERS)|{'schemaVersion','overflow','errors'}: return False
    if type(value['schemaVersion']) is not int or value['schemaVersion']!=1 or type(value['overflow']) is not bool: return False
    if any(type(value[key]) is not int or not 0<=value[key]<=2147483647 for key in COUNTERS): return False
    if any(value[key]>value['connections'] for key in ('connected','clientClosed','upstreamClosed')): return False
    if type(value['errors']) is not dict or set(value['errors'])!={'client','upstream'}: return False
    return all(type(row) is dict and set(row)==set(CODES) and all(type(count) is int and 0<=count<=2147483647 for count in row.values()) for row in value['errors'].values())
def unique(pairs):
    result={}
    for key,value in pairs:
        if key in result: raise ValueError('duplicate relay field')
        result[key]=value
    return result
def read(path):
    unavailable=lambda reason: {'schemaVersion':1,'available':False,'reason':reason}
    try:
        path=Path(path)
        if not path.is_absolute() or len(str(path))>2048: return unavailable('unowned')
        parent=path.parent.lstat()
        if not stat.S_ISDIR(parent.st_mode) or parent.st_uid!=os.getuid(): return unavailable('unowned')
        directory=os.open(path.parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
        try:
            fd=os.open(path.name,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=directory)
            with os.fdopen(fd,'rb') as source:
                owner=os.fstat(source.fileno())
                if not stat.S_ISREG(owner.st_mode) or owner.st_uid!=os.getuid(): return unavailable('unowned')
                if owner.st_size>2048: return unavailable('oversized')
                raw=source.read(2049)
        finally: os.close(directory)
        if len(raw)>2048: return unavailable('oversized')
        lines=raw.decode('utf-8').splitlines()
        if not 1<=len(lines)<=2: return unavailable('invalid')
        value=json.loads(lines[-1],object_pairs_hook=unique,parse_constant=lambda _: (_ for _ in ()).throw(ValueError('number')))
        if type(value) is not dict or set(value)!={'schemaVersion','containerRunning','networkInternal','targetAdmitted','transport'}: return unavailable('invalid')
        if type(value['schemaVersion']) is not int or value['schemaVersion']!=1 or any(value[key] is not True for key in ('containerRunning','networkInternal','targetAdmitted')) or not valid(value['transport']): return unavailable('invalid')
        return {'schemaVersion':1,'available':True,'transport':value['transport']}
    except FileNotFoundError: return unavailable('missing')
    except OSError: return unavailable('unowned')
    except (ValueError,TypeError,UnicodeError): return unavailable('invalid')
def write(source,destination):
    if any(not isinstance(path,(str,Path)) or not 0<len(str(path))<=2048 for path in (source,destination)): raise ValueError('bounded transport receipt paths required')
    source,destination=Path(source),Path(destination)
    if any(not path.is_absolute() or len(str(path))>2048 or '..' in path.parts or path != Path(os.path.normpath(path)) for path in (source,destination)) or source.name!='relay-failure.json' or destination.name!='relay-transport.json': raise ValueError('invalid transport receipt paths')
    directory=os.open(destination.parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:
        if os.fstat(directory).st_uid!=os.getuid(): raise ValueError('unowned transport destination')
        fd=os.open(destination.name,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600,dir_fd=directory)
        with os.fdopen(fd,'w') as output: output.write(json.dumps(read(source))+'\n')
    finally: os.close(directory)
if __name__=='__main__':
    if len(sys.argv)!=3: raise SystemExit(2)
    write(sys.argv[1],sys.argv[2])
