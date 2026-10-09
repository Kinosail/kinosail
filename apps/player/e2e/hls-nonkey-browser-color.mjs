// Isolated metadata counterfactual; never edits cached assets or coded media.
import {createHash} from 'node:crypto';
import {frameQualification} from './hls-nonkey-browser-observer.mjs';
const sha=b=>createHash('sha256').update(b).digest('hex');
const fail=()=>{throw Error('color_box_shape');};
function boxes(bytes,first,last){
  const result=[];
  while(first<last){
    if(result.length>=256 || last-first<8)fail();
    const size=bytes.readUInt32BE(first),kind=bytes.toString('ascii',first+4,first+8);
    if(size<8 || size>last-first || !/^[A-Za-z0-9 ]{4}$/.test(kind))fail();
    result.push({kind,first,last:first+size,size});first+=size;
  }
  return result;
}
function layout(bytes){
  if(!Buffer.isBuffer(bytes) || !bytes.length || bytes.length>8<<20)fail();
  const top=boxes(bytes,0,bytes.length),media=top.find(b=>['moof','mdat','styp','sidx'].includes(b.kind));
  if(!media || top.filter(b=>b.kind==='moov').length!==1 || top.filter(b=>b.kind==='ftyp').length!==1)fail();
  const initBytes=media.first,candidates=[];
  const walk=(parent,chain,depth)=>{
    if(depth>7)fail();
    let first=parent.first+8;
    if(parent.kind==='stsd'){
      if(parent.last-first<8 || bytes.readUInt32BE(first)!==0)fail();
      const count=bytes.readUInt32BE(first+4);first+=8;
      if(count!==boxes(bytes,first,parent.last).length)fail();
    }
    for(const child of boxes(bytes,first,parent.last)){
      const next=[...chain,child];
      if(child.kind==='avc1')candidates.push(next);
      else if(['trak','mdia','minf','stbl','stsd'].includes(child.kind))walk(child,next,depth+1);
    }
  };
  const movie=top.find(b=>b.kind==='moov');if(movie.last>initBytes)fail();
  walk(movie,[movie],0);
  if(candidates.length!==1)fail();
  const chain=candidates[0],entry=chain.at(-1);
  if(entry.size<86)fail();
  const children=boxes(bytes,entry.first+86,entry.last),avc=children.filter(b=>b.kind==='avcC');
  const color=children.filter(b=>b.kind==='colr');
  if(avc.length!==1 || avc[0].size<15 || avc[0].size>4104 || color.length>1)fail();
  let colr=null;
  if(color.length){
    const b=color[0],start=b.first+8;
    if(b.size!==19 || bytes.toString('ascii',start,start+4)!=='nclx' || (bytes[start+10]&127)!==0)fail();
    colr={type:'nclx',primaries:bytes.readUInt16BE(start+4),transfer:bytes.readUInt16BE(start+6),
      matrix:bytes.readUInt16BE(start+8),fullRange:bytes[start+10]>>7};
  }
  return {chain,entry,color,initBytes,colr,avc};
}
export function colorFacts(bytes){
  const v=layout(bytes);
  return {initBytes:v.initBytes,colr:v.colr,avcSHA256:sha(bytes.subarray(v.avc[0].first,v.avc[0].last)),
    fragmentSHA256:sha(bytes.subarray(v.initBytes)),joinedSHA256:sha(bytes)};
}
export function colorArm(input,arm){
  if(!['original','601','709'].includes(arm))throw Error('color_arm_shape');
  const v=layout(input);if(v.color.length)throw Error('color_original_already_specified');
  const bytes=Buffer.from(input),original=colorFacts(input);
  if(arm==='original')return {bytes,facts:{...original,inverseOriginalSHA256:sha(input),metadataByteIdentity:true}};
  const n=arm==='601'?6:1,colr=Buffer.alloc(19);
  colr.writeUInt32BE(19);colr.write('colr',4);colr.write('nclx',8);
  for(const offset of [12,14,16])colr.writeUInt16BE(n,offset);
  const output=Buffer.concat([bytes.subarray(0,v.entry.last),colr,bytes.subarray(v.entry.last)]);
  for(const b of v.chain)output.writeUInt32BE(b.size+19,b.first);
  const actual=layout(output),facts=colorFacts(output);
  const inverse=Buffer.concat([output.subarray(0,actual.color[0].first),output.subarray(actual.color[0].last)]);
  for(const b of v.chain)inverse.writeUInt32BE(b.size,b.first);
  if(!inverse.equals(input) || facts.avcSHA256!==original.avcSHA256 ||
    facts.fragmentSHA256!==original.fragmentSHA256)throw Error('color_inverse_identity');
  return {bytes:output,facts:{...facts,inverseOriginalSHA256:sha(inverse),metadataByteIdentity:true}};
}
export function colorFrameQualification(row,reference,expected){
  const phase=row.observer?.phases?.at(-1);
  if(!phase)return {qualified:false,reason:'missing_observer'};
  const frame=frameQualification(phase,reference,expected);
  const seek=row.forceSeek?.seeking && row.forceSeek?.seeked && Number.isFinite(row.forceSeek?.rawTime) &&
    Math.abs(row.forceSeek.rawTime-row.requestedSource)<=0.000001;
  const config=row.appendConfiguration,clock=row.rawMSEClockFacts;
  const configured=Number.isFinite(row.requestedSource) && row.requestedSource===12.5 &&
    config?.mode==='segments' && config.appendWindowStart===0 && config.appendWindowEnd==='Infinity' &&
    config.timestampOffset===row.requestedSource && clock?.qualified===true &&
    clock.timestampOffset===row.requestedSource && clock.firstClipPTSSeconds===-0.5 && clock.firstSourcePTSSeconds===12;
  const healthy=configured && row.endOfStreamReturned===true && row.result==='observed' && row.referenceComplete===true && row.publicVideoSuffixQualified===true &&
    row.deliveredBytesVerified===true && row.metadataByteIdentity===true && row.appendedBytesVerified===true &&
    row.adapter?.rawMSE===true && !row.pageErrors && !row.snapshotFailure && row.appendFailures?.length===0 &&
    phase.firstCallbackGap===0 && phase.quality?.droppedVideoFrames===0;
  return {...frame,qualified:Boolean(frame.qualified && healthy && seek),
    scope:'Isolated raw MSE video component; no app, audio, native or production acceptance'};
}
