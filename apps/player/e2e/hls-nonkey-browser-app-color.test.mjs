// Written-first actual-app counterfactual failure matrix; no product source or color policy.
// A patched HTTP init alone cannot establish actual SourceBuffer bytes or rendered frames.
// Missing/unmatched/failed appends, wrong AVC/color, source substitution or incomplete coverage stay red.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {prepareAppColor,appColorAppendFacts,observeAppColor,installAppColorAppendObserver,appColorConsumerQualification} from './hls-nonkey-browser-app-color.mjs';
const sha=b=>createHash('sha256').update(b).digest('hex');
const box=(kind,...parts)=>{const body=Buffer.concat(parts),out=Buffer.alloc(body.length+8);
  out.writeUInt32BE(out.length);out.write(kind,4,4,'ascii');body.copy(out,8);return out;};
const fragment=n=>Buffer.concat([box('moof',Buffer.from([n])),box('mdat',Buffer.from([n+1]))]);
const init=Buffer.concat([box('ftyp',Buffer.from('isom')),box('moov',box('trak',box('mdia',box('minf',
  box('stbl',box('stsd',Buffer.from([0,0,0,0,0,0,0,1]),
    box('avc1',Buffer.alloc(78),box('avcC',Buffer.from([1,100,0,30,255,225,0])))))))))]);
const pieces=[init,fragment(1),fragment(3)],joined=Buffer.concat(pieces);
const item={request:12.5,selectedSource:'/hls/held/index.m3u8',publicJoinedSHA256:sha(joined),
  publicAssetSHA256:{'360p/init.mp4':sha(init),'360p/segment-00000.m4s':sha(pieces[1]),'360p/segment-00001.m4s':sha(pieces[2])}};
function witness(plan){
  return {failures:[],overflow:false,buffers:[{id:0,mime:'video/mp4; codecs="avc1.64001e,mp4a.40.2"'}],
    appends:plan.appendPlans.filter(v=>!v.combined).map((v,n)=>({ordinal:n,bufferID:0,bytes:v.bytes,
      sha256:v.sha256,payloadBase64:v.data.toString('base64'),mime:'video/mp4; codecs="avc1.64001e,mp4a.40.2"',outcome:'updateend',mode:'segments',timestampOffset:12.5,appendWindowStart:0,
      appendWindowEnd:'Infinity',initBytes:v.init?[...plan.init]:null}))};
}
test('counterfactual is per-case only and preserves every original coded fragment',()=>{
  const before=Buffer.from(joined),plan=prepareAppColor(item,joined,pieces);
  assert.equal(joined.equals(before),true);assert.equal(item.publicAssetSHA256['360p/init.mp4'],sha(init));
  assert.equal(plan.item.publicAssetSHA256['360p/init.mp4'],sha(plan.init));
  assert.equal(plan.item.publicAssetSHA256['360p/segment-00000.m4s'],sha(pieces[1]));
  assert.equal(plan.facts.inverseOriginalSHA256,sha(joined));
  assert.deepEqual(plan.facts.colr,{type:'nclx',primaries:6,transfer:6,matrix:6,fullRange:0});
  assert.equal(plan.initPath,'/hls/held/360p/init.mp4');
  for(const bad of [{...item,publicJoinedSHA256:'0'.repeat(64)},
    {...item,publicAssetSHA256:{...item.publicAssetSHA256,'360p/segment-00000.m4s':'0'.repeat(64)}},
    {...item,request:12},{...item,selectedSource:'/media/source.mp4'}])
    assert.throws(()=>prepareAppColor(bad,joined,pieces));
});
test('only actual exact init and complete ordered known native append outcomes qualify',()=>{
  const plan=prepareAppColor(item,joined,pieces),value=witness(plan);
  assert.equal(appColorAppendFacts(value,plan,pieces).qualified,true);
  for(const bad of [{...value,overflow:true},{...value,failures:['hook']},{...value,appends:[]},
    {...value,appends:value.appends.slice(0,-1)},{...value,appends:[value.appends[0],value.appends[2],value.appends[1]]},
    {...value,appends:[...value.appends,value.appends[1]]}])
    assert.equal(appColorAppendFacts(bad,plan,pieces).qualified,false);
  for(const bad of [{sha256:'0'.repeat(64)},{bytes:0},{outcome:'error'},{mode:'sequence'},
    {timestampOffset:NaN},{initBytes:[...init]},{initBytes:null},{bufferID:9},{mime:'text/plain'},{payloadBase64:'%%%'}])
    assert.equal(appColorAppendFacts({...value,appends:[{...value.appends[0],...bad},...value.appends.slice(1)]},plan,pieces).qualified,false);
});
test('combined init and first media append still binds the exact counterfactual and every cut',()=>{
  const plan=prepareAppColor(item,joined,pieces),value=witness(plan),combined=plan.appendPlans.find(v=>v.combined);
  value.appends=[{...value.appends[0],sha256:combined.sha256,bytes:combined.bytes,payloadBase64:combined.data.toString('base64')},{...value.appends[2],ordinal:1}];
  assert.equal(appColorAppendFacts(value,plan,pieces).qualified,true);
});
test('setup failures retain new failure evidence after original cases without invoking app playback',async()=>{
  const old={label:'original-audio',qualification:{qualified:false}},rows=[old];let calls=0;
  await observeAppColor(null,'',item,joined,[],true,rows,async()=>{calls++;});
  assert.equal(rows[0],old);assert.equal(rows.length,2);assert.equal(calls,0);
  assert.equal(rows[1].label,'app-color-601-setup');assert.equal(rows[1].result,'observation-failed');
});

test('native wrappers preserve offset views, receiver, return values and native exception identity',async()=>{
  const saved={window:globalThis.window,MediaSource:globalThis.MediaSource,SourceBuffer:globalThis.SourceBuffer};
  const nativeFailure=Error('native-sentinel'),calls=[];
  class FakeBuffer extends EventTarget{
    mode='segments';timestampOffset=12.5;appendWindowStart=0;appendWindowEnd=Infinity;
    appendBuffer(...args){calls.push({receiver:this,args});if(this.throwNext)throw nativeFailure;return 73;}
    changeType(...args){calls.push({changeReceiver:this,args});return 91;}
  }
  class FakeMedia{addSourceBuffer(...args){calls.push({mediaReceiver:this,args});return new FakeBuffer();}}
  try{
    globalThis.window={};globalThis.MediaSource=FakeMedia;globalThis.SourceBuffer=FakeBuffer;
    installAppColorAppendObserver();
    const media=new FakeMedia(),buffer=media.addSourceBuffer('video/mp4; codecs="avc1.64001e,mp4a.40.2"','extra');
    const full=Uint8Array.from([9,9,...fragment(1),9]),view=new DataView(full.buffer,2,full.length-3);
    assert.equal(buffer.appendBuffer(view,'extra'),73);
    buffer.dispatchEvent(new Event('updateend'));
    assert.equal(calls[1].receiver,buffer);assert.equal(calls[1].args[0],view);assert.equal(calls[1].args[1],'extra');
    assert.equal(buffer.changeType('audio/mp4; codecs="mp4a.40.2"','extra'),91);
    buffer.throwNext=true;assert.throws(()=>buffer.appendBuffer(view),error=>error===nativeFailure);
    await Promise.all(window.nonkeyAppColorAppend.pending);
    const rows=window.nonkeyAppColorAppend.appends;
    assert.equal(rows[0].sha256,sha(Buffer.from(view.buffer,view.byteOffset,view.byteLength)));
    assert.equal(rows[0].payloadBase64,Buffer.from(view.buffer,view.byteOffset,view.byteLength).toString('base64'));
    assert.equal(rows[0].outcome,'updateend');assert.equal(rows[1].outcome,'exception');
    assert.equal(rows[1].mime,'audio/mp4; codecs="mp4a.40.2"');
    window.nonkeyAppColorAppend.appends.length=128;
    const before=calls.length;assert.throws(()=>buffer.appendBuffer(view),error=>error===nativeFailure);
    assert.equal(calls.length,before+1);
  }finally{Object.assign(globalThis,saved);}
});
test('error or abort followed by updateend cannot qualify and buffers never share outcome state',async()=>{
  const saved={window:globalThis.window,MediaSource:globalThis.MediaSource,SourceBuffer:globalThis.SourceBuffer};
  class FakeBuffer extends EventTarget{
    mode='segments';timestampOffset=12.5;appendWindowStart=0;appendWindowEnd=Infinity;
    appendBuffer(){return undefined;}changeType(){}
  }
  class FakeMedia{addSourceBuffer(){return new FakeBuffer();}}
  try{
    globalThis.window={};globalThis.MediaSource=FakeMedia;globalThis.SourceBuffer=FakeBuffer;
    installAppColorAppendObserver();
    const media=new FakeMedia(),one=media.addSourceBuffer('video/mp4; codecs="avc1.64001e"'),
      two=media.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
    one.appendBuffer(fragment(1));two.appendBuffer(fragment(3));
    one.dispatchEvent(new Event('error'));one.dispatchEvent(new Event('updateend'));
    assert.equal(window.nonkeyAppColorAppend.appends[1].outcome,'pending');
    two.dispatchEvent(new Event('abort'));two.dispatchEvent(new Event('updateend'));
    await Promise.all(window.nonkeyAppColorAppend.pending);
    assert.deepEqual(window.nonkeyAppColorAppend.appends.map(v=>v.outcome),['error','abort']);
  }finally{Object.assign(globalThis,saved);}
});

test('contiguous media append groups preserve complete order without equating HTTP cuts to calls',()=>{
  const plan=prepareAppColor(item,joined,pieces),value=witness(plan),
    group=plan.appendPlans.find(v=>!v.init && v.fragmentIndices.length===2);
  value.appends=[value.appends[0],{...value.appends[1],sha256:group.sha256,bytes:group.bytes,
    payloadBase64:group.data.toString('base64')}];
  assert.equal(appColorAppendFacts(value,plan,pieces).qualified,true);
});

test('new actual-app qualification binds completeness and forced-seek semantics before evaluation',()=>{
  const reference=[0,1,2].map(n=>({sha256:String(n).padStart(64,'0')}));
  const phase={rows:reference,ended:true,droppedCallbacks:0,firstCallbackGap:0,captureErrors:[],
    quality:{droppedVideoFrames:0},events:[]};
  const row={label:'app-color-601-unchanged-client',result:'observed',selectedPublicHLSObserved:true,observer:{phases:[phase]}};
  assert.equal(appColorConsumerQualification(row,reference,[0,1,2],true).qualified,true);
  assert.equal(appColorConsumerQualification(row,reference,[0,1,2],false).qualified,false);
  const forced={...row,label:'app-color-601-forced-source-coordinate-seek'};
  assert.equal(appColorConsumerQualification(forced,reference,[0,1,2],true).qualified,false);
  assert.equal(appColorConsumerQualification({...forced,forceSeek:{seeking:true,seeked:true}},reference,[0,1,2],true).qualified,true);
  assert.equal(forced.label,'app-color-601-forced-source-coordinate-seek');
  assert.equal(Object.hasOwn(forced,'referenceComplete'),false);
});

test('each app arm is registered before execution and callback failure cannot omit the following arm',async()=>{
  const old={label:'original-audio'},rows=[old],plan=prepareAppColor(item,joined,pieces);
  const context={request:{get:async url=>{
    const name=url.slice(url.lastIndexOf('/')+1),bytes=name==='init.mp4'?pieces[0]:
      name==='segment-00000.m4s'?pieces[1]:pieces[2];
    return {status:()=>200,body:async()=>bytes};
  }}};
  let calls=0;
  await observeAppColor(context,'http://held.invalid',item,joined,[],true,rows,async(request,label)=>{
    assert.equal(rows.at(-1).label,label);calls++;
    if(calls===1)throw Error('held_callback_failure');
    return {request,label,result:'observed',frameConsumerQualified:false,actualAppColorAppend:witness(plan)};
  });
  assert.equal(calls,2);assert.equal(rows[0],old);assert.equal(rows.length,3);
  assert.equal(rows[1].label,'app-color-601-unchanged-client');assert.equal(rows[1].result,'observation-failed');
  assert.equal(rows[1].failureClass,'held_callback_failure');
  assert.equal(rows[2].label,'app-color-601-forced-source-coordinate-seek');
  assert.equal(rows[2].result,'observed');assert.equal(rows[2].appendWitness.qualified,true);
});

test('each media buffer needs its own exact init before its first fragment',()=>{
  const plan=prepareAppColor(item,joined,pieces),value=witness(plan);
  const split={...value,buffers:[...value.buffers,{...value.buffers[0],id:1}],
    appends:value.appends.map((v,n)=>({...v,bufferID:n===0?0:1}))};
  assert.equal(appColorAppendFacts(split,plan,pieces).qualified,false);
  const reversed={...value,appends:[value.appends[1],value.appends[0],value.appends[2]].map((v,n)=>({...v,ordinal:n}))};
  assert.equal(appColorAppendFacts(reversed,plan,pieces).qualified,false);
});
