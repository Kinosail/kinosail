// Written-first actual-app counterfactual failure matrix; no product source or color policy.
// A patched HTTP init alone cannot establish actual SourceBuffer bytes or rendered frames.
// Missing/unmatched/failed appends, wrong AVC/color, source substitution or incomplete coverage stay red.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {prepareAppColor,appColorAppendFacts,observeAppColor} from './hls-nonkey-browser-app-color.mjs';
const sha=b=>createHash('sha256').update(b).digest('hex');
const box=(kind,...parts)=>{const body=Buffer.concat(parts),out=Buffer.alloc(body.length+8);
  out.writeUInt32BE(out.length);out.write(kind,4,4,'ascii');body.copy(out,8);return out;};
const fragment=n=>Buffer.concat([box('moof',Buffer.from([n])),box('mdat',Buffer.from([n+1]))]);
const init=Buffer.concat([box('ftyp',Buffer.from('isom')),box('moov',box('trak',box('mdia',box('minf',
  box('stbl',box('stsd',Buffer.from([0,0,0,0,0,0,0,1]),
    box('avc1',Buffer.alloc(78),box('avcC',Buffer.from([1,100,0,30,255,225,0])))))))))]);
const pieces=[init,fragment(1),fragment(3)],joined=Buffer.concat(pieces);
const item={request:12.5,selectedSource:'/hls/held/master.m3u8',publicJoinedSHA256:sha(joined),
  publicAssetSHA256:{'360p/init.mp4':sha(init),'360p/segment-00000.m4s':sha(pieces[1]),'360p/segment-00001.m4s':sha(pieces[2])}};
function witness(plan){
  return {failures:[],overflow:false,buffers:[{id:0,mime:'video/mp4; codecs="avc1.64001e,mp4a.40.2"'}],
    appends:plan.appendPlans.filter(v=>!v.combined).map((v,n)=>({ordinal:n,bufferID:0,bytes:v.bytes,
      sha256:v.sha256,outcome:'updateend',mode:'segments',timestampOffset:12.5,appendWindowStart:0,
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
    {timestampOffset:NaN},{initBytes:[...init]},{initBytes:null},{bufferID:9}])
    assert.equal(appColorAppendFacts({...value,appends:[{...value.appends[0],...bad},...value.appends.slice(1)]},plan,pieces).qualified,false);
});
test('combined init and first media append still binds the exact counterfactual and every cut',()=>{
  const plan=prepareAppColor(item,joined,pieces),value=witness(plan),combined=plan.appendPlans.find(v=>v.combined);
  value.appends=[{...value.appends[0],sha256:combined.sha256,bytes:combined.bytes},value.appends[2]];
  assert.equal(appColorAppendFacts(value,plan,pieces).qualified,true);
});
test('setup failures retain new failure evidence after original cases without invoking app playback',async()=>{
  const old={label:'original-audio',qualification:{qualified:false}},rows=[old];let calls=0;
  await observeAppColor(null,'',item,joined,[],true,rows,async()=>{calls++;});
  assert.equal(rows[0],old);assert.equal(rows.length,2);assert.equal(calls,0);
  assert.equal(rows[1].label,'app-color-601-setup');assert.equal(rows[1].result,'observation-failed');
});
