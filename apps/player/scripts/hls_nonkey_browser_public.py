"""Bounded actual browser/group and public HTTP observations; no cache admission."""
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import time
from hls_followon_public import bounded_bytes, check, encoder_count
from hls_timeline_http import sha
from hls_timeline_packets import manifest_facts
from hls_remaining_process import group_members

def chromium_join(node, path):
    if not path.exists():
        return {'identityUnavailable': True, 'confirmedZeroSamples': 0}
    raw=bounded_bytes(path,4096,'browser_owned_identity_bound')
    owner=json.loads(raw)
    check(type(owner.get('pid')) is int and owner['pid']>1 and owner.get('nodePID')==node.pid
          and owner.get('parent')==node.pid and owner.get('group')==owner['pid']
          and owner.get('session')==owner['pid'] and re.fullmatch(r'[0-9]{1,24}',owner.get('startTicks','')),
          'browser_owned_identity_shape')
    def members():
        selected=group_members(owner['group'])
        check(len(selected)<=256,'browser_owned_group_bound')
        for pid in selected:
            try:
                data=Path('/proc/'+str(pid)+'/stat').read_text()
            except FileNotFoundError:
                continue
            check(len(data)<=4096,'browser_owned_stat_bound')
            fields=data[data.rfind(')')+2:].split()
            check(int(fields[2])==owner['group'] and int(fields[3])==owner['session']
                  and int(fields[19])>=int(owner['startTicks']),'browser_owned_group_changed')
        return selected
    alive=members()
    if alive:
        leader=Path('/proc/'+str(owner['pid'])+'/stat')
        if leader.exists():
            data=leader.read_text()
            check(data[data.rfind(')')+2:].split()[19]==owner['startTicks'],'browser_owned_leader_reused')
        try: os.killpg(owner['group'],signal.SIGTERM)
        except ProcessLookupError: pass
        deadline=time.monotonic()+2
        while members() and time.monotonic()<deadline: time.sleep(0.05)
        if members():
            try: os.killpg(owner['group'],signal.SIGKILL)
            except ProcessLookupError: pass
    zeros=0
    deadline=time.monotonic()+3
    while zeros<2 and time.monotonic()<deadline:
        zeros=zeros+1 if members()==[] else 0
        time.sleep(0.05)
    return {'identity':owner,'confirmedZeroSamples':zeros,'remainingOwnedPIDs':members()}

def public_media(api, selected, directory, log_path, server, source):
    directory.mkdir()
    request_ids=set()
    def get(path):
        status,data,headers=api.http(path)
        identity=next((v for k,v in headers.items() if k.lower()=='x-request-id'),'')
        check(re.fullmatch(r'[a-zA-Z0-9_-]{1,80}',identity),'browser_public_request_id')
        request_ids.add(identity)
        return status,data
    status,master=get(selected)
    check(status==200 and len(master)<=65536,'browser_public_master')
    renditions=[v for v in master.decode().splitlines() if re.fullmatch(r'[1-9][0-9]{2,3}p/index\.m3u8',v)]
    check(len(renditions)==1,'browser_public_rendition')
    rendition=renditions[0].rsplit('/',1)[0]
    prefix=selected.rsplit('/',1)[0]+'/'+rendition+'/'
    status,manifest=get(prefix+'index.m3u8')
    check(status==200 and len(manifest)<=65536,'browser_public_variant')
    facts,names=manifest_facts(manifest)
    check(0<len(names)<=32,'browser_public_asset_count')
    evidence={'initialMasterSHA256':hashlib.sha256(master).hexdigest(),
        'initialVariantSHA256':hashlib.sha256(manifest).hexdigest(),'initialVariantFacts':facts,
        'publicEndlistScope':'Public projection observation; not physical producer EOF',
        'initialAssetFailures':[],'initialAssets':{},'finalAssets':{},'rendition':rendition}
    assets=[]
    for name in ['init.mp4',*[v for v,_ in names]]:
        status,data=get(prefix+name)
        check(status==200 and 0<len(data)<=2<<20,'browser_initial_public_asset')
        path=directory/name
        path.write_bytes(data)
        assets.append(path)
        evidence['initialAssets'][name]={'status':status,'bytes':len(data),'sha256':sha(path)}
    joined=directory/'joined.mp4'
    joined.write_bytes(b''.join(bounded_bytes(p,2<<20,'browser_join_bound') for p in assets))
    deadline=time.monotonic()+10
    zeros=0
    completed=[]
    while zeros<2 and time.monotonic()<deadline:
        rows=[]
        for line in bounded_bytes(log_path,2<<20,'browser_completion_log_bound').splitlines():
            if line.startswith(b'{') and line.endswith(b'}'):
                entry=json.loads(line)
                if entry.get('msg')=='HLS transcode completed' and entry.get('request_id') in request_ids:
                    rows.append({'requestID':entry['request_id'],'state':'completed'})
        status,metrics=api.http('/settings/metrics')
        check(status==200 and len(metrics)<=65536,'browser_owner_metrics')
        selected_metrics={}
        for line in metrics.decode().splitlines():
            match=re.fullmatch(r'kinosail_workload_(active|waiting)\{class="(playback|background)"\} ([0-9]{1,8})',line)
            if match:selected_metrics[match[1]+':'+match[2]]=int(match[3])
        check(len(selected_metrics)==4,'browser_workload_projection_shape')
        clear=encoder_count(server,source)==0 and all(v==0 for v in selected_metrics.values()) and rows
        zeros=zeros+1 if clear else 0
        completed=rows
        time.sleep(0.05)
    check(zeros==2,'browser_correlated_publication_not_joined')
    evidence.update(correlatedCompletions=completed,ownedFFmpegZeroSamples=zeros,workloadZeros=selected_metrics)
    status,final_manifest=get(prefix+'index.m3u8')
    check(status==200 and len(final_manifest)<=65536,'browser_final_public_variant')
    evidence['finalVariantSHA256']=hashlib.sha256(final_manifest).hexdigest()
    evidence['finalVariantFacts']=manifest_facts(final_manifest)[0]
    for path in assets:
        status,data=get(prefix+path.name)
        check(status==200 and 0<len(data)<=2<<20,'browser_final_public_asset')
        evidence['finalAssets'][path.name]={'status':status,'bytes':len(data),
            'sha256':hashlib.sha256(data).hexdigest()}
        if evidence['initialAssets'][path.name]!=evidence['finalAssets'][path.name]:
            evidence['initialAssetFailures'].append('changed_public_asset:'+path.name)
    evidence['initialAssetBytesUnchanged']=not evidence['initialAssetFailures']
    return joined,manifest,assets,evidence
