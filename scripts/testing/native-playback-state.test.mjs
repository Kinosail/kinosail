import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';
const owner=()=>import('../../apps/player/e2e/native-playback-state.mjs');
const fixture=()=>{const rows=[],calls=[],media={paused:false,ended:false,seeking:false,currentTime:4,duration:12,readyState:4,networkState:1,error:null,controls:true,hasAttribute:name=>name==='data-native-controls'};
return {rows,calls,media,video:{evaluate:async callback=>{calls.push('evaluate');return callback(media);}},info:{attach:async(name,data)=>{calls.push('attach');assert.equal(name,'native-playback-state');assert(Buffer.byteLength(data.body)<=2048);rows.push(JSON.parse(data.body));}}};};
test('finite pre after and failure observations expose only existing real media facts',async()=>{
 const {attachNativePlaybackState}=await owner();const f=fixture();
 for(const phase of ['before-click','after-click','failure'])await attachNativePlaybackState(f.video,f.info,phase);
 assert.deepEqual(f.calls,['evaluate','attach','evaluate','attach','evaluate','attach']);
 assert.deepEqual(f.rows.map(x=>x.phase),['before-click','after-click','failure']);
 assert.deepEqual(f.rows[0],{schemaVersion:1,phase:'before-click',media:{paused:false,ended:false,seeking:false,currentTime:4,duration:12,readyState:4,networkState:1,errorCode:0,controls:true,nativeControls:true}});
 f.media.ended=true;f.media.paused=true;await attachNativePlaybackState(f.video,f.info,'failure');assert.equal(f.rows.at(-1).media.ended,true);
});
test('missing malformed unknown oversized out of range observations fail closed without private projection',async()=>{
 const {attachNativePlaybackState}=await owner();
 for(const value of [null,{}, {paused:'PRIVATE'}, {secret:'PRIVATE'}, 'PRIVATE'.repeat(2048)]){
  const f=fixture();f.video.evaluate=async()=>value;await attachNativePlaybackState(f.video,f.info,'failure');
  assert.deepEqual(f.rows,[{schemaVersion:1,phase:'failure',unavailable:true}]);
 }
 for(const [key,value] of [['currentTime',-1],['duration',31622401],['readyState',5],['networkState',4],['error',{code:5}],['error',{code:NaN}],['error',{code:''}],['controls','PRIVATE']]){
  const f=fixture();f.media[key]=value;await attachNativePlaybackState(f.video,f.info,'failure');assert.deepEqual(f.rows,[{schemaVersion:1,phase:'failure',unavailable:true}]);
 }
 for(const phase of [null,'','PRIVATE','before-click'.repeat(2048)]){const f=fixture();await assert.rejects(attachNativePlaybackState(f.video,f.info,phase));assert.deepEqual(f.calls,[]);}
});
test('observer rejection and attachment failure cannot replace the original assertion',async()=>{
 const {attachNativePlaybackState}=await owner();
 for(const failure of ['evaluate','attach','deadline']){
  const f=fixture();if(failure==='evaluate')f.video.evaluate=async()=>{throw Error('PRIVATE');};
  else if(failure==='attach')f.info.attach=async()=>{throw Error('PRIVATE');};else f.info.attach=()=>new Promise(()=>{});
  const original=Error('original paused assertion');await assert.rejects(async()=>{await attachNativePlaybackState(f.video,f.info,'failure');throw original;},e=>e===original);
  assert.equal(JSON.stringify(f.rows).includes('PRIVATE'),false);
 }
});

test('actual native smoke registration retains one click and original assertion through observation failures',async()=>{
 const {attachNativePlaybackState}=await owner();
 const source=stripTypeScriptTypes(readFileSync(new URL('../../apps/player/e2e/test-instance-playback.spec.ts',import.meta.url),'utf8')).replace(/^import .*;\n/gm,'');
 for(const failure of ['none','attach','evaluate']){
  let run,clicks=0,plays=0;const rows=[],original=Error('original paused false assertion');const f=fixture();
  f.media.pause=()=>{f.media.paused=true;};f.media.play=async()=>{plays++;f.media.paused=false;f.media.currentTime++;};
  f.video.evaluate=async(callback,arg)=>{if(failure==='evaluate'&&String(callback).includes('errorCode'))throw Error('PRIVATE');return callback(f.media,arg);};
  f.video.dispatchEvent=async type=>{assert.equal(type,'click');clicks++;f.media.paused=true;};
  const info={attach:async(name,data)=>{if(failure==='attach')throw Error('PRIVATE');rows.push(JSON.parse(data.body));}};
  const register=(title,callback)=>{if(title.startsWith('@smoke native playback'))run=callback;};
  const expect=()=>({toHaveJSProperty:async(key,value)=>{if(f.media[key]!==value)throw original;},toHaveAttribute:async()=>{}});
  expect.poll=callback=>({toBeTruthy:async()=>assert(await callback()),toBeGreaterThanOrEqual:async value=>assert(await callback()>=value),toBeGreaterThan:async value=>assert(await callback()>value)});
  runInNewContext(source,{test:register,expect,attachNativePlaybackState,configureTestInstance:()=>{},login:async()=>{},firstPlayable:async()=>'/watch/item'});
  await assert.rejects(run({page:{goto:async()=>{},locator:()=>f.video}},info),error=>error===original);
  assert.equal(clicks,1);assert.equal(plays,1);
  if(failure!=='attach')assert.deepEqual(rows.map(row=>row.phase),['before-click','after-click','failure']);
  if(failure==='none'){assert.equal(rows[0].media.paused,false);assert.equal(rows[1].media.paused,true);assert.equal(rows[2].media.ended,false);}
  assert.equal(JSON.stringify(rows).includes('PRIVATE'),false);
 }
});
