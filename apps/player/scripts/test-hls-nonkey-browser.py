#!/usr/bin/env python3
"""Real authenticated Go cold transport; failed preparation is never certified."""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import socket
import subprocess
import sys
import threading
import time
from hls_timeline_http import PublicServer, sha, source_state
from hls_timeline_packets import manifest_facts, fragment_audio
from hls_remaining_nonkey_evidence import observed_media, native_pcm
from hls_remaining_nonkey_boundary import packet_tail
from hls_remaining_process import finish_processes, join_group
from hls_nonkey_browser_public import chromium_join, public_media, safe_transport_projection, browser_reference_config
from hls_nonkey_browser_config import measured_delta, diagnostic_result
from hls_nonkey_browser_video import actual_video_evidence
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
guard = DiagnosticDeadline(840)
guard.__enter__()

def run(argv, timeout=60, bound=4<<20):
    guard.check()
    process = subprocess.run(argv,capture_output=True,timeout=timeout)
    check(len(process.stdout)<=bound,'browser_command_bound')
    check(process.returncode==0,'browser_command_failed')
    return process.stdout



try:
    receipt['currentStage']='strict-cli-source-proof'
    # The original all-frame, payload-tail, native PCM and negative-control proof stays unchanged.
    run([sys.executable,str(ROOT/'apps/player/scripts/test-hls-nonkey-supported-followon.py')],300,2<<20)
    paths = list((ROOT/'.verification/hls-nonkey-mux').glob('seek-followon-*/receipt.json'))
    check(len(paths)==1,'browser_cli_receipt_selection')
    cli = json.loads(bounded_bytes(paths[0],32<<20,'browser_cli_receipt_bound'))
    check(cli['revision']==receipt['revision'] and cli['result']=='observed'
          and cli['priorProducerGateRemainsEmpty'] and not cli['productionAcceptance'],'browser_cli_binding')
    receipt['unchangedCLIReceipt'] = {'sha256':sha(paths[0]),'revision':cli['revision'],'tree':cli['tree'],
        'qualifiedConsumerCases':cli['qualifiedConsumerCases'],'priorProducerGateRemainsEmpty':True}
    receipt['currentStage']='canonical-server-build'
    binary = RUN/'kinosail'
    run(['go','-C',str(ROOT/'apps/player'),'build','-p=1','-o',str(binary),'./cmd/kinosail'],180)
    real = shutil.which('ffmpeg')
    check(real is not None,'browser_pinned_codec_missing')
    for source_facts in [v for v in cli['sources'] if v['container']=='mp4']:
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
        selected_cases = [v for v in cli['cases'] if v['container']==container and v['label']=='shift-no-prior' and v['requestedRelativeSeconds'] in [12,12.5]]
        check(len(selected_cases)==2 and all(v['consumerCapabilityQualified'] for v in selected_cases),
              'browser_fixed_recipe_cli_qualification')
        invocations = directory/'producer-invocations.jsonl'
        adapter = directory/'diagnostic-ffmpeg'
        adapter.write_text((ROOT/'apps/player/scripts/hls_nonkey_browser_wrapper.py').read_text())
        adapter.chmod(0o700)
        shutil.copy2(ROOT/'apps/player/scripts/hls_nonkey_browser_config.py',directory/'hls_nonkey_browser_config.py')
        receipt['currentStage']='fixed-producer-rational-config'
        producer = directory/'producer-private.json'
        producer_cases=[]
        for value in selected_cases:
            delta=measured_delta(value['measuredDeltaSeconds'])
            producer_cases.append({'request':value['requestedRelativeSeconds'],
                'sourceIDRPTS':float(value['requiredPrecedingIDR']['pts_time']),
                'inputSeek':float(value['requiredPrecedingIDR']['pts_time'])-float(source_facts['metadata']['format']['start_time']),
                'delta':delta['seconds'],'measuredDeltaRational':delta['rational']})
        receipt['rationalProducerClocks']=[{'request':v['request'],'deltaRational':v['measuredDeltaRational'],'deltaSeconds':v['delta']} for v in producer_cases]
        producer.write_text(json.dumps({'source':str(source),'ffmpeg':real,'invocations':str(invocations),'cases':producer_cases}))
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
                receipt['currentStage']='authenticated-public-setup'
                api.authorize()
                items=api.call('/api/v1/library')['items']
                item=next(v for v in items if v['title']=='Fixture')
                ref_item=next(v for v in items if v['title']=='Reference')
                plan=api.call('/api/v1/items/'+item['id']+'/playback?videoCodecs=h264&audioCodecs=aac')
                check(plan['compatiblePlan']['mode']=='remux','browser_public_remux_policy')
                reference_config,case['referenceBindings']=browser_reference_config(api,ref_item,source,reference,source_facts)
                browser_cases=[]
                for selected_case in selected_cases:
                    requested=selected_case['requestedRelativeSeconds']
                    selected=plan['compatible'].replace('/index.m3u8','-o'+str(round(requested*1000))+'/index.m3u8')
                    check(re.fullmatch(r'/hls/[a-f0-9]{16}/p/r-[a-zA-Z0-9-]+/index\.m3u8',selected),
                          'browser_public_recipe')
                    receipt['currentStage']='public-preparation'
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
                    receipt['currentStage']='public-media-delivery'
                    joined,manifest,assets,delivery=public_media(api,selected,directory/('public-'+str(requested)),log_path,server,source)
                    receipt['currentStage']='complete-raw-media-observation'
                    observed={}
                    partial={'request':requested,'observations':observed,'delivery':delivery,
                        'mediaSHA256Before':{p.name:sha(p) for p in assets},'publicJoinedSHA256':sha(joined)}
                    case['publicCases'].append(partial)
                    invocation_rows=[json.loads(row) for row in bounded_bytes(invocations,65536,'browser_invocations_bound').decode().splitlines()] if invocations.exists() else []
                    delivery['diagnosticAdaptedInvocationCount']=sum(v.get('adapted') is True and v.get('request')==requested for v in invocation_rows)
                    check(requested!=12.5 or delivery['diagnosticAdaptedInvocationCount']>0,'browser_fixed_recipe_never_installed')
                    metadata={'sourceFramePTS':source_facts['sourceFramePTS'],
                              'sourceTimeOriginSeconds':float(source_facts['metadata']['format']['start_time'])}
                    observed_media(source,joined,assets[0].read_bytes(),assets[1:],metadata,requested,observed)
                    tail=packet_tail(observed['sourcePacketRows'],observed['publicPacketRows'])
                    partial['aacPayloadTail']=tail
                    try:
                        check(tail['wholePublicPacketTail'],'browser_complete_aac_payload_tail')
                    except RuntimeError:
                        if requested!=12:raise
                        partial.update(result='failed',failureClass='browser_complete_aac_payload_tail')
                        case.setdefault('heldTransportControls',[]).append({'request':requested,'failureClass':partial['failureClass']})
                        continue
                    partial['actualVideoEvidence']={}
                    try:
                        actual_video_evidence(source,joined,observed['sourcePacketRows'],observed['publicPacketRows'],selected_case['requiredPrecedingIDR'],partial['actualVideoEvidence'])
                    finally:print(json.dumps({'actualPublicVideo':partial['actualVideoEvidence']}),flush=True)
                    expected=selected_case['observations']['mapping']['expectedSourceIndices']
                    pcm,_=native_pcm(source,requested)
                    public_case={'request':requested,'observations':observed,'aacPayloadTail':tail,
                        'delivery':delivery,'manifestSHA256':hashlib.sha256(manifest).hexdigest(),'audioDiscontinuities':[],
                        'fragmentAudioFacts':[],
                        'mediaSHA256Before':partial['mediaSHA256Before'],'publicJoinedSHA256':partial['publicJoinedSHA256']}
                    partial.update(public_case)
                    public_case=partial
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
                        'publicJoinedPath':str(joined),'publicJoinedSHA256':sha(joined),'selectedSource':selected,
                        'publicVideoSuffixQualified':partial['actualVideoEvidence']['packetSuffix']['qualified'],
                        'initialDeliveryStable':delivery['initialAssetBytesUnchanged'],
                        'publicAssetSHA256':{delivery['rendition']+'/'+p.name:sha(p) for p in assets},
                        'expectedPCM':{'samples':pcm['samples'],'sha256':pcm['sha256']}})
                output=directory/'browser.json'
                private=directory/'browser-private.json'
                private.write_text(json.dumps({'origin':api.url,'token':api.token,'itemID':item['id'],
                    'referenceID':ref_item['id'],**reference_config,'cases':browser_cases,'output':str(output),
                    'browserOwnerFile':str(directory/'browser-owner-private.json')}))
                private.chmod(0o600)
                receipt['currentStage']='actual-browser-execution'
                node=subprocess.Popen(['node',str(ROOT/'apps/player/e2e/hls-nonkey-browser.mjs'),str(private)],
                    stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True)
                node_failed=False
                try:
                    node_stdout,node_stderr=node.communicate(timeout=min(480,guard.check(25)))
                    check(len(node_stdout)<=1<<20 and len(node_stderr)<=1<<20,'browser_node_output_bound')
                    print(node_stdout.decode(),end='',flush=True)
                    check(node.returncode==0,'browser_node_failed')
                except Exception:
                    node_failed=True
                    raise
                finally:
                    case['browserCleanupFailures']=[]
                    with guard.cleanup():
                        for label,join in [('ownedBrowserProcessJoin',lambda:join_group(node)),
                            ('ownedChromiumProcessJoin',lambda:chromium_join(node,directory/'browser-owner-private.json'))]:
                            try:case[label]=join()
                            except Exception:case['browserCleanupFailures'].append(label+'_failed')
                    joined=case.get('ownedBrowserProcessJoin',{})
                    chromium=case.get('ownedChromiumProcessJoin',{})
                    if case['browserCleanupFailures'] or joined.get('confirmedZeroSamples')!=2 or joined.get('qualificationFailures') or chromium.get('confirmedZeroSamples')!=2:
                        case['browserCleanupFailures'].append('browser_owned_join_failed')
                        if not node_failed:raise RuntimeError('browser_owned_join_failed')
                browser=json.loads(bounded_bytes(output,8<<20,'browser_result_bound'))
                case['browser']=browser
                for value in case['publicCases']:
                    assets_directory=directory/('public-'+str(value['request']))
                    value['mediaSHA256After']={name:sha(assets_directory/name) for name in value['mediaSHA256Before']}
                    check(value['mediaSHA256Before']==value['mediaSHA256After'],'browser_media_changed')
                case['result']='observed'
            finally:
                primary_failure=sys.exc_info()[1]
                cleanup_failed=False
                with guard.cleanup():
                    try:
                        case.update(finish_processes(server,source,stop,sampler))
                    except Exception:
                        case['serverCleanupFailureClass']='browser_server_join_failed'
                        cleanup_failed=True
                    joined=case.get('ownedProcessJoin',{})
                    cleanup_failed=cleanup_failed or joined.get('confirmedZeroSamples')!=2 or bool(joined.get('qualificationFailures')) or bool(case.get('cleanupFailures'))
                    case.update(sourceUnchanged=source_state(source)==before,
                        referenceUnchanged=source_state(reference)==reference_before,resources=resources)
                    if not case['sourceUnchanged'] or not case['referenceUnchanged']:
                        case['sourceFailureClass']='browser_source_changed'
                        cleanup_failed=True
                    if invocations.exists():
                        rows=bounded_bytes(invocations,65536,'browser_invocations_bound').decode().splitlines()
                        case['actualProducerInvocations']=[json.loads(row) for row in rows]
                case['safeTransportProjection']=safe_transport_projection(case)
                if cleanup_failed:
                    case['result']='failed'
                    if primary_failure is None:raise RuntimeError('browser_server_or_source_join_failed')
        print(json.dumps({'container':container,'result':case['result'],
            'preparation':[{'request':v['request'],'state':v.get('preparationAttempt',{}).get('completionState')}
                           for v in case['preparation']],
            'browserCases':case.get('browser',{}).get('cases',[]) and
                [{'request':v['request'],'label':v.get('label'),'frames':v.get('observer',{}).get('phases',[{}])[-1].get('rows',[]) and
                    len(v['observer']['phases'][-1]['rows']),'frameQualified':v.get('frameConsumerQualified'),
                  'audioQualified':v.get('qualification',{}).get('qualified'),'failureClass':v.get('failureClass')}
                 for v in case['browser']['cases']],
            'productionAcceptance':False,'browserAudioPresentationAccepted':False}),flush=True)
    receipt['currentStage']='complete'
    receipt['result']=diagnostic_result(receipt['containers'])
    if receipt['result']=='failed':receipt['failureClass']='browser_prepared_control_aac_payload_tail'
except Exception as error:
    receipt['failureClass']=str(error) if isinstance(error,RuntimeError) else type(error).__name__
    trace=error.__traceback__
    frames=[]
    while trace is not None and len(frames)<32:
        code=trace.tb_frame.f_code
        name=Path(code.co_filename).name
        function=code.co_name if re.fullmatch(r'[a-zA-Z_][a-zA-Z0-9_]{0,79}',code.co_name) else 'module'
        frames.append({'file':name if re.fullmatch(r'[a-zA-Z0-9_.-]{1,100}',name) else 'bounded-file',
            'function':function,'line':trace.tb_lineno})
        trace=trace.tb_next
    receipt['safeFailureFrames']=frames[-4:]
    receipt['failureMessageIncluded']=False
finally:
    with guard.cleanup():
        receipt['handledTerminationSignals']=guard.signals
        files={Path(__file__),ROOT/'apps/player/scripts/hls_nonkey_browser_wrapper.py',ROOT/'apps/player/scripts/hls_nonkey_browser_public.py',ROOT/'apps/player/scripts/hls_nonkey_browser_config.py',
               ROOT/'apps/player/scripts/test_hls_nonkey_browser_config.py',ROOT/'apps/player/scripts/hls_nonkey_browser_video.py',ROOT/'apps/player/scripts/test_hls_nonkey_browser_video.py',
               *list((ROOT/'apps/player/e2e').glob('hls-nonkey-browser*.mjs'))}
        receipt['executedScriptSHA256']={str(p.relative_to(ROOT)):sha(p) for p in files}
        raw=json.dumps(receipt,separators=(',',':'),allow_nan=False)+'\n'
        check(0<len(raw.encode())<=32<<20,'browser_receipt_bound')
        target=RUN/'receipt.json'
        target.write_text(raw)
        (RUN/'SHA256SUMS').write_text(sha(target)+'  receipt.json\n'+''.join(sha(p)+'  '+str(p.relative_to(ROOT))+'\n' for p in sorted(files)))
        print(json.dumps({'revision':receipt['revision'],'tree':receipt['tree'],'result':receipt['result'],
            'failureClass':receipt.get('failureClass'),'currentStage':receipt.get('currentStage'),
            'safeFailureFrames':receipt.get('safeFailureFrames',[]),'rationalProducerClocks':receipt.get('rationalProducerClocks',[]),
            'safeTransportProjection':[c.get('safeTransportProjection',{}) for c in receipt['containers']],
            'receiptSHA256':sha(target),
            'productionAcceptance':False,'preparationAcceptance':False,'browserAudioPresentationAccepted':False}))
    guard.__exit__()
raise SystemExit(0 if receipt['result']=='observed' else 1)
