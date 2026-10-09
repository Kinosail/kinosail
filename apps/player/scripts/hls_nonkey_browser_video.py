"""Bind actual public video packets and AVC/color configuration before browser decode."""
from fractions import Fraction
import hashlib
import json
import math
import re
import struct
import subprocess
from hls_followon_public import bounded_bytes, check
from hls_remaining_nonkey_init import boxes, one

def video_rows(rows):
    selected=[v for v in rows if v.get('stream_index')==0]
    check(0<len(selected)<=4096,'browser_video_packet_bound')
    for row in selected:
        check(re.fullmatch(r'SHA256:[a-f0-9]{64}',row.get('data_hash','')),'browser_video_packet_hash')
        for field in ['pts_time','dts_time','duration_time']:
            value=row.get(field)
            check(type(value) is str and len(value)<=64,'browser_video_packet_clock_shape')
            try:finite=math.isfinite(float(value))
            except ValueError:finite=False
            check(finite and (field!='duration_time' or float(value)>0),'browser_video_packet_clock')
    return selected

def public_video_suffix(source,public,required):
    source,public=video_rows(source),video_rows(public)
    digest=required.get('payloadSHA256','')
    check(re.fullmatch(r'[a-f0-9]{64}',digest),'browser_required_idr_hash')
    check('K' in public[0].get('flags',''),'browser_actual_video_first_key')
    point=required.get('pts_time')
    check(type(point) is str and len(point)<=64,'browser_required_idr_clock_shape')
    try:finite=math.isfinite(float(point))
    except ValueError:finite=False
    check(finite,'browser_required_idr_clock')
    source_hashes=[v['data_hash'] for v in source]
    public_hashes=[v['data_hash'] for v in public]
    starts=[n for n,v in enumerate(source_hashes) if v==public_hashes[0] and
        source_hashes[n:n+len(public_hashes)]==public_hashes]
    start=starts[0] if len(starts)==1 else None
    key=next((v for v in source if v['data_hash']=='SHA256:'+digest),None)
    check(key is not None and 'K' in key.get('flags','') and
        abs(Fraction(key['pts_time'])-Fraction(required['pts_time']))<=Fraction(1,1000000),
        'browser_actual_required_source_idr')
    return {'sourceVideoPackets':len(source),'publicVideoPackets':len(public),'sequenceMatches':len(starts),
        'uniqueSourceStart':start,'firstPayloadIsRequiredIDR':public_hashes[0]=='SHA256:'+digest,
        'wholeSourcePayloadTail':start is not None and start+len(public)==len(source),
        'qualified':start is not None and start+len(public)==len(source) and public_hashes[0]=='SHA256:'+digest,
        'packetRowsTrimmed':0,'payloadRowsScope':'All actual server public video packets; clocks retained separately'}

def avc_configuration_metadata(data):
    check(isinstance(data,(bytes,memoryview)) and 0<len(data)<=8<<20,'browser_avc_input_bound')
    movie=one(boxes(memoryview(data)),b'moov')
    selected=[]
    for kind,track in boxes(movie):
        if kind!=b'trak':continue
        node=track
        for child in [b'mdia',b'minf',b'stbl',b'stsd']:node=one(boxes(node),child)
        check(len(node)>=8 and bytes(node[:4])==bytes(4),'browser_stsd_header')
        count=struct.unpack_from('>I',node,4)[0]
        entries=boxes(node[8:])
        check(0<count<=8 and len(entries)==count,'browser_stsd_entry_bound')
        selected.extend(value for kind,value in entries if kind==b'avc1')
    check(len(selected)==1 and len(selected[0])>=78,'browser_avc_sample_entry')
    children=boxes(selected[0][78:])
    config=one(children,b'avcC')
    check(7<=len(config)<=4096 and config[0]==1,'browser_avc_configuration_bound')
    raw=one(children,b'colr',required=False)
    color=None
    if raw is not None:
        kind=bytes(raw[:4])
        check(kind in [b'nclx',b'nclc'] and len(raw)==(11 if kind==b'nclx' else 10),'browser_color_box_shape')
        primaries,transfer,matrix=struct.unpack_from('>HHH',raw,4)
        if kind==b'nclx':check(raw[10]&127==0,'browser_color_reserved_bits')
        color={'kind':kind.decode(),'primaries':primaries,'transfer':transfer,'matrix':matrix,
            'fullRange':bool(raw[10]&128) if kind==b'nclx' else None}
    return {'entry':'avc1','avcConfigurationBytes':len(config),
        'avcConfigurationSHA256':hashlib.sha256(config).hexdigest(),'color':color}

def video_stream(path):
    fields=['codec_name','profile','level','width','height','pix_fmt','color_range','color_space',
        'color_transfer','color_primaries','chroma_location','sample_aspect_ratio','time_base',
        'start_time','duration','extradata_size','extradata_hash']
    result=subprocess.run(['ffprobe','-v','error','-select_streams','v:0','-show_data_hash','sha256',
        '-show_entries','stream='+','.join(fields),'-of','json',str(path)],capture_output=True,timeout=20)
    check(result.returncode==0 and len(result.stdout)<=65536,'browser_video_stream_probe')
    streams=json.loads(result.stdout).get('streams',[])
    check(len(streams)==1,'browser_video_stream_count')
    row={key:streams[0].get(key) for key in fields}
    check(row['codec_name']=='h264' and row['width']==640 and row['height']==360 and
        re.fullmatch(r'SHA256:[a-f0-9]{64}',row.get('extradata_hash') or ''),'browser_video_stream_identity')
    for value in row.values():
        check(value is None or type(value) is int or type(value) is str and len(value)<=80,
            'browser_video_stream_projection_bound')
    return row

def actual_video_evidence(source,public,source_rows,public_rows,required,result):
    result.update(currentStage='actual-public-video-packets',productionAcceptance=False,browserFrameAcceptance=False)
    result['packetSuffix']=public_video_suffix(source_rows,public_rows,required)
    check(result['packetSuffix']['qualified'],'browser_actual_video_complete_payload_tail')
    result['currentStage']='source-avc-configuration'
    result['sourceAVC']=avc_configuration_metadata(bounded_bytes(source,2<<20,'browser_source_avc_file_bound'))
    result['currentStage']='public-avc-configuration'
    result['publicAVC']=avc_configuration_metadata(bounded_bytes(public,8<<20,'browser_public_avc_file_bound'))
    result['currentStage']='source-video-stream'
    result['sourceVideoStream']=video_stream(source)
    result['currentStage']='public-video-stream'
    result['publicVideoStream']=video_stream(public)
    source_config,public_config=result['sourceAVC'],result['publicAVC']
    source_stream,public_stream=result['sourceVideoStream'],result['publicVideoStream']
    result['configurationIdentity']=source_config['avcConfigurationSHA256']==public_config['avcConfigurationSHA256'] and source_stream['extradata_hash']==public_stream['extradata_hash']=='SHA256:'+source_config['avcConfigurationSHA256']
    result['colorMetadataIdentity']=source_config['color']==public_config['color']
    result['currentStage']='configuration-identity'
    check(result['configurationIdentity'],'browser_actual_avc_configuration_identity')
    result['currentStage']='complete'
    return result
