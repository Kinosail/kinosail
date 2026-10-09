"""Bounded source/public raw AAC boundary facts; never infers a discard or presentation map."""
import hashlib
import re
import struct
from hls_followon_public import bounded_bytes
from hls_remaining_nonkey_init import boxes, one, initialization_metadata, version

def fail():
    raise RuntimeError('browser_audio_boundary_shape')

def integer(value,missing=False):
    if missing and value is None:
        return None
    if type(value) is not int or not -(1<<63)<=value<(1<<64):
        fail()
    return value

def decimal(value):
    if value is None:
        return None
    if type(value) is not str or re.fullmatch(r'-?[0-9]{1,20}(?:\.[0-9]{1,12})?',value) is None:
        fail()
    return value

def packet_edge(row):
    digest=row.get('data_hash','')
    if re.fullmatch(r'SHA256:[a-f0-9]{64}',digest) is None:
        fail()
    result={key:integer(row.get(key),True) for key in ['pts','dts','duration']}
    result.update({key:decimal(row.get(key)) for key in ['pts_time','dts_time','duration_time']})
    size=row.get('size')
    if size is not None and (type(size) is not str or re.fullmatch(r'[0-9]{1,12}',size) is None):
        fail()
    flags=row.get('flags')
    if flags is not None and (type(flags) is not str or re.fullmatch(r'[A-Z_]{1,8}',flags) is None):
        fail()
    side=row.get('side_data_list')
    if side is not None and (type(side) is not list or len(side)>8):
        fail()
    skip=[]
    for value in side or []:
        if type(value) is not dict or value.get('side_data_type')!='Skip Samples':
            fail()
        skip.append({key:integer(value.get(key),True) for key in
                     ['skip_samples','discard_padding','skip_reason','discard_reason']})
    result.update(size=size,flags=flags,payloadSHA256=digest,skipSamples=skip,sideDataFieldMissing=side is None)
    return result

def packet_edges(rows):
    if type(rows) is not list or not 0<len(rows)<=4096:
        fail()
    selected=[v for v in rows if v.get('stream_index')==1]
    if not selected:
        fail()
    return {'count':len(selected),'firstThree':[packet_edge(v) for v in selected[:3]],
            'lastThree':[packet_edge(v) for v in selected[-3:]],
            'clockScope':'Edited demux packet clocks; distinct from raw BMFF sample ticks'}

def stream_facts(value):
    stream=value.get('stream',{})
    if stream.get('sample_rate')!='48000' or stream.get('channels')!=2 or stream.get('codec_name')!='aac':
        fail()
    base=stream.get('time_base','')
    if type(base) is not str or re.fullmatch(r'[1-9][0-9]{0,11}/[1-9][0-9]{0,11}',base) is None:
        fail()
    return {'sampleRate':48000,'channels':2,'codec':'aac','timeBase':base,
            'startTime':decimal(stream.get('start_time')),'duration':decimal(stream.get('duration')),
            'decodedSamples':integer(value.get('samples'),True),'completeEOFAccounted':value.get('completeEOFAccounted') is True}

def retained_audio_boundary(observed):
    result={'observed':False,'missingSampleLocationEstablished':False,'decoderEOFQualified':False,
            'scope':'Retained demux and raw public clocks only; no missing-sample explanation'}
    try:
        result['sourcePackets']=packet_edges(observed.get('sourcePacketRows'))
        result['publicPackets']=packet_edges(observed.get('publicPacketRows'))
        pcm=observed.get('nativePCM',{})
        result['sourceStream']=stream_facts(pcm.get('source',{}))
        result['publicStream']=stream_facts(pcm.get('public',{}))
        init=observed.get('initialization',{})
        tracks=[v for v in init.get('tracks',[]) if v.get('handler')=='soun']
        if len(tracks)!=1:
            fail()
        track=tracks[0];identifier=integer(track.get('trackID'));scale=integer(track.get('mediaTimescale'))
        if identifier<=0 or scale<=0:
            fail()
        samples=[]
        fragments=observed.get('physicalFragments',[])
        if not 0<len(fragments)<=32:
            fail()
        for fragment in fragments:
            selected=[v for v in fragment.get('tracks',[]) if v.get('trackID')==identifier]
            if len(selected)!=1:
                fail()
            for row in selected[0].get('samples',[]):
                value={key:integer(row.get(key)) for key in ['pts','dts','duration']}
                if value['duration']<=0 or value['duration']>=(1<<32):
                    fail()
                samples.append(value)
                if len(samples)>4096:
                    fail()
        if not samples:
            fail()
        edits=[]
        for edit in track.get('edits',[]):
            edits.append({key:integer(edit.get(key)) for key in ['duration','mediaTime','rateInteger','rateFraction']})
        if len(edits)>8:
            fail()
        result['publicRaw']={'trackID':identifier,'mediaTimescale':scale,
            'movieTimescale':integer(init.get('movieTimescale')),'edits':edits,
            'sampleCount':len(samples),'firstPhysicalSamples':samples[:3],'lastPhysicalSamples':samples[-3:],
            'rawTicksNeverReinterpreted':True}
        result['observed']=True
    except (RuntimeError,TypeError,ValueError,KeyError,AttributeError,struct.error):
        result['failureClass']='browser_audio_boundary_shape'
    return result

def clock_duration(data):
    current=version(data);offset=12 if current==0 else 20
    shape='>II' if current==0 else '>IQ'
    if len(data)<offset+struct.calcsize(shape):
        fail()
    scale,duration=struct.unpack_from(shape,data,offset)
    if not scale:
        fail()
    return {'timescale':scale,'duration':duration,'version':current}

def source_audio_boundary(data,digest):
    if type(data) is not bytes or not 0<len(data)<=2<<20 or re.fullmatch(r'[a-f0-9]{64}',digest) is None:
        fail()
    if hashlib.sha256(data).hexdigest()!=digest:
        fail()
    values=boxes(memoryview(data));movie=one(values,b'moov');ftyp=one(values,b'ftyp')
    def packed(kind,body):
        return struct.pack('>I4s',len(body)+8,kind)+bytes(body)
    initialization=initialization_metadata(packed(b'ftyp',ftyp)+packed(b'moov',movie))
    tracks=[v for v in initialization['tracks'] if v['handler']=='soun']
    if len(tracks)!=1:
        fail()
    track=tracks[0];movie_boxes=boxes(movie)
    raw_tracks=[]
    for kind,body in movie_boxes:
        if kind!=b'trak':
            continue
        parts=boxes(body);media=boxes(one(parts,b'mdia'))
        handler=one(media,b'hdlr')
        if len(handler)>=12 and bytes(handler[8:12])==b'soun':
            raw_tracks.append((parts,media))
    if len(raw_tracks)!=1:
        fail()
    parts,media=raw_tracks[0]
    mdhd=clock_duration(one(media,b'mdhd'));movie_clock=clock_duration(one(movie_boxes,b'mvhd'))
    header=one(parts,b'tkhd');current=version(header);offset=20 if current==0 else 28
    width=4 if current==0 else 8
    if len(header)<offset+width:
        fail()
    track_duration=int.from_bytes(header[offset:offset+width],'big')
    tables=boxes(one(boxes(one(media,b'minf')),b'stbl'));stts=one(tables,b'stts')
    if len(stts)<8 or bytes(stts[:4])!=bytes(4):
        fail()
    count=struct.unpack_from('>I',stts,4)[0]
    if not 0<count<=64 or len(stts)!=8+count*8:
        fail()
    runs=[];samples=[];clock=0
    for n in range(count):
        sample_count,duration=struct.unpack_from('>II',stts,8+n*8)
        if not sample_count or not duration or len(samples)+sample_count>4096:
            fail()
        runs.append({'sampleCount':sample_count,'sampleDuration':duration})
        for _ in range(sample_count):
            samples.append({'dts':clock,'duration':duration});clock+=duration
    return {'observed':True,'sourceSHA256':digest,'movie':movie_clock,
        'audio':{**track,'mdhdDuration':mdhd['duration'],'tkhdMovieDuration':track_duration,
            'sttsRuns':runs,'sttsSamples':len(samples),'sttsDuration':clock,
            'firstPhysicalSamples':samples[:3],'lastPhysicalSamples':samples[-3:]},
        'sampleClocksScope':'Raw source STTS/MDHD/MVHD/ELST units; no presentation discard map',
        'missingSampleLocationEstablished':False,'decoderEOFQualified':False}

def retained_source_boundary_bytes(data,digest):
    try:
        return source_audio_boundary(data,digest)
    except (RuntimeError,TypeError,ValueError,KeyError,AttributeError,struct.error):
        return {'observed':False,'failureClass':'browser_source_audio_boundary_shape',
                'missingSampleLocationEstablished':False}

def retained_source_boundary(path,digest):
    try:
        return retained_source_boundary_bytes(bounded_bytes(path,2<<20,'browser_source_audio_boundary_bound'),digest)
    except (RuntimeError,OSError):
        return {'observed':False,'failureClass':'browser_source_audio_boundary_read','missingSampleLocationEstablished':False}
