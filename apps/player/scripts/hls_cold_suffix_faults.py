"""Task-owned executable faults for public cold HLS resource-limit proof."""
import shutil
import sys


def wrappers(directory, source, fault):
    for kind in ['ffprobe', 'ffmpeg']:
        path = directory / (kind + '-wrapper.py')
        path.write_text('#!' + sys.executable + '\n' + f'''
import json,os,subprocess,sys,time
from pathlib import Path
args=sys.argv[1:]
real={shutil.which(kind)!r}
directory=Path({str(directory)!r})
source=Path({str(source)!r})
metadata={kind!r}=='ffprobe' and str(source) in args and '-of' in args and args[args.index('-of')+1]=='json' and '-read_intervals' not in args
if metadata and {fault!r} in ['unsupported-duration','endpoint-tolerance']:
 result=subprocess.run([real]+args,capture_output=True)
 facts=json.loads(result.stdout)
 facts['format']['duration']='604810' if {fault!r}=='unsupported-duration' else '74.460000'
 sys.stdout.write(json.dumps(facts))
 raise SystemExit(result.returncode)
suffix={kind!r}=='ffprobe' and str(source) in args and '-read_intervals' in args and args[args.index('-read_intervals')+1].endswith('%')
encode={kind!r}=='ffmpeg' and str(source) in args and '-f' in args and args[args.index('-f')+1]=='hls'
if encode:
 with (directory/'encoder-starts').open('a') as f:f.write('start\\n')
 with (directory/'encoder-pids').open('a') as f:f.write(str(os.getpid())+'\\n')
clock={kind!r}=='ffprobe' and 'pipe:0' in args and '%+#8' in args
if clock and {fault!r}=='clock-race':
 with (directory/'wrapper-pids').open('a') as f:f.write(str(os.getpid())+'\\n')
 with (directory/'clock-starts').open('a') as f:f.write('start\\n')
 deadline=time.monotonic()+5
 while not (directory/'clock-release').exists() and time.monotonic()<deadline:time.sleep(.01)
cert={kind!r}=='ffmpeg' and str(source) in args and '-f' in args and args[args.index('-f')+1]=='framehash'
if cert and {fault!r} in ['idr-failure','configuration-failure','aggregate-deadline']:
 with (directory/'wrapper-pids').open('a') as f:f.write(str(os.getpid())+'\\n')
 with (directory/'certification-starts').open('a') as f:f.write('start\\n')
 if {fault!r}=='aggregate-deadline':time.sleep(3)
 result=subprocess.run([real]+args,capture_output=True)
 sys.stdout.buffer.write(result.stdout);sys.stdout.buffer.flush()
 selected=args[args.index('-bsf:v')+1]
 fail={fault!r}=='idr-failure' and selected=='filter_units=pass_types=5' or {fault!r}=='configuration-failure' and 'pass_types=7|8' in selected
 raise SystemExit(42 if fail else result.returncode)
if encode and {fault!r}=='producer-failure' and len((directory/'encoder-starts').read_text().splitlines())==1:
 child=subprocess.Popen([real,'-readrate','2']+args,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 cache=Path(args[-1]).parent.parent
 deadline=time.monotonic()+12
 while child.poll() is None and time.monotonic()<deadline:
  if (cache/'.copy-clock').exists() and (cache/'index.m3u8').exists():break
  time.sleep(.01)
 if child.poll() is None:child.kill()
 child.wait(timeout=5)
 (directory/'producer-failed').write_text('failed')
 raise SystemExit(42)
if encode and {fault!r}=='refill-completion' and len((directory/'encoder-starts').read_text().splitlines())==3:
 result=subprocess.run([real]+args)
 (directory/'refill-held').write_text('held')
 time.sleep(3)
 raise SystemExit(result.returncode)
if not suffix:os.execv(real,[real]+args)
with (directory/'wrapper-pids').open('a') as f:f.write(str(os.getpid())+'\\n')
with (directory/'suffix-starts').open('a') as f:f.write('start\\n')
if {fault!r}=='timeout':time.sleep(45)
if {fault!r}=='aggregate-deadline':time.sleep(3)
result=subprocess.run([real]+args,capture_output=True)
output=result.stdout
if {fault!r} in ['packet-limit','byte-limit','key-limit']:
 first=next(row for row in output.decode().splitlines() if 'pts=' in row)
 fields=dict(part.split('=',1) for part in first.split('|') if '=' in part)
 pts=int(fields['pts']);stamp=fields['pts_time']
 sys.stdout.write(first+'\\n')
 if {fault!r}=='key-limit':
  for index in range(1,4100):
   tick=pts+index
   sys.stdout.write(f'pts={{tick}}|pts_time={{tick/12288:.6f}}|dts={{tick}}|duration=1|duration_time=0.000081|flags=K__\\n')
 else:
  padding='|padding='+('a'*65536) if {fault!r}=='byte-limit' else ''
  row=f'pts={{pts}}|pts_time={{stamp}}|duration=512|duration_time=.041667{{padding}}\\n'
  for _ in range(140 if {fault!r}=='byte-limit' else 100020):sys.stdout.write(row)
 sys.stdout.flush()
 time.sleep(45)
 raise SystemExit(0)
if {fault!r} in ['early-bound','non-key']:
 rows=output.decode().splitlines()
 for number,row in enumerate(rows):
  if 'pts=' not in row:continue
  fields=row.split('|')
  if {fault!r}=='early-bound':fields=['pts_time=-0.000001' if f.startswith('pts_time=') else f for f in fields]
  else:fields=['flags=___' if f.startswith('flags=') else f for f in fields]
  rows[number]='|'.join(fields);break
 output=('\\n'.join(rows)+'\\n').encode()
if {fault!r}=='source-change':
 with source.open('ab') as f:f.write(b'qa-source-change')
sys.stdout.buffer.write(output);sys.stdout.buffer.flush()
if {fault!r}=='early-bound':time.sleep(45)
raise SystemExit(42 if {fault!r}=='late-failure' else result.returncode)
''')
        path.chmod(0o700)
