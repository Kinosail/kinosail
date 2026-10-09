#!/usr/bin/env python3
"""Real authenticated Go cold transport; failed preparation is never certified."""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import socket
import subprocess
import sys
import threading
import time
from hls_timeline_http import PublicServer, sha, source_state
from hls_timeline_packets import manifest_facts, fragment_audio
from hls_remaining_nonkey_evidence import observed_media, native_pcm
from hls_remaining_nonkey_boundary import packet_tail
from hls_remaining_process import finish_processes
from hls_followon_public import bounded_bytes, check, prepare_once, sample_resources
from hls_remaining_nonkey_deadline import DiagnosticDeadline

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-nonkey-browser' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),
    'tree': subprocess.check_output(['git','rev-parse','HEAD^{tree}'],text=True).strip(),
    'productionAcceptance': False, 'sourceAndClientUnchanged': True, 'containers': [], 'result': 'failed',
    'browserAudioPresentationAccepted': False, 'transport': 'Real Go authenticated unindexed cold HLS',
    'preparationAcceptance': False}
guard = DiagnosticDeadline(1260)
guard.__enter__()

def run(argv, timeout=60, bound=4<<20):
    guard.check()
    process = subprocess.run(argv,capture_output=True,timeout=timeout)
    check(len(process.stdout)<=bound,'browser_command_bound')
    check(process.returncode==0,'browser_command_failed')
    return process.stdout

def public_media(api, selected, directory):
    status, master, _ = api.http(selected)
    check(status==200 and len(master)<=65536,'browser_public_master')
    lines = master.decode().splitlines()
    renditions = [v for v in lines if re.fullmatch(r'[1-9][0-9]{2,3}p/index\.m3u8',v)]
    check(len(renditions)==1,'browser_public_rendition')
    prefix = selected.rsplit('/',1)[0]+'/'+renditions[0].rsplit('/',1)[0]+'/'
    status, manifest, _ = api.http(prefix+'index.m3u8')
    check(status==200 and b'#EXT-X-ENDLIST' in manifest,'browser_public_eof_manifest')
    _, names = manifest_facts(manifest)
    check(0<len(names)<=32,'browser_public_asset_count')
    directory.mkdir()
    assets = []
    for name in ['init.mp4',*[v for v,_ in names]]:
        status, data, _ = api.http(prefix+name)
        check(status==200 and 0<len(data)<=2<<20,'browser_public_asset_bound')
        target = directory/name
        target.write_bytes(data)
        assets.append(target)
    joined = directory/'joined.mp4'
    joined.write_bytes(b''.join(bounded_bytes(p,2<<20,'browser_join_bound') for p in assets))
    return joined, manifest, assets

try:
    # The original all-frame, payload-tail, native PCM and negative-control proof stays unchanged.
    run([sys.executable,str(ROOT/'apps/player/scripts/test-hls-nonkey-supported-followon.py')],300,2<<20)
    paths = list((ROOT/'.verification/hls-nonkey-mux').glob('seek-followon-*/receipt.json'))
    check(len(paths)==1,'browser_cli_receipt_selection')
    cli = json.loads(bounded_bytes(paths[0],32<<20,'browser_cli_receipt_bound'))
    check(cli['revision']==receipt['revision'] and cli['result']=='observed'
          and cli['priorProducerGateRemainsEmpty'] and not cli['productionAcceptance'],'browser_cli_binding')
    receipt['unchangedCLIReceipt'] = {'sha256':sha(paths[0]),'revision':cli['revision'],'tree':cli['tree'],
        'qualifiedConsumerCases':cli['qualifiedConsumerCases'],'priorProducerGateRemainsEmpty':True}
    binary = RUN/'kinosail'
    run(['go','-C',str(ROOT/'apps/player'),'build','-p=1','-o',str(binary),'./cmd/kinosail'],180)
    real = shutil.which('ffmpeg')
    check(real is not None,'browser_pinned_codec_missing')
    for source_facts in cli['sources']:
        guard.check(180)
        source_original = Path(source_facts['path'])
        container = source_facts['container']
        directory = RUN/container
        media = directory/'media'
        media.mkdir(parents=True)
        source = media/('Fixture.'+container)
        reference = media/'Reference.mp4'
        shutil.copy2(source_original,source)
        shutil.copy2(paths[0].parent/'regular-copy.mp4',reference)
        before, reference_before = source_state(source),source_state(reference)
        # Both references preserve all independently decoded video source identities.
        mp4_facts = next(v for v in cli['sources'] if v['container']=='mp4')
        check([r[1] for r in source_facts['completeSourceFrameRows']]==
              [r[1] for r in mp4_facts['completeSourceFrameRows']],'browser_reference_video_identity')
        selected_cases = [v for v in cli['cases'] if v['container']==container and v['label']=='shift-no-prior']
        check(len(selected_cases)==4 and all(v['consumerCapabilityQualified'] for v in selected_cases),
              'browser_fixed_recipe_cli_qualification')
        invocations = directory/'producer-invocations.jsonl'
        adapter = directory/'diagnostic-ffmpeg'
        adapter.write_text((ROOT/'apps/player/scripts/hls_nonkey_browser_wrapper.py').read_text())
        adapter.chmod(0o700)
        producer = directory/'producer-private.json'
        producer.write_text(json.dumps({'source':str(source),'ffmpeg':real,'invocations':str(invocations),
            'cases':[{'request':v['requestedRelativeSeconds'],
                'sourceIDRPTS':float(v['requiredPrecedingIDR']['pts_time']),
                'inputSeek':float(v['requiredPrecedingIDR']['pts_time'])-float(source_facts['metadata']['format']['start_time']),
                'delta':float(v['measuredDeltaSeconds'])} for v in selected_cases]}))
        with socket.socket() as listener:
            listener.bind(('127.0.0.1',0)); port=listener.getsockname()[1]
        api = PublicServer('http://localhost:'+str(port))
        env = dict(os.environ,KINOSAIL_LISTEN='127.0.0.1:'+str(port),KINOSAIL_AUTH_URL=api.url,
            KINOSAIL_TLS_ENABLED='false',KINOSAIL_DATA_DIR=str(directory/'config'),KINOSAIL_MEDIA_DIR=str(media),
            KINOSAIL_CACHE_DIR=str(directory/'cache'),KINOSAIL_BACKUP_DIR=str(directory/'backups'),
            KINOSAIL_BACKUP_KEY='synthetic-browser-key',KINOSAIL_FFMPEG=str(adapter),
            KINOSAIL_BROWSER_PRODUCER_CONFIG=str(producer))
        case = {'container':container,'preparation':[],'publicCases':[],'result':'failed'}
        receipt['containers'].append(case)
        stop=threading.Event()
        resources={'samples':0,'peakOwnedFFmpeg':0,'samplingErrors':0}
        log_path=directory/'server-private.log'
        with log_path.open('w') as log:
            server=subprocess.Popen([str(binary)],cwd=ROOT,env=env,stdout=log,stderr=log,start_new_session=True)
            sampler=threading.Thread(target=sample_resources,args=(server,source,stop,resources),daemon=True)
            sampler.start()
            try:
                api.authorize()
                items=api.call('/api/v1/library')['items']
                item=next(v for v in items if v['title']=='Fixture')
                ref_item=next(v for v in items if v['title']=='Reference')
                plan=api.call('/api/v1/items/'+item['id']+'/playback?videoCodecs=h264&audioCodecs=aac')
                check(plan['compatiblePlan']['mode']=='remux','browser_public_remux_policy')
                browser_cases=[]
                for selected_case in selected_cases:
                    requested=selected_case['requestedRelativeSeconds']
                    selected=plan['compatible'].replace('/index.m3u8','-o'+str(round(requested*1000))+'/index.m3u8')
                    check(re.fullmatch(r'/hls/[a-f0-9]{16}/p/r-[a-zA-Z0-9-]+/index\.m3u8',selected),
                          'browser_public_recipe')
                    preparation={'request':requested}
                    status,_,_=api.http('/api/v1/items/'+item['id']+'/playback-prepare','POST',{'source':selected},authenticated=False)
                    check(status==401,'browser_unauthorized_preparation')
                    prepare_once(api,'/api/v1/items/'+item['id']+'/playback-prepare',selected,
                                 log_path,server,source,preparation)
                    case['preparation'].append(preparation)
                    private_log = bounded_bytes(log_path,2<<20,'browser_private_log_bound')
                    for line in private_log.splitlines():
                        if line.startswith(b'{') and line.endswith(b'}'):
                            entry=json.loads(line)
                            if entry.get('msg')=='HLS startup preparation' and entry.get('request_id')==preparation['preparationAttempt']['requestID']:
                                preparation['safeCompletionPhase']=entry.get('phase','completion')
                    check(preparation['preparationAttempt']['completionState']==('ready' if requested==12 else 'unavailable'),
                          'browser_preparation_boundary_changed')
                    joined,manifest,assets=public_media(api,selected,directory/('public-'+str(requested)))
                    observed={}
                    metadata={'sourceFramePTS':source_facts['sourceFramePTS'],
                              'sourceTimeOriginSeconds':float(source_facts['metadata']['format']['start_time'])}
                    observed_media(source,joined,assets[0].read_bytes(),assets[1:],metadata,requested,observed)
                    tail=packet_tail(observed['sourcePacketRows'],observed['publicPacketRows'])
                    check(tail['wholePublicPacketTail'],'browser_complete_aac_payload_tail')
                    expected=selected_case['observations']['mapping']['expectedSourceIndices']
                    pcm,_=native_pcm(source,requested)
                    public_case={'request':requested,'observations':observed,'aacPayloadTail':tail,
                        'manifestSHA256':hashlib.sha256(manifest).hexdigest(),'audioDiscontinuities':[],
                        'fragmentAudioFacts':[],
                        'mediaSHA256Before':{p.name:sha(p) for p in assets},'publicJoinedSHA256':sha(joined)}
                    case['publicCases'].append(public_case)
                    previous_audio=None
                    for fragment in assets[1:]:
                        joined_fragment=joined.parent/'fragment.mp4'
                        joined_fragment.write_bytes(assets[0].read_bytes()+fragment.read_bytes())
                        audio=fragment_audio(joined_fragment)
                        public_case['fragmentAudioFacts'].append(dict(audio,segment=fragment.name))
                        if not audio['audioPacketOrderValid'] or audio['maximumAudioGapSeconds']>0.05 or audio['maximumAudioOverlapSeconds']>0.05:
                            public_case['audioDiscontinuities'].append('interior_audio_discontinuity')
                        if previous_audio is not None and abs(audio['firstAudioTime']-previous_audio)>0.05:
                            public_case['audioDiscontinuities'].append('fragment_audio_discontinuity')
                        previous_audio=audio['lastAudioEnd']
                    browser_cases.append({'request':requested,'expectedSourceIndices':expected,
                        'publicJoinedPath':str(joined),'expectedPCM':{'samples':pcm['samples'],'sha256':pcm['sha256']}})
                output=directory/'browser.json'
                private=directory/'browser-private.json'
                private.write_text(json.dumps({'origin':api.url,'token':api.token,'itemID':item['id'],
                    'referenceID':ref_item['id'],'cases':browser_cases,'output':str(output)}))
                private.chmod(0o600)
                run(['node',str(ROOT/'apps/player/e2e/hls-nonkey-browser.mjs'),str(private)],600,1<<20)
                browser=json.loads(bounded_bytes(output,8<<20,'browser_result_bound'))
                case['browser']=browser
                for value in case['publicCases']:
                    assets_directory=directory/('public-'+str(value['request']))
                    value['mediaSHA256After']={name:sha(assets_directory/name) for name in value['mediaSHA256Before']}
                    check(value['mediaSHA256Before']==value['mediaSHA256After'],'browser_media_changed')
                case['result']='observed'
            finally:
                case.update(finish_processes(server,source,stop,sampler))
                check(source_state(source)==before and source_state(reference)==reference_before,'browser_source_changed')
                case.update(sourceUnchanged=True,referenceUnchanged=True,resources=resources)
                if invocations.exists():
                    case['actualProducerInvocations']=json.loads('['+','.join(invocations.read_text().splitlines())+']')
                    check(invocations.stat().st_size<=65536,'browser_invocations_bound')
        print(json.dumps({'container':container,'result':case['result'],
            'preparation':[{'request':v['request'],'state':v.get('preparationAttempt',{}).get('completionState')}
                           for v in case['preparation']],
            'browserCases':case.get('browser',{}).get('cases',[]) and
                [{'request':v['request'],'label':v.get('label'),'frames':v.get('observer',{}).get('phases',[{}])[-1].get('rows',[]) and
                    len(v['observer']['phases'][-1]['rows']),'frameQualified':v.get('frameConsumerQualified'),
                  'audioQualified':v.get('qualification',{}).get('qualified'),'failureClass':v.get('failureClass')}
                 for v in case['browser']['cases']],
            'productionAcceptance':False,'browserAudioPresentationAccepted':False}),flush=True)
    receipt['result']='observed'
except Exception as error:
    receipt['failureClass']=str(error) if isinstance(error,RuntimeError) else type(error).__name__
finally:
    with guard.cleanup():
        receipt['handledTerminationSignals']=guard.signals
        files={Path(__file__),ROOT/'apps/player/scripts/hls_nonkey_browser_wrapper.py',
               *list((ROOT/'apps/player/e2e').glob('hls-nonkey-browser*.mjs'))}
        receipt['executedScriptSHA256']={str(p.relative_to(ROOT)):sha(p) for p in files}
        raw=json.dumps(receipt,separators=(',',':'),allow_nan=False)+'\n'
        check(0<len(raw.encode())<=32<<20,'browser_receipt_bound')
        target=RUN/'receipt.json'
        target.write_text(raw)
        (RUN/'SHA256SUMS').write_text(sha(target)+'  receipt.json\n'+''.join(sha(p)+'  '+str(p.relative_to(ROOT))+'\n' for p in sorted(files)))
        print(json.dumps({'revision':receipt['revision'],'tree':receipt['tree'],'result':receipt['result'],
            'failureClass':receipt.get('failureClass'),'receiptSHA256':sha(target),
            'productionAcceptance':False,'preparationAcceptance':False,'browserAudioPresentationAccepted':False}))
    guard.__exit__()
raise SystemExit(0 if receipt['result']=='observed' else 1)
