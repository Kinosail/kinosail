// Fixed-fixture Hls.js metadata transfer only; no product bytes, cache or color policy edits.
import {createHash} from 'node:crypto';
import {colorArm,colorFacts} from './hls-nonkey-browser-color.mjs';
import {consumerQualification} from './hls-nonkey-browser-observer.mjs';
const sha=b=>createHash('sha256').update(b).digest('hex');
export function prepareAppColor(item,joined,pieces){
  const entries=Object.entries(item.publicAssetSHA256||{});
  if(item.request!==12.5 || !/^\/hls\/[A-Za-z0-9_/-]+\/index\.m3u8$/.test(item.selectedSource||'') ||
    entries.length<2 || entries.length>33 || entries.length!==pieces.length ||
    !entries[0][0].endsWith('/init.mp4') || sha(joined)!==item.publicJoinedSHA256 ||
    !Buffer.concat(pieces).equals(joined))throw Error('app_color_input_binding');
  for(const [n,[name,digest]] of entries.entries()){
    if(!/^[1-9][0-9]{2,3}p\/(?:init\.mp4|[A-Za-z0-9_-]{1,80}\.m4s)$/.test(name) ||
      pieces[n].length===0 || pieces[n].length>2<<20 || sha(pieces[n])!==digest)throw Error('app_color_asset_binding');
  }
  const generated=colorArm(joined,'601'),init=generated.bytes.subarray(0,generated.facts.initBytes);
  if(colorFacts(joined).initBytes!==pieces[0].length || init.length>65536)throw Error('app_color_init_binding');
  const prefix=item.selectedSource.slice(0,item.selectedSource.lastIndexOf('/')+1);
  const appendPlans=[{sha256:sha(init),bytes:init.length,data:init,init:true,fragmentIndices:[],combined:false}];
  for(let first=0;first<pieces.length-1;first++)for(let last=first;last<pieces.length-1;last++){
    const data=Buffer.concat(pieces.slice(first+1,last+2)),indices=Array.from({length:last-first+1},(_,n)=>first+n);
    if(data.length<=2<<20)appendPlans.push({sha256:sha(data),bytes:data.length,data,init:false,fragmentIndices:indices,combined:last>first});
    if(first===0){
      const combined=Buffer.concat([init,data]);
      if(combined.length<=2<<20)appendPlans.push({sha256:sha(combined),bytes:combined.length,data:combined,
        init:true,fragmentIndices:indices,combined:true});
    }
  }
  return {item:{...item,publicAssetSHA256:{...item.publicAssetSHA256,[entries[0][0]]:sha(init)}},
    init,initPath:prefix+entries[0][0],facts:generated.facts,appendPlans};
}
export function appColorAppendFacts(value,plan,pieces){
  const result={qualified:false,scope:'Actual Hls.js fixed-fixture init transfer; no policy/audio/native acceptance',
    appends:[],actualInitFacts:[],failures:value?.failures||[],overflow:value?.overflow===true};
  if(!value || value.overflow || value.failures?.length || !Array.isArray(value.buffers) ||
    !value.buffers.length || value.buffers.length>8 || !Array.isArray(value.appends) ||
    !value.appends.length || value.appends.length>128)return result;
  const sequences=new Map(),initialized=new Set();let initCount=0,healthy=true;
  for(const [ordinal,row] of value.appends.entries()){
    let payload;
    try{
      if(typeof row.payloadBase64!=='string' || row.payloadBase64.length>3<<20 ||
        !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(row.payloadBase64))throw Error('append_payload');
      payload=Buffer.from(row.payloadBase64,'base64');
      if(payload.length!==row.bytes || sha(payload)!==row.sha256)throw Error('append_payload_binding');
    }catch{healthy=false;}
    const expected=plan.appendPlans.find(v=>v.sha256===row.sha256 && v.bytes===row.bytes && payload?.equals(v.data));
    const buffer=value.buffers.find(v=>v.id===row.bufferID);
    const safe={ordinal,bufferID:row.bufferID,bytes:row.bytes,sha256:row.sha256,outcome:row.outcome,
      mode:row.mode,timestampOffset:row.timestampOffset,appendWindowStart:row.appendWindowStart,
      appendWindowEnd:row.appendWindowEnd,matched:!!expected,fragmentIndices:expected?.fragmentIndices,mime:row.mime};
    result.appends.push(safe);
    if(!expected || !buffer || typeof row.mime!=='string' ||
      !/^(?:video|audio)\/mp4;\s*codecs="(?:avc1\.64001e(?:,mp4a\.40\.2)?|mp4a\.40\.2)"$/.test(row.mime) || row.ordinal!==ordinal || row.outcome!=='updateend' || row.mode!=='segments' ||
      !Number.isFinite(row.timestampOffset) || !Number.isFinite(row.appendWindowStart) ||
      !(row.appendWindowEnd==='Infinity' || Number.isFinite(row.appendWindowEnd)))healthy=false;
    if(expected?.init){
      try{
        if(!Array.isArray(row.initBytes) || row.initBytes.length!==plan.init.length ||
          !row.initBytes.every(v=>Number.isInteger(v) && v>=0 && v<=255))throw Error('app_init_shape');
        const actual=Buffer.from(row.initBytes);
        if(!actual.equals(plan.init))throw Error('app_init_bytes');
        const facts=colorFacts(Buffer.concat([actual,...pieces.slice(1)]));
        result.actualInitFacts.push(facts);initCount++;initialized.add(row.bufferID);
        if(facts.avcSHA256!==plan.facts.avcSHA256 || facts.colr?.type!=='nclx' ||
          facts.colr.primaries!==6 || facts.colr.transfer!==6 || facts.colr.matrix!==6 ||
          facts.colr.fullRange!==0)healthy=false;
      }catch{healthy=false;}
    }else if(row.initBytes!==null)healthy=false;
    if(expected?.fragmentIndices.length){
      if(!initialized.has(row.bufferID))healthy=false;
      if(!sequences.has(row.bufferID))sequences.set(row.bufferID,[]);
      sequences.get(row.bufferID).push(...expected.fragmentIndices);
    }
  }
  result.bufferSequences=[...sequences].map(([bufferID,indices])=>({bufferID,indices}));
  result.qualified=healthy && initCount>0 && sequences.size>0 && [...sequences.values()].every(indices=>
    indices.length===pieces.length-1 && indices.every((v,n)=>v===n));
  return result;
}
export function installAppColorAppendObserver(){
  const state={buffers:[],appends:[],failures:[],pending:[],overflow:false,totalBytes:0};
  window.nonkeyAppColorAppend=state;
  const bufferIDs=new WeakMap(),nativeAdd=MediaSource.prototype.addSourceBuffer,
    nativeAppend=SourceBuffer.prototype.appendBuffer,nativeChange=SourceBuffer.prototype.changeType;
  const digest=async b=>Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',b)),
    v=>v.toString(16).padStart(2,'0')).join('');
  MediaSource.prototype.addSourceBuffer=function(...args){
    const buffer=Reflect.apply(nativeAdd,this,args);
    if(state.buffers.length>=8){state.overflow=true;return buffer;}
    const id=state.buffers.length;bufferIDs.set(buffer,id);state.buffers.push({id,mime:args[0]});return buffer;
  };
  if(nativeChange)SourceBuffer.prototype.changeType=function(...args){
    const value=Reflect.apply(nativeChange,this,args),id=bufferIDs.get(this);
    if(id===undefined)state.failures.push('unknown_change_buffer');else state.buffers[id].mime=args[0];
    return value;
  };
  SourceBuffer.prototype.appendBuffer=function(...args){
    let row,bytes,finish;
    if(state.appends.length>=128){state.overflow=true;return Reflect.apply(nativeAppend,this,args);}
    try{
      const input=args[0],view=input instanceof ArrayBuffer?new Uint8Array(input):
        new Uint8Array(input.buffer,input.byteOffset,input.byteLength);
      if(!view.length || view.length>2<<20 || state.totalBytes+view.length>2<<20)throw Error('bound');
      bytes=Uint8Array.from(view);state.totalBytes+=bytes.length;
      const bufferID=bufferIDs.get(this);
      row={ordinal:state.appends.length,bufferID,bytes:bytes.length,mime:state.buffers[bufferID]?.mime,
        mode:this.mode,timestampOffset:this.timestampOffset,appendWindowStart:this.appendWindowStart,
        appendWindowEnd:this.appendWindowEnd===Infinity?'Infinity':this.appendWindowEnd,outcome:'pending',initBytes:null};
      state.appends.push(row);
      const dv=new DataView(bytes.buffer);let first=0,seenMovie=false,seenFile=false;
      while(first<bytes.length){
        if(bytes.length-first<8)break;
        const size=dv.getUint32(first),kind=String.fromCharCode(...bytes.subarray(first+4,first+8));
        if(size<8 || size>bytes.length-first)break;
        if(['moof','mdat','styp','sidx'].includes(kind))break;
        seenMovie=seenMovie || kind==='moov';seenFile=seenFile || kind==='ftyp';first+=size;
      }
      if(seenMovie && seenFile){
        if(first>65536)throw Error('init_bound');row.initBytes=[...bytes.subarray(0,first)];
      }
      const end=()=>finish('updateend'),error=()=>finish('error'),abort=()=>finish('abort');
      finish=value=>{row.outcome=value;this.removeEventListener('updateend',end);
        this.removeEventListener('error',error);this.removeEventListener('abort',abort);};
      this.addEventListener('updateend',end);this.addEventListener('error',error);this.addEventListener('abort',abort);
    }catch{state.failures.push('append_observer');}
    try{return Reflect.apply(nativeAppend,this,args);}
    catch(error){if(finish)finish('exception');throw error;}
    finally{
      if(row && bytes)state.pending.push(Promise.resolve().then(async()=>{
        row.sha256=await digest(bytes);let text='';
        for(let first=0;first<bytes.length;first+=8192)text+=String.fromCharCode(...bytes.subarray(first,first+8192));
        row.payloadBase64=btoa(text);
      }).catch(()=>{state.failures.push('append_digest');}));
    }
  };
}
async function snapshotAppColor(){
  const state=window.nonkeyAppColorAppend;if(!state)return {failures:['missing_append_observer']};
  await Promise.all(state.pending);
  return {buffers:state.buffers,appends:state.appends,failures:state.failures,overflow:state.overflow};
}
export async function observeAppColor(context,origin,item,joined,reference,referenceComplete,retained,observe){
  let pieces,plan;
  try{
    pieces=[];
    const prefix=item.selectedSource.slice(0,item.selectedSource.lastIndexOf('/')+1);
    for(const [name,digest] of Object.entries(item.publicAssetSHA256)){
      if(!/^[1-9][0-9]{2,3}p\/(?:init\.mp4|[A-Za-z0-9_-]{1,80}\.m4s)$/.test(name))throw Error('app_color_asset_shape');
      const response=await context.request.get(origin+prefix+name,{timeout:15000}),bytes=await response.body();
      if(response.status()!==200 || !bytes.length || bytes.length>2<<20 || sha(bytes)!==digest)
        throw Error('app_color_asset_binding');
      pieces.push(bytes);
    }
    plan=prepareAppColor(item,joined,pieces);
  }catch(error){
    retained.push({request:item.request,label:'app-color-601-setup',result:'observation-failed',
      failureClass:error.message?.match(/^[a-z_]+$/)?.[0]||error.name||'app_color_setup',
      colorScope:'Retained setup failure; all original app/audio/raw-MSE observations precede this case'});
    return;
  }
  for(const label of ['app-color-601-unchanged-client','app-color-601-forced-source-coordinate-seek']){
    const routeFailures=[],routeHits=[],row={request:item.request,label,result:'observation-failed'};
    retained.push(row);
    const diagnostic={item:plan.item,install:async page=>{
      await page.addInitScript(installAppColorAppendObserver);
      await page.route(url=>url.origin===origin && url.pathname===plan.initPath,async route=>{
        const hit={ordinal:routeHits.length,requestShape:appColorRouteFacts(route.request().url(),origin,plan.initPath,
          route.request().method(),route.request().headers().range),fulfilled:false};
        if(routeHits.length>=32){routeFailures.push('init_route_overflow');await route.abort();return;}
        routeHits.push(hit);
        try{
          if(!hit.requestShape.qualified)throw Error('app_color_init_request');
          const original=await route.fetch({timeout:15000}),body=await original.body();
          hit.originalStatus=original.status();hit.originalBytes=body.length;hit.originalSHA256=sha(body);
          hit.requestShape=appColorRouteFacts(route.request().url(),origin,plan.initPath,
            route.request().method(),route.request().headers().range,original.headers()['x-playback-session']);
          if(!hit.requestShape.qualified || original.status()!==200 || body.length>65536 ||
            hit.originalSHA256!==sha(pieces[0]))throw Error('app_color_route_binding');
          await route.fulfill({status:200,contentType:'video/mp4',body:plan.init});
          hit.fulfilled=true;hit.fulfilledBytes=plan.init.length;hit.fulfilledSHA256=sha(plan.init);
        }catch{hit.failureClass='init_route_binding';routeFailures.push('init_route_binding');await route.abort();}
      });
    },snapshot:async page=>{const value=await page.evaluate(snapshotAppColor);value.failures.push(...routeFailures);value.routeHits=routeHits;return value;}};
    try{Object.assign(row,await observe(item.request,label,diagnostic));}
    catch(error){row.failureClass=error.message?.match(/^[a-z_]+$/)?.[0]||error.name||'app_color_observation';}
    row.referenceComplete=referenceComplete;row.colorMetadata=plan.facts;
    row.originalHTTPInitSHA256=sha(pieces[0]);row.counterfactualHTTPInitSHA256=sha(plan.init);
    row.colorScope='Actual unchanged app with page-scoped sealed601 init; no automatic-seek/color-policy/audio/native acceptance';
    row.appendWitness=appColorAppendFacts(row.actualAppColorAppend,plan,pieces);
    row.appends=row.appendWitness.appends;row.appendedMetadata=row.appendWitness.actualInitFacts;
    row.appendedBytesVerified=row.appendWitness.qualified;
    row.routeWitness=routeHits;
    row.independentPage601InitSeen=Boolean(row.observedSelectedAssets?.[Object.keys(plan.item.publicAssetSHA256)[0]]?.length &&
      row.observedSelectedAssets[Object.keys(plan.item.publicAssetSHA256)[0]].every(v=>v.status===200 &&
        v.bytes===plan.init.length && v.sha256===sha(plan.init)));
    row.actual601InitSeen=row.appendWitness.actualInitFacts.some(v=>v.joinedSHA256===plan.facts.joinedSHA256);
    row.routed601TransferObserved=appColorTransferObserved(routeHits,routeFailures,row.actualAppColorAppend,
      row.independentPage601InitSeen,row.actual601InitSeen,sha(pieces[0]),sha(plan.init));
    row.transferObservationScope='Exact routed601 init delivered and appended only; aggregate MIME/duplicate/frame oracles unchanged';
    row.colorComponentQualified=row.frameConsumerQualified===true && row.appendWitness.qualified;
  }
}

export function appColorConsumerQualification(row,reference,expected,complete){
  return consumerQualification({...row,referenceComplete:complete,
    label:row.label==='app-color-601-forced-source-coordinate-seek'?'forced-source-coordinate-seek':row.label},reference,expected);
}

export function appColorRouteFacts(url,origin,path,method,range,responseSession){
  const result={qualified:false,sameOrigin:false,selectedPathMatched:false,methodGET:method==='GET',
    rangePresent:range!==undefined,hasQuery:false,queryCount:0,unknownQueryKeyCount:0,
    queryKeyFlags:{playbackSession:false,playSessionId:false,start:false,ticket:false,api_key:false},
    responseSessionHeaderPresent:typeof responseSession==='string' && responseSession.length>0,
    responseSessionHeaderShapeMatched:false,querySessionHeaderMatched:null};
  try{
    if(typeof url!=='string' || url.length>2048 || typeof origin!=='string' || origin.length>256 ||
      typeof path!=='string' || path.length>512)throw Error('route_shape');
    const parsed=new URL(url),keys=[...parsed.searchParams.keys()];
    result.sameOrigin=parsed.origin===origin;result.selectedPathMatched=parsed.pathname===path;
    result.hasQuery=parsed.search.length>0;result.queryCount=keys.length;
    for(const key of keys){
      if(Object.hasOwn(result.queryKeyFlags,key))result.queryKeyFlags[key]=true;else result.unknownQueryKeyCount++;
    }
    const sessions=parsed.searchParams.getAll('playbackSession'),valid=v=>/^[A-Za-z0-9_-]{8,64}$/.test(v);
    const queryOK=keys.length===0 || keys.length===1 && keys[0]==='playbackSession' && sessions.length===1 && valid(sessions[0]);
    result.responseSessionHeaderShapeMatched=result.responseSessionHeaderPresent && valid(responseSession);
    if(result.responseSessionHeaderPresent && sessions.length===1)result.querySessionHeaderMatched=sessions[0]===responseSession;
    const headerOK=!result.responseSessionHeaderPresent || result.responseSessionHeaderShapeMatched &&
      (sessions.length===0 || result.querySessionHeaderMatched===true);
    result.qualified=result.sameOrigin && result.selectedPathMatched && result.methodGET && !result.rangePresent &&
      !parsed.username && !parsed.password && !parsed.hash && queryOK && headerOK;
  }catch{}
  return result;
}

export function appColorTransferObserved(hits,routeFailures,appendState,pageInitSeen,actualInitSeen,originalSHA,modifiedSHA){
  return Array.isArray(hits) && hits.length>0 && hits.length<=32 && Array.isArray(routeFailures) &&
    routeFailures.length===0 && appendState?.overflow===false && Array.isArray(appendState.failures) &&
    appendState.failures.length===0 && pageInitSeen===true && actualInitSeen===true &&
    /^[a-f0-9]{64}$/.test(originalSHA) && /^[a-f0-9]{64}$/.test(modifiedSHA) &&
    hits.every(v=>v.requestShape?.qualified===true && v.originalStatus===200 &&
      v.originalSHA256===originalSHA && v.fulfilled===true && v.fulfilledSHA256===modifiedSHA);
}
