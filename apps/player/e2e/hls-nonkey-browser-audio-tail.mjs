// Whole browser-returned PCM correspondence; never trims buffers or certifies media clocks.
export function installAudioTailMatcher(){
  window.nonkeyAudioTailMatch=(source,publicPCM,nominal)=>{
    const valid=v=>Array.isArray(v) && v.length===2 && v.every(c=>c instanceof Float32Array &&
      c.length>0 && c.length<=1600000) && v[0].length===v[1].length;
    if(!valid(source) || !valid(publicPCM) || !Number.isInteger(nominal) || nominal<0 || nominal>1600000)
      throw Error('audio_tail_shape');
    for(const channels of [source,publicPCM])for(const channel of channels)
      for(const sample of channel)if(!Number.isFinite(sample))throw Error('audio_tail_shape');
    const sourceBits=source.map(v=>new Uint32Array(v.buffer,v.byteOffset,v.length)),
      publicBits=publicPCM.map(v=>new Uint32Array(v.buffer,v.byteOffset,v.length));
    const quantize=value=>Math.min(32767,Math.max(-32768,Math.round(value*32768)));
    const candidates=[],exactOffsets=[];
    for(let offset=nominal-64;offset<=nominal+64;offset++){
      const row={offset,inRange:offset>=0 && offset+publicPCM[0].length<=source[0].length,
        floatBitMismatches:null,s16Mismatches:null,firstFloatMismatches:[],firstS16Mismatches:[]};
      candidates.push(row);if(!row.inRange)continue;
      row.floatBitMismatches=0;row.s16Mismatches=0;
      for(let n=0;n<publicPCM[0].length;n++)for(let c=0;c<2;c++){
        if(sourceBits[c][offset+n]!==publicBits[c][n]){
          row.floatBitMismatches++;if(row.firstFloatMismatches.length<8)row.firstFloatMismatches.push({sample:n,channel:c});
        }
        if(quantize(source[c][offset+n])!==quantize(publicPCM[c][n])){
          row.s16Mismatches++;if(row.firstS16Mismatches.length<8)row.firstS16Mismatches.push({sample:n,channel:c});
        }
      }
      if(row.floatBitMismatches===0 && row.s16Mismatches===0)exactOffsets.push(offset);
    }
    const whole=exactOffsets.length===1 && exactOffsets[0]+publicPCM[0].length===source[0].length;
    return {qualified:whole,nominalOffset:nominal,candidateRadius:64,candidates,exactOffsets,
      sourceSamples:source[0].length,publicSamples:publicPCM[0].length,wholeReturnedEOFTail:whole,
      everyUntouchedPublicStereoSampleCompared:true,publicSamplesTrimmed:0,
      correspondenceScope:'Returned AudioContext buffers only; no source-clock/edit/decoder-EOF/audible acceptance'};
  };
  window.nonkeyAudioTailQualification=(tail,first,repeat,pub,oldSource,oldPublic,codedBinding)=>{
    const equal=(a,b)=>a && b && a.completeDecode===true && b.completeDecode===true &&
      a.sampleRate===48000 && b.sampleRate===48000 && a.channels===2 && b.channels===2 &&
      a.samples===b.samples && /^[a-f0-9]{64}$/.test(a.float32InterleavedSHA256||'') &&
      /^[a-f0-9]{64}$/.test(a.s16leSHA256||'') &&
      a.float32InterleavedSHA256===b.float32InterleavedSHA256 && a.s16leSHA256===b.s16leSHA256;
    const sourceRepeatMatched=Boolean(equal(first,repeat)),historicalSourceMatched=Boolean(equal(first,oldSource)),
      historicalPublicMatched=Boolean(equal(pub,oldPublic));
    return {qualified:Boolean(tail?.qualified && codedBinding===true && sourceRepeatMatched &&
      historicalSourceMatched && historicalPublicMatched && tail.sourceSamples===first?.samples &&
      tail.publicSamples===pub?.samples),sourceRepeatMatched,historicalSourceMatched,historicalPublicMatched,
      codedInputBytesVerified:codedBinding===true,
      scope:'Whole returned PCM tail only; FFmpeg count/hash failures remain unchanged'};
  };
}
export async function browserAudioTail(input){
  const shape=(v,bound)=>Array.isArray(v) && v.length>0 && v.length<=bound &&
    v.every(n=>Number.isInteger(n) && n>=0 && n<=255);
  if(!shape(input.sourceBytes,2<<20) || !shape(input.publicBytes,8<<20) ||
    !/^[a-f0-9]{64}$/.test(input.sourceSHA256) || !/^[a-f0-9]{64}$/.test(input.publicSHA256) ||
    typeof window.nonkeyAudioTailMatch!=='function')throw Error('audio_tail_input_shape');
  const hash=async bytes=>Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes)),
    n=>n.toString(16).padStart(2,'0')).join('');
  const sourceBytes=Uint8Array.from(input.sourceBytes),publicBytes=Uint8Array.from(input.publicBytes);
  const sourceSHA256=await hash(sourceBytes),publicSHA256=await hash(publicBytes);
  if(sourceSHA256!==input.sourceSHA256 || publicSHA256!==input.publicSHA256)throw Error('audio_tail_input_binding');
  const context=new AudioContext({sampleRate:48000});
  async function decode(bytes){
    const value=await context.decodeAudioData(bytes.slice().buffer);
    if(context.sampleRate!==48000 || value.sampleRate!==48000 || value.numberOfChannels!==2 ||
      value.length<=0 || value.length>1600000)throw Error('audio_tail_decode_shape');
    const channels=[value.getChannelData(0),value.getChannelData(1)];
    const raw=new DataView(new ArrayBuffer(value.length*8)),s16=new DataView(new ArrayBuffer(value.length*4));
    let clippedSamples=0;
    for(let n=0;n<value.length;n++)for(let c=0;c<2;c++){
      const sample=channels[c][n];if(!Number.isFinite(sample))throw Error('audio_tail_nonfinite');
      raw.setFloat32((n*2+c)*4,sample,true);
      const integer=Math.round(sample*32768);if(integer<-32768 || integer>32767)clippedSamples++;
      s16.setInt16((n*2+c)*2,Math.min(32767,Math.max(-32768,integer)),true);
    }
    const chunk=async(first,last)=>({firstSample:first,samples:last-first,
      float32InterleavedSHA256:await hash(raw.buffer.slice(first*8,last*8)),
      s16leSHA256:await hash(s16.buffer.slice(first*4,last*4))});
    const facts={sampleRate:48000,channels:2,samples:value.length,completeDecode:true,clippedSamples,
      float32InterleavedSHA256:await hash(raw.buffer),s16leSHA256:await hash(s16.buffer),
      first1024:await chunk(0,Math.min(1024,value.length)),
      last1024:await chunk(Math.max(0,value.length-1024),value.length),
      nativeDecoderEOFInstrumentation:false,htmlAudiblePresentationQualified:false};
    return {facts,channels};
  }
  try{
    const source=await decode(sourceBytes),repeat=await decode(sourceBytes),pub=await decode(publicBytes);
    const correspondence=window.nonkeyAudioTailMatch(source.channels,pub.channels,600000);
    const qualification=window.nonkeyAudioTailQualification(correspondence,source.facts,repeat.facts,pub.facts,
      input.existingSource,input.existingPublic,true);
    return {sourceSHA256,publicSHA256,sourceFirst:source.facts,sourceRepeat:repeat.facts,publicPCM:pub.facts,
      correspondence,qualification,nominalSourceSampleOrdinal:600000,
      nominalBasis:'12.5 seconds at48000; ordinal search only, no measured source-clock claim',
      sourceClockQualified:false,missingFFmpegSampleLocationEstablished:false,
      nativeDecoderEOFInstrumentation:false,htmlAudiblePresentationQualified:false,productionAcceptance:false};
  }finally{await context.close();}
}
export async function observeAudioTail(context,origin,item,sourceBytes,joined,oldSource,oldPublic,retained){
  const row={request:item.request,label:'whole-browser-returned-audio-tail',result:'observation-failed',
    scope:'Diagnostic correspondence only; original source/public PCM failures remain retained'};
  retained.push(row);let page;
  try{
    page=await context.newPage();await page.goto(origin+'/healthz',{waitUntil:'domcontentloaded',timeout:15000});
    await page.evaluate(installAudioTailMatcher);
    row.facts=await page.evaluate(browserAudioTail,{sourceBytes:[...sourceBytes],publicBytes:[...joined],
      sourceSHA256:item.sourceSHA256,publicSHA256:item.publicJoinedSHA256,existingSource:oldSource,existingPublic:oldPublic});
    row.result='observed';row.correspondenceQualified=row.facts.qualification.qualified;
  }catch(error){row.failureClass=error.message?.match(/^[a-z_]+$/)?.[0]||error.name||'audio_tail_observation';}
  finally{if(page)await page.close();}
  return row;
}
