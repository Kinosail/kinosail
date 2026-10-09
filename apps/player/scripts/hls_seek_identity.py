"""Fixed coded fixture identity after ordinary lossy compatible conversion."""
import os
import statistics
import subprocess
import time

FRAME_BYTES=240*64

def marker_frame(raw):
    if type(raw) is not bytes or len(raw)!=FRAME_BYTES:raise RuntimeError('coded_frame_shape')
    def bit(x,y):
        value=statistics.median(raw[row*240+column] for row in range(y-1,y+2) for column in range(x-1,x+2))
        if value<=50:return 0
        if value>=200:return 1
        raise RuntimeError('coded_frame_ambiguous')
    if bit(200,24)!=0 or bit(232,24)!=1:raise RuntimeError('coded_frame_guard')
    frame=0
    for index in range(10):
        value=bit(24+16*index,24)
        if bit(24+16*index,56)==value:raise RuntimeError('coded_frame_complement')
        frame|=value<<index
    if frame>=768:raise RuntimeError('coded_frame_outside_source')
    return frame

def qualify_sequence(actual,expected):
    if type(actual) is not list or type(expected) is not list or not 1<=len(actual)<=768 or not 1<=len(expected)<=768:
        raise RuntimeError('coded_sequence_shape')
    if any(type(value) is not int or not 0<=value<768 for value in actual+expected):raise RuntimeError('coded_sequence_shape')
    if actual!=expected:raise RuntimeError('exact_requested_source_sequence')
    return len(actual)

def decode_identity(path,directory):
    # Pixel dimensions are fixed by the fixture. Nearest scaling only restores
    # the marker coordinates; each bit has separated luminance and complement.
    command=['ffmpeg','-nostdin','-v','error','-xerror','-threads','2','-i',str(path),'-an',
        '-vf','scale=640:360:flags=neighbor,format=gray,crop=240:64:0:0',
        '-frames:v','769','-fps_mode','passthrough','-f','rawvideo','pipe:1']
    pixels=directory/'coded-delivery-private.gray';errors=directory/'coded-delivery-private.stderr'
    with pixels.open('xb') as out,errors.open('xb') as err:
        os.chmod(pixels,0o600);os.chmod(errors,0o600)
        child=subprocess.Popen(command,stdout=out,stderr=err)
        deadline=time.monotonic()+60
        try:
            while child.poll() is None:
                if pixels.stat().st_size>768*FRAME_BYTES or errors.stat().st_size>16384 or time.monotonic()>deadline:
                    raise RuntimeError('coded_decode_bound')
                time.sleep(.01)
            if child.returncode!=0 or pixels.stat().st_size>768*FRAME_BYTES or errors.stat().st_size>16384:
                raise RuntimeError('coded_decode_failed')
        finally:
            if child.poll() is None:child.kill()
            child.wait()
    size=pixels.stat().st_size
    if not 0<size<=768*FRAME_BYTES or size%FRAME_BYTES:raise RuntimeError('coded_decode_shape')
    with pixels.open('rb') as stream:return [marker_frame(stream.read(FRAME_BYTES)) for _ in range(size//FRAME_BYTES)]
