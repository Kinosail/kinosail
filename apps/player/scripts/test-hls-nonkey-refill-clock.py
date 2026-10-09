#!/usr/bin/env python3
"""R15 fixed-source initial/refill mux isolation; not public or production proof.
A/B/C preserve original, explicit-discontinuous, and normal-initial policies.
Every result retains its own canonical init, full payloads, PCM and packet clocks.
"""
from fractions import Fraction
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import time
from hls_timeline_fixture import fixture
from hls_timeline_http import sha, source_state
from hls_timeline_packets import manifest_facts
from hls_followon_frames import decode_frames, stream_metadata
from hls_followon_public import bounded_bytes, check
from hls_remaining_mux import mux_case
from hls_remaining_nonkey_evidence import observed_media, packet_rows
from hls_remaining_nonkey_fragment import fragment_metadata
from hls_remaining_nonkey_init import initialization_metadata
from hls_remaining_nonkey_boundary import packet_tail
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_nonkey_browser_audio_boundary import retained_audio_boundary
from hls_nonkey_browser_packet_association import packet_association
from hls_nonkey_browser_packet_clock import packet_clock
from hls_nonkey_refill_clock import audio_origin, refill_shift

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-nonkey-refill-clock' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),
    'tree': subprocess.check_output(['git','rev-parse','HEAD^{tree}'],text=True).strip(),
    'result':'failed','initialCases':[],'refillCases':[],'productionAcceptance':False,
    'publicServerProof':False,'sourceAndClientChanged':False,
    'boundary':'Fixed MP4/pinned-codec initial-refill isolation; all historical public/browser failures remain.',
    'sourceIdentityExpected':'198a6808092cea2a5481afc24e79481e4966ae577cbb8c46c214737e2173a3ed'}
guard = DiagnosticDeadline(240)
guard.__enter__()

def run(command, timeout=30, bound=4<<20):
    guard.check()
    process = subprocess.run(command,capture_output=True,timeout=min(timeout,guard.check(5)))
    check(len(process.stdout)<=bound and len(process.stderr)<=bound,'refill_matrix_command_bound')
    check(process.returncode==0,'refill_matrix_command_failed')
    return process.stdout

def assets(directory):
    manifest = bounded_bytes(directory/'index.m3u8',65536,'refill_matrix_manifest')
    facts, names = manifest_facts(manifest)
    check(facts['endlist'] and 0 < len(names) <= 32,'refill_matrix_complete_manifest')
    paths = [directory/name for name,_ in names]
    return bounded_bytes(directory/'init.mp4',2<<20,'refill_matrix_init'), paths, facts

def initial_facts(source_rows, directory, initial):
    init, fragments, manifest = assets(directory)
    tracks = initialization_metadata(init)['tracks']
    audio = [v for v in tracks if v['handler']=='soun']
    check(len(audio)==1 and audio[0]['mediaTimescale']==48000,'refill_matrix_audio_track')
    first = fragment_metadata(bounded_bytes(fragments[0],8<<20,'refill_matrix_first_fragment'))
    selected = [v for v in first['tracks'] if v['trackID']==audio[0]['trackID']]
    check(len(selected)==1 and selected[0]['samples'],'refill_matrix_raw_first')
    raw_first = selected[0]['samples'][0]['pts']
    public_rows, missing = packet_rows(directory/'joined.mp4')
    check(not missing,'refill_matrix_complete_packet_clocks')
    video = [v for v in public_rows if v['stream_index']==0]
    metadata = stream_metadata(directory/'joined.mp4')
    video_streams = [v for v in metadata['streams'] if v['codec_type']=='video']
    check(len(video_streams)==1 and video_streams[0]['time_base']=='1/16000',
          'refill_matrix_fixed_video_grid')
    clock = Fraction(video[0]['pts'],16000)
    check(0 <= clock <= 1,'refill_matrix_video_clock_bound')
    offset = Fraction(8)+clock
    mux_us = (offset.numerator*1000000+offset.denominator//2)//offset.denominator
    origin = audio_origin(source_rows,public_rows,raw_first,48000)
    initial.update(initialization=initialization_metadata(init),manifest=manifest,
        completePacketRows=public_rows,audioRawFirstPTS=raw_first,audioPhysicalSourceOffsetTicks=origin,
        measuredEditedVideoClock={'numerator':clock.numerator,'denominator':clock.denominator},
        actualRefillMuxOffsetMicros=mux_us,
        derivedAudioBSFShiftTicks=refill_shift(origin,20000000,mux_us,48000))
    return init, fragments, mux_us, initial['derivedAudioBSFShiftTicks']

def projection(row):
    facts = row.get('observations',{})
    tail = row.get('aacPayloadTail',{})
    association = packet_association(tail) if tail else {}
    association.pop('sourceOrdinals',None)
    boundary = retained_audio_boundary(facts) if facts else {}
    pcm = facts.get('nativePCM',{})
    return {'initialPolicy':row.get('initialPolicy'),'audioMapping':row.get('audioMapping'),
        'result':row.get('result'),'failureClass':row.get('failureClass'),
        'audioShiftTicks':row.get('audioShiftTicks'),'videoCompletePayloadTail':row.get('videoCompletePayloadTail'),
        'videoFramesExact':facts.get('mapping',{}).get('exactRequestedSequence'),
        'aac':{k:tail.get(k) for k in ['sourcePackets','publicPackets','wholePublicPacketTail','uniqueSourceStart']},
        'aacAssociation':association,'aacClock':packet_clock(tail,boundary) if tail else {},
        'audioBoundary':boundary,'nativeReferenceEqual':pcm.get('wholePublicEqualsReference'),
        'publicSamples':pcm.get('public',{}).get('samples'),'expectedSamples':pcm.get('referenceSeek',{}).get('samples'),
        'publicAndSourceEOF':pcm.get('publicAndSourceCompleteEOFAccounted'),
        'canonicalAssetsUnchanged':row.get('canonicalAssetsUnchanged')}

try:
    receipt['codecVersion']=run(['ffmpeg','-version'],10,65536).decode().splitlines()[0]
    regular, base = fixture(RUN,'regular',48,','.join(str(v) for v in range(0,32,2)),frames=768)
    source = RUN/'regular-copy.mp4'
    run(['ffmpeg','-nostdin','-v','error','-i',str(regular),'-map','0:v:0','-map','0:a:0','-c','copy',str(source)],45)
    before = source_state(source)
    check(before['sha256']==receipt['sourceIdentityExpected'],'refill_matrix_source_identity')
    source_rows, missing = packet_rows(source)
    check(not missing,'refill_matrix_source_clocks')
    source_decode, frames = decode_frames(source)
    check(len(frames)==768,'refill_matrix_source_frames')
    metadata = dict(base,sha256=before['sha256'],sourceFramePTS=[v[0] for v in frames],
        sourceTimeOriginSeconds=0)
    receipt.update(source=before,sourceMetadata=stream_metadata(source),sourceDecode=source_decode,
        completeSourcePacketRows=source_rows,completeSourceFrameRows=frames)
    video = [v for v in source_rows if v['stream_index']==0]
    check(len(video)==768,'refill_matrix_video_count')
    key = video[480]
    check(key['pts']==320000 and 'K' in key['flags'],'refill_matrix_source20s_key')
    clip = 'noise=amount=0:drop=lt(pts+round(20/tb)\\,ceil('+str(key['dts'])+'*round(1/tb)/16000))'
    explicit = 'avoid_negative_ts=disabled:use_editlist=1'
    policies = [
        ('r15-original',{'output':['-copypriorss:v','0']}),
        ('explicit-discontinuous',{'output':['-copypriorss:v','0','-avoid_negative_ts','disabled'],
            'segment':'movflags=+frag_discont+skip_sidx:'+explicit}),
        ('normal-initial',{'output':['-copypriorss:v','0','-avoid_negative_ts','disabled'],
            'segment':'movflags=+skip_sidx:'+explicit})]
    for label, options in policies:
        guard.check(45)
        directory = RUN/label
        initial = mux_case(run,source,metadata,directory,label,12,options)
        receipt['initialCases'].append(initial)
        if initial['result']!='observed':
            continue
        init, prefix, mux_us, shift = initial_facts(source_rows,directory,initial)
        check(len(prefix)==10,'refill_matrix_initial_cuts')
        canonical_hashes = {p.name:sha(p) for p in [directory/'init.mp4',*prefix]}
        for mapping, adjustment in [('video-global-only',0),('source-packet-origin',shift)]:
            guard.check(30)
            target = directory/mapping
            bsf = clip
            if mapping=='source-packet-origin':
                bsf += ',setts=pts=PTS+'+str(adjustment)+':dts=DTS+'+str(adjustment)
            output = ['-copypriorss:v','0','-avoid_negative_ts','disabled',
                '-output_ts_offset',format(mux_us/1000000,'.6f'),'-bsf:a',bsf,'-start_number','4']
            value = mux_case(run,source,metadata,target,mapping,20,{'output':output})
            value.update(initialPolicy=label,audioMapping=mapping,audioShiftTicks=adjustment)
            receipt['refillCases'].append(value)
            if value['result']!='observed':
                continue
            _, suffix, refill_manifest = assets(target)
            check(len(suffix)==6,'refill_matrix_refill_cuts')
            fragments = [*prefix[:4],*suffix]
            joined = target/'canonical-joined.mp4'
            data = init
            for path in fragments:
                data += bounded_bytes(path,8<<20,'refill_matrix_join_fragment')
                check(len(data)<=32<<20,'refill_matrix_join_bound')
            joined.write_bytes(data)
            observed = {}
            value['observations']=observed
            observed_media(source,joined,init,fragments,metadata,12,observed)
            tail = packet_tail(observed['sourcePacketRows'],observed['publicPacketRows'])
            value['aacPayloadTail']=tail
            public_video = [v['data_hash'] for v in observed['publicPacketRows'] if v['stream_index']==0]
            value['videoCompletePayloadTail']=public_video==[v['data_hash'] for v in video[288:]]
            value['canonicalAssetsUnchanged']=canonical_hashes=={p.name:sha(p) for p in [directory/'init.mp4',*prefix]}
            value['sourceUnchanged']=source_state(source)==before
            check(value['canonicalAssetsUnchanged'] and value['sourceUnchanged'],'refill_matrix_assets_changed')
            value['safeProjection']=projection(value)
            print(json.dumps({'refillClockCase':value['safeProjection']}),flush=True)
    check(len(receipt['initialCases'])==3,'refill_matrix_initial_case_count')
    check(len(receipt['refillCases'])==6,'refill_matrix_refill_case_count')
    receipt['sourceUnchanged']=source_state(source)==before
    check(receipt['sourceUnchanged'],'refill_matrix_source_changed')
    receipt['result']='observed'
except Exception as error:
    receipt['failureClass']=str(error) if isinstance(error,RuntimeError) else type(error).__name__
finally:
    with guard.cleanup():
        files = [Path(__file__),ROOT/'apps/player/scripts/hls_nonkey_refill_clock.py',
                 ROOT/'apps/player/scripts/test_hls_nonkey_refill_clock.py']
        receipt['executedScriptSHA256']={str(p.relative_to(ROOT)):sha(p) for p in files}
        raw = json.dumps(receipt,separators=(',',':'),allow_nan=False)+'\n'
        check(0<len(raw.encode())<=32<<20,'refill_matrix_receipt_bound')
        path = RUN/'receipt.json'
        path.write_text(raw)
        (RUN/'SHA256SUMS').write_text(sha(path)+'  receipt.json\n'+''.join(sha(p)+'  '+str(p.relative_to(ROOT))+'\n' for p in files))
        print(json.dumps({'revision':receipt['revision'],'tree':receipt['tree'],'result':receipt['result'],
            'failureClass':receipt.get('failureClass'),'sourceUnchanged':receipt.get('sourceUnchanged'),
            'initialPolicies':[{k:v.get(k) for k in ['label','result','failureClass','audioRawFirstPTS',
                'audioPhysicalSourceOffsetTicks','measuredEditedVideoClock','actualRefillMuxOffsetMicros',
                'derivedAudioBSFShiftTicks']} for v in receipt['initialCases']],
            'refillClockCases':[projection(v) for v in receipt['refillCases']],
            'receiptSHA256':sha(path),'productionAcceptance':False,'publicServerProof':False}))
    guard.__exit__()
raise SystemExit(0 if receipt['result']=='observed' else 1)
