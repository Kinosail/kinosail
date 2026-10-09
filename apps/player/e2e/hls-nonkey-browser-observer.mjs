// Strict observations; a diagnostic never grants production/native/cache admission.
export function frameQualification(facts, reference, expected) {
  const identities=new Map(reference.map((r,n)=>[r.sha256,n]));
  const actual=facts.rows.map(r=>identities.get(r.sha256) ?? null);
  const unique=identities.size===reference.length;
  const exact=unique && actual.length===expected.length && actual.every((v,n)=>v===expected[n]);
  return {qualified:Boolean(facts.ended && exact && facts.droppedCallbacks===0 && facts.captureErrors.length===0 &&
      !(facts.quality?.droppedVideoFrames>0) && !facts.events?.some(e=>e.name==='error' || e.errorCode>0)),
    exactRequestedSequence:exact, actualSourceIndices:actual, expectedSourceIndices:expected,
    allObserverRowsRetained:true, sourceReferenceUnique:unique, genuinePresentedEOF:facts.ended};
}

export function consumerQualification(row, reference, expected) {
  const phase=row.observer?.phases?.at(-1);
  if(!phase)return {qualified:false,reason:'missing_observer'};
  const frame=frameQualification(phase,reference,expected);
  const healthy=row.result==='observed' && !row.pageErrors && !row.snapshotFailure &&
    !row.httpOverflow && !row.httpBodyFailure && !row.publicAssetHashMismatch &&
    !row.directMediaRequested && !row.wrongHLSRecipe && !row.unexpectedSelectedAsset &&
    !row.selectedHLSError && row.selectedPublicHLSObserved===true;
  const actualQuality=phase.quality && phase.quality.droppedVideoFrames===0 &&
    phase.firstCallbackGap===0;
  const seek=row.label!=='forced-source-coordinate-seek' || row.forceSeek?.seeking && row.forceSeek?.seeked;
  return {...frame,qualified:Boolean(frame.qualified && healthy && actualQuality && seek && row.referenceComplete)};
}
export function completeAudioQualification(value, expected) {
  return {qualified:Boolean(value.completeDecode && value.sampleRate===48000 && value.channels===2 &&
    value.samples===expected.samples && value.s16leSHA256===expected.sha256),
    scope:'Whole AudioContext decode diagnostic; does not prove HTMLMediaElement audible presentation',
    wholeDecodedSamplesRetained:true, expectedSamples:expected.samples, actualSamples:value.samples};
}
export function installBrowserObserver() {
  const state={phases:[],active:undefined,captureErrors:[]};
  window.nonkeyObservation=state;
  const attach=video=>{
    if(video.nonkeyAttached)return;
    video.nonkeyAttached=true;
    video.playbackRate=0.5;
    const phase=label=>{
      const quality=video.getVideoPlaybackQuality?.();
      const value={label,rows:[],events:[],ended:false,droppedCallbacks:0,firstCallbackGap:0,captureErrors:[],pending:[],
        qualityAtStart:quality?{droppedVideoFrames:quality.droppedVideoFrames,totalVideoFrames:quality.totalVideoFrames}:null};
      state.phases.push(value);state.active=value;return value;
    };
    phase('initial-unchanged-client');
    window.nonkeyBeginPhase=phase;
    const rawTime=Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype,'currentTime').get;
    const rawDuration=Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype,'duration').get;
    for(const name of ['loadedmetadata','seeking','seeked','playing','pause','ended','error']){
      video.addEventListener(name,()=>{
        const value=state.active;
        if(value.events.length>=256){value.captureErrors.push('event_bound');return;}
        value.events.push({name,rawTime:rawTime.call(video),projectedTime:video.currentTime,
          rawDuration:rawDuration.call(video),projectedDuration:video.duration,errorCode:video.error?.code||0});
        if(name==='ended')value.ended=true;
      });
    }
    if(typeof video.requestVideoFrameCallback!=='function'){
      state.active.captureErrors.push('requestVideoFrameCallback_unavailable');return;
    }
    const canvas=document.createElement('canvas');
    const context=canvas.getContext('2d',{willReadFrequently:true});
    let previous;
    const capture=(_,metadata)=>{
      video.requestVideoFrameCallback(capture);
      const value=state.active;
      if(value.rows.length>=2048){value.captureErrors.push('frame_bound');video.pause();return;}
      const row={mediaTime:metadata.mediaTime,presentedFrames:metadata.presentedFrames,
        rawTime:rawTime.call(video),projectedTime:video.currentTime,width:video.videoWidth,height:video.videoHeight};
      if(previous===undefined && metadata.presentedFrames>1){
        value.firstCallbackGap=metadata.presentedFrames-1;value.droppedCallbacks+=metadata.presentedFrames-1;
      }
      if(previous!==undefined && metadata.presentedFrames>previous+1)value.droppedCallbacks+=metadata.presentedFrames-previous-1;
      previous=metadata.presentedFrames;
      value.rows.push(row);
      try{
        if(row.width!==640 || row.height!==360)throw Error('frame_dimensions');
        canvas.width=row.width;canvas.height=row.height;context.drawImage(video,0,0);
        const bytes=context.getImageData(0,0,row.width,row.height).data;
        value.pending.push(crypto.subtle.digest('SHA-256',bytes).then(digest=>{
          row.sha256=Array.from(new Uint8Array(digest),n=>n.toString(16).padStart(2,'0')).join('');
        }).catch(()=>value.captureErrors.push('frame_digest')));
      }catch(error){value.captureErrors.push(error.message==='frame_dimensions'?'frame_dimensions':'canvas_capture');}
    };
    video.requestVideoFrameCallback(capture);
  };
  new MutationObserver(()=>document.querySelectorAll('video').forEach(attach)).observe(document,{childList:true,subtree:true});
  document.querySelectorAll('video').forEach(attach);
  window.nonkeySnapshot=async()=>{
    const video=document.querySelector('video');
    const quality=video?.getVideoPlaybackQuality?.();
    if(state.active)state.active.quality=quality?{droppedVideoFrames:quality.droppedVideoFrames,totalVideoFrames:quality.totalVideoFrames}:null;
    for(const value of state.phases)await Promise.all(value.pending);
    return {phases:state.phases.map(({pending,...value})=>value),captureErrors:state.captureErrors};
  };
}
export async function browserAudioDecode(bytes) {
  const context=new AudioContext({sampleRate:48000});
  try{
    if(context.sampleRate!==48000)throw Error('native_48000_context_unavailable');
    const decoded=await context.decodeAudioData(Uint8Array.from(bytes).buffer);
    if(decoded.sampleRate!==48000 || decoded.numberOfChannels!==2 || decoded.length>1600000)
      throw Error('native_audio_shape');
    const channels=[decoded.getChannelData(0),decoded.getChannelData(1)];
    const raw=new Float32Array(decoded.length*2);
    const quantized=new DataView(new ArrayBuffer(decoded.length*4));
    let clipped=0;
    for(let n=0;n<decoded.length;n++)for(let c=0;c<2;c++){
      const sample=channels[c][n];raw[n*2+c]=sample;
      const integer=Math.round(sample*32768);
      if(integer<-32768||integer>32767)clipped++;
      quantized.setInt16((n*2+c)*2,Math.min(32767,Math.max(-32768,integer)),true);
    }
    const hash=async array=>Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',array)),
      n=>n.toString(16).padStart(2,'0')).join('');
    return {sampleRate:decoded.sampleRate,channels:decoded.numberOfChannels,samples:decoded.length,
      float32InterleavedSHA256:await hash(raw.buffer),s16leSHA256:await hash(quantized.buffer),
      quantization:'round(sample*32768), signed16 clipping; complete float32 hash also retained',
      clippedSamples:clipped,completeDecode:true,requestedAudioContextRate:48000,decodeAudioDataReturnsContextRate:true,
      browserResamplingMayApply:true,sourceRateVerifiedByCLI:48000,
      nativeDecoderEOFInstrumentation:false,htmlAudiblePresentationQualified:false};
  }finally{await context.close();}
}
