"""Fixed coded source and strict independent decoded-frame PTS mapping."""
import json
import math
import re
import hashlib
import os
import subprocess
import time

def proof_enabled(env):
 value=env.get('KINOSAIL_HLS_PRESENTATION_PROOF','0')
 if not isinstance(value,str) or value not in {'0','1'}:
  raise ValueError('invalid presentation proof option')
 if value=='1' and env.get('KINOSAIL_STARTUP_BASELINE')=='1':
  raise ValueError('conflicting presentation baseline')
 return value=='1'

def coded_filter():
 # FFmpeg geq N is the input frame number. The two rows have opposite bits.
 bit='mod(floor(N/pow(2,floor((X-16)/16))),2)'
 first=f'if({bit},235,16)'
 second=f'if({bit},16,235)'
 lum=f'if(between(X,16,175)*between(Y,16,31),{first},if(between(X,16,175)*between(Y,48,63),{second},if(between(X,192,207)*between(Y,16,31),16,if(between(X,224,239)*between(Y,16,31),235,lum(X,Y)))))'
 # Neutral chroma makes the fixed luminance code independently classifiable.
 return f"geq=lum='{lum}':cb=128:cr=128"

def fixture_command(path):
 return ['ffmpeg','-nostdin','-v','error','-f','lavfi','-i','testsrc2=s=640x360:r=24:d=32',
  '-f','lavfi','-i','sine=frequency=440:sample_rate=48000:duration=32',
  '-vf','format=yuv420p,'+coded_filter(),'-c:v','libx264','-threads','2','-preset','veryfast',
  '-crf','18','-pix_fmt','yuv420p','-g','48','-keyint_min','48','-sc_threshold','0',
  '-frames:v','768','-c:a','aac','-ac','2',str(path)]

def build_fixture(media, run):
 path=media/'HLS Presented.mkv'
 command=fixture_command(path)
 subprocess.run(command,check=True,timeout=120,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 probe=['ffprobe','-v','error','-select_streams','v:0','-show_entries',
  'frame=best_effort_timestamp_time:frame_side_data=','-of','json',str(path)]
 # Retain bounded private bytes before parsing. No raw tool error publication.
 output=run/'source-frame-probe.json'; errors=run/'source-frame-probe.stderr'
 with output.open('xb') as out, errors.open('xb') as err:
  os.chmod(output,0o600);os.chmod(errors,0o600)
  child=subprocess.Popen(probe,stdout=out,stderr=err)
  deadline=time.monotonic()+30
  try:
   while child.poll() is None:
    if output.stat().st_size>65536 or errors.stat().st_size>16384 or time.monotonic()>deadline:
     raise RuntimeError('source frame probe exceeded bound')
    time.sleep(.01)
   if child.returncode!=0 or output.stat().st_size>65536 or errors.stat().st_size>16384:
    raise RuntimeError('source frame probe failed')
  finally:
   if child.poll() is None:child.kill()
   child.wait()
 times=source_frames(output.read_bytes())
 facts={'schema':1,'frameRate':24,'frameCount':768,'seekSeconds':12.5,'targetFrame':300,
  'prerollFrame':288,'sourcePTS':times,'command':command,'probe':probe,
  'sourceSHA256':hashlib.sha256(path.read_bytes()).hexdigest(),
  'probeSHA256':hashlib.sha256(output.read_bytes()).hexdigest(),
  'pixelFormat':'yuv420p','marker':'ten binary luminance bits + complement + black/white guards'}
 target=run/'source-frame-map.json'
 target.write_text(json.dumps(facts,separators=(',',':'))+'\n');os.chmod(target,0o600)
 return facts

def source_frames(raw):
 if not isinstance(raw,bytes) or not 1<=len(raw)<=65536:raise ValueError('invalid source frame map')
 def pairs(rows):
  value={}
  for key,item in rows:
   if key in value:raise ValueError('duplicate frame map key')
   value[key]=item
  return value
 try:
  data=json.loads(raw.decode('utf-8','strict'),object_pairs_hook=pairs,
    parse_constant=lambda value:(_ for _ in ()).throw(ValueError('invalid numeric map')))
 except (UnicodeError,json.JSONDecodeError,RecursionError) as error:
  raise ValueError('invalid source frame map') from error
 if not isinstance(data,dict) or set(data)!={'frames'} or not isinstance(data['frames'],list) or len(data['frames'])!=768:
  raise ValueError('invalid source frame map')
 times=[]
 for frame in data['frames']:
  if not isinstance(frame,dict) or set(frame)-{'best_effort_timestamp_time','side_data_list'} or 'best_effort_timestamp_time' not in frame:
   raise ValueError('invalid source frame row')
  side=frame.get('side_data_list',[])
  if not isinstance(side,list) or len(side)>4 or any(item!={} for item in side):
   raise ValueError('unexpected source frame side data')
  value=frame['best_effort_timestamp_time']
  if not isinstance(value,str) or not re.fullmatch(r'(?:0|[1-9][0-9]{0,2})(?:\.[0-9]{1,6})?',value):
   raise ValueError('invalid source frame timestamp')
  time=float(value)
  if not math.isfinite(time) or not 0<=time<33:raise ValueError('invalid source frame timestamp')
  times.append(time)
 if times[0]>.1 or any(abs(value-times[0]-i/24)>.0011 for i,value in enumerate(times)):
  raise ValueError('conflicting source frame clock')
 return times
