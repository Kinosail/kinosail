"""Existing executable installation seams for the real copied-prefix journey."""
import shutil
from hls_timeline_fixture import check


def held_encoder(run, source, completed_suffix, long_resume):
    real = shutil.which('ffmpeg')
    check(real, 'ffmpeg_missing')
    wrapper = run / 'held-ffmpeg'
    wrapper.write_text('''#!/usr/bin/env python3
import os,signal,subprocess,sys,time
from pathlib import Path
a=sys.argv[1:]
real=''' + repr(real) + '''
source=''' + repr(str(source)) + '''
if '-hls_time' not in a or source not in a: os.execv(real,[real]+a)
a[a.index('-i'):a.index('-i')]=['-readrate','2']
root=Path(__file__).parent
admission=root/(str(os.getpid())+'.started')
admission.write_text('pending')
child=subprocess.Popen([real]+a)
admission.write_text(str(child.pid))
def stop(signum,frame): raise SystemExit(1)
signal.signal(signal.SIGTERM,stop)
try:
    manifest=Path(a[-1])
    limit=time.monotonic()+90
    while child.poll() is None and time.monotonic()<limit:
        if manifest.exists() and b'segment-00000.m4s' in manifest.read_bytes():
            child.send_signal(signal.SIGSTOP)
            (root/'held').write_text(str(child.pid))
            while not (root/'release').exists() and time.monotonic()<limit: time.sleep(.01)
            child.send_signal(signal.SIGCONT)
            break
        time.sleep(.005)
    status=child.wait(timeout=''' + repr(65 if long_resume else 30) + ''')
    if ''' + repr(bool(completed_suffix)) + ''' and status == 0:
        (root/'encoded').touch()
        while not (root/'finish').exists() and time.monotonic()<limit: time.sleep(.01)
        if not (root/'finish').exists(): raise SystemExit(38)
    sys.exit(37 if status == 0 and ''' + repr(completed_suffix == 'producer') + ''' else status)
finally:
    if child.poll() is None:
        child.send_signal(signal.SIGCONT)
        child.terminate()
        try: child.wait(timeout=3)
        except subprocess.TimeoutExpired: child.kill(); child.wait()
    (root/(str(child.pid)+'.joined')).write_text('joined')
''')
    wrapper.chmod(0o700)
    return wrapper


def fault_probe(run, env, fault):
    if fault:
        probe = run / 'fault-ffprobe'
        probe.write_text('#!/usr/bin/env python3\nimport os,sys,time\nfrom pathlib import Path\n' +
            'a=sys.argv[1:]\nif "-show_packets" in a:\n' +
            ' Path(' + repr(str(run / 'index-probe-pid')) + ').write_text(str(os.getpid()))\n' +
            (' time.sleep(30)\n' if fault == 'timeout' else ' print("invalid metadata",end="",flush=True)\n') +
            ' sys.exit(0)\nos.execv(' + repr(shutil.which('ffprobe')) + ',[' + repr(shutil.which('ffprobe')) + ']+a)\n')
        probe.chmod(0o700)
        env['KINOSAIL_FFPROBE'] = str(probe)
