// Raw MSE decoder counterfactual; original public assets and app cases remain untouched.
import {createHash} from 'node:crypto';
import {colorArm,colorFacts,colorFrameQualification} from './hls-nonkey-browser-color.mjs';
const sha=b=>createHash('sha256').update(b).digest('hex');
async function delivered(context,origin,item,joined){
  const entries=Object.entries(item.publicAssetSHA256);
  if(entries.length<2 || entries.length>33 || !entries[0][0].endsWith('/init.mp4'))throw Error('color_public_asset_shape');
  const prefix=item.selectedSource.slice(0,item.selectedSource.lastIndexOf('/')+1),pieces=[];
  for(const [name,digest] of entries){
    if(!/^[1-9][0-9]{2,3}p\/(?:init\.mp4|[A-Za-z0-9_-]{1,80}\.m4s)$/.test(name) ||
      !/^[a-f0-9]{64}$/.test(digest))throw Error('color_public_asset_shape');
    const response=await context.request.get(origin+prefix+name,{timeout:15000}),bytes=await response.body();
    if(response.status()!==200 || !bytes.length || bytes.length>2<<20 || sha(bytes)!==digest)
      throw Error('color_public_asset_binding');
    pieces.push(bytes);
  }
  const bytes=Buffer.concat(pieces);
  if(bytes.length>8<<20 || !bytes.equals(joined) || sha(bytes)!==item.publicJoinedSHA256)
    throw Error('color_delivered_join_binding');
  return pieces;
}
async function installMSE(input){
  const video=document.createElement('video');video.controls=true;document.body.append(video);
  await Promise.resolve();
  const media=new MediaSource();window.nonkeyColorMedia=media;
  window.nonkeyColorURL=URL.createObjectURL(media);video.src=window.nonkeyColorURL;
  const state={appends:[],failures:[],initBytes:null};window.nonkeyColorAppend=state;
  const digest=async b=>Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',b)),
    v=>v.toString(16).padStart(2,'0')).join('');
  const opened=await new Promise(resolve=>{
    const timer=setTimeout(()=>resolve(false),10000);
    media.addEventListener('sourceopen',()=>{clearTimeout(timer);resolve(true);},{once:true});
  });
  if(!opened)throw Error('color_sourceopen_timeout');
  const mime='video/mp4; codecs="avc1.64001e,mp4a.40.2"';
  if(!MediaSource.isTypeSupported(mime))throw Error('color_combined_mse_unavailable');
  const buffer=media.addSourceBuffer(mime);window.nonkeyColorBuffer=buffer;
  buffer.timestampOffset=input.request;
  state.configuration={mode:buffer.mode,appendWindowStart:buffer.appendWindowStart,
    appendWindowEnd:buffer.appendWindowEnd===Infinity?'Infinity':buffer.appendWindowEnd,
    timestampOffset:buffer.timestampOffset,mime};
  if(buffer.mode!=='segments' || buffer.appendWindowStart!==0 || buffer.appendWindowEnd!==Infinity ||
    buffer.timestampOffset!==input.request)throw Error('color_append_configuration');
  for(const [index,piece] of input.pieces.entries()){
    const bytes=Uint8Array.from(piece.bytes);
    const row={index,bytes:bytes.length,sha256:await digest(bytes),expectedSHA256:piece.sha256};
    state.appends.push(row);
    if(row.sha256!==row.expectedSHA256)throw Error('color_append_byte_binding');
    if(index===0){if(bytes.length>65536)throw Error('color_appended_init_bound');state.initBytes=[...bytes];}
    const outcome=await new Promise(resolve=>{
      const finish=value=>{clearTimeout(timer);buffer.removeEventListener('updateend',end);
        buffer.removeEventListener('error',error);buffer.removeEventListener('abort',abort);resolve(value);};
      const end=()=>finish('updateend'),error=()=>finish('error'),abort=()=>finish('abort');
      const timer=setTimeout(()=>finish('timeout'),10000);
      buffer.addEventListener('updateend',end);buffer.addEventListener('error',error);buffer.addEventListener('abort',abort);
      try{buffer.appendBuffer(bytes);}catch{finish('exception');}
    });
    row.outcome=outcome;
    if(outcome!=='updateend'){state.failures.push(outcome);throw Error('color_append_failed');}
  }
  media.endOfStream();state.endOfStreamReturned=true;state.mediaSourceState=media.readyState;
  const button=document.createElement('button');button.id='nonkey-color-play';button.textContent='Play diagnostic';
  button.onclick=()=>video.play().catch(()=>{window.nonkeyColorPlayFailed=true;});document.body.append(button);
}
export async function observeMSEColors(context,origin,item,joined,reference,referenceComplete,retained){
  let original;
  try{
  if(!item.rawMSEClockFacts?.qualified || item.rawMSEClockFacts.timestampOffset!==item.request ||
    item.rawMSEClockFacts.firstClipPTSSeconds!==-0.5 || item.rawMSEClockFacts.firstSourcePTSSeconds!==12)
    throw Error('color_raw_clock_binding');
  original=await delivered(context,origin,item,joined);
  }catch(error){
    retained.push({request:item.request,label:'raw-mse-color-setup',result:'observation-failed',
      failureClass:error.message?.match(/^[a-z_]+$/)?.[0]||error.name||'color_setup_operation',
      colorScope:'Retained isolated component setup failure; original cases remain unchanged'});
    return;
  }
  for(const arm of ['original','601','709']){
    const row={request:item.request,requestedSource:item.request,label:'raw-mse-color-'+arm,result:'observation-failed',
      referenceComplete,publicVideoSuffixQualified:item.publicVideoSuffixQualified===true,deliveredBytesVerified:true,
      rawMSEClockFacts:item.rawMSEClockFacts,appendFailures:[],colorScope:'Isolated raw-MSE metadata counterfactual; no application or audible-audio acceptance'};
    retained.push(row);
    let page;
    try{
    const generated=colorArm(joined,arm);row.colorMetadata=generated.facts;
    row.metadataByteIdentity=generated.facts.metadataByteIdentity;
    if(colorFacts(joined).initBytes!==original[0].length)throw Error('color_init_extent_binding');
    const pieces=[generated.bytes.subarray(0,generated.facts.initBytes),...original.slice(1)];
    if(!Buffer.concat(pieces).equals(generated.bytes))throw Error('color_generated_join_binding');
    row.generatedSHA256=sha(generated.bytes);
    page=await context.newPage();page.on('pageerror',()=>{row.pageErrors=(row.pageErrors||0)+1;});
      await page.goto(origin+'/healthz',{waitUntil:'domcontentloaded',timeout:15000});
      row.currentStage='raw-mse-sealed-append';
      await page.evaluate(installMSE,{request:item.request,pieces:pieces.map(bytes=>({bytes:[...bytes],sha256:sha(bytes)}))});
      const append=await page.evaluate(()=>window.nonkeyColorAppend);
      row.appends=append.appends;row.appendConfiguration=append.configuration;
      row.endOfStreamReturned=append.endOfStreamReturned;row.mediaSourceState=append.mediaSourceState;
      row.appendedMetadata=colorFacts(Buffer.concat([Buffer.from(append.initBytes),...original.slice(1)]));
      row.appendedBytesVerified=append.appends.length===pieces.length && append.appends.every((v,n)=>
        v.index===n && v.bytes===pieces[n].length && v.sha256===sha(pieces[n]) && v.outcome==='updateend') &&
        row.appendedMetadata.joinedSHA256===generated.facts.joinedSHA256;
      if(!row.appendedBytesVerified)throw Error('color_actual_append_binding');
      await page.waitForFunction(()=>document.querySelector('video')?.readyState>=2,{},{timeout:20000});
      row.beforeSeek=await page.locator('video').evaluate(v=>({rawTime:v.currentTime,rawDuration:v.duration,
        buffered:Array.from({length:v.buffered.length},(_,n)=>[v.buffered.start(n),v.buffered.end(n)]),
        seekable:Array.from({length:v.seekable.length},(_,n)=>[v.seekable.start(n),v.seekable.end(n)])}));
      row.forceSeek=await page.evaluate(async target=>{
        const video=document.querySelector('video');window.nonkeyBeginPhase('raw-mse-explicit-source-seek');
        let seeking=false,seeked=false;
        const first=()=>{seeking=true;},last=()=>{seeked=true;};
        video.addEventListener('seeking',first);video.addEventListener('seeked',last);
        try{await new Promise(resolve=>{const timer=setTimeout(resolve,5000);
          video.addEventListener('seeked',()=>{clearTimeout(timer);resolve();},{once:true});video.currentTime=target;});}
        finally{video.removeEventListener('seeking',first);video.removeEventListener('seeked',last);}
        return {requestedSourceCoordinate:target,seeking,seeked,rawTime:video.currentTime,diagnosticOnly:true};
      },item.request);
      row.currentStage='raw-mse-complete-presented-EOF';
      await page.locator('#nonkey-color-play').click({timeout:10000});
      await page.waitForFunction(()=>window.nonkeyColorPlayFailed || window.nonkeyObservation?.active?.ended,{},{timeout:85000});
      if(await page.evaluate(()=>window.nonkeyColorPlayFailed===true))throw Error('color_play_failed');
      row.adapter=await page.locator('video').evaluate(v=>({rawMSE:true,rawTime:v.currentTime,rawDuration:v.duration,mediaSourceState:window.nonkeyColorMedia.readyState,
        buffered:Array.from({length:v.buffered.length},(_,n)=>[v.buffered.start(n),v.buffered.end(n)]),
        seekable:Array.from({length:v.seekable.length},(_,n)=>[v.seekable.start(n),v.seekable.end(n)])}));
      row.mediaSourceState=row.adapter.mediaSourceState;row.result='observed';row.currentStage='complete';
    }catch(error){row.failureClass=error.message?.match(/^[a-z_]+$/)?.[0]||error.name||'color_mse_operation';}
    finally{
      if(page){
      try{row.observer=await page.evaluate(()=>window.nonkeySnapshot());}catch{row.snapshotFailure=true;}
      try{
        const partial=await page.evaluate(()=>window.nonkeyColorAppend);
        if(partial){row.appends??=partial.appends;row.appendConfiguration??=partial.configuration;
          row.appendFailures=partial.failures;row.endOfStreamReturned??=partial.endOfStreamReturned;}
      }catch{row.appendFailures.push('append_snapshot_failed');}
      try{await page.evaluate(()=>{document.querySelector('video')?.pause();
        if(window.nonkeyColorURL)URL.revokeObjectURL(window.nonkeyColorURL);});}
      finally{await page.close();}
      }
    }
    row.frameQualification=colorFrameQualification(row,reference,item.expectedSourceIndices);
    row.frameConsumerQualified=row.frameQualification.qualified;
  }
}
