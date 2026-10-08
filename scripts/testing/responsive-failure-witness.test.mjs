import test from 'node:test';
import assert from 'node:assert/strict';
import {runInNewContext} from 'node:vm';
import {attachResponsiveFailure,responsiveFailureFacts} from '../../apps/player/e2e/responsive-failure-witness.mjs';

function page(nodes={},extra={}) {
 return {evaluate: async(fn,kind)=>runInNewContext(`(${fn.toString()})(kind)`,{
  kind,innerWidth:390,innerHeight:844,getComputedStyle:node=>({display:node.display??'block'}),
  document:{querySelector:selector=>nodes[selector]??null,querySelectorAll:selector=>nodes[selector]??[]},...extra})};
}
const node=(values={})=>({hidden:false,classList:{contains:()=>false},closest:()=>null,
 querySelector:()=>null,getBoundingClientRect:()=>({x:16,y:20,width:358,height:200}),...values});
const recorder=()=>{const rows=[];return {rows,info:{attach:async(name,data)=>rows.push({name,...data})}};};

test('actual browser callback reports unavailable DOM and no HTML URL or arbitrary classes',async()=>{
 const {rows,info}=recorder();assert.equal(await attachResponsiveFailure(page(),info,'home-resume'),true);
 const value=JSON.parse(rows[0].body);assert.equal(value.elements.featured.available,false);
 assert.equal(value.elements.featured.rect,null);assert.equal(value.state.featuredMatchesExample,false);
 assert.deepEqual(value.viewport,{width:390,height:844});assert.equal(value.kind,'home-resume');
});
test('actual callback distinguishes native settings ownership and finite stage rectangles',async()=>{
 const options=node({classList:{contains:name=>name==='has-settings'}}),stage=node();
 const settings=node({closest:selector=>selector==='.player-native-options'?options:null});
 const status=node({classList:{contains:name=>name==='is-recovery'}});
 const video=node({paused:true,readyState:4});
 const facts=await page({'.media-stage':stage,'.player-native-options':options,'.player-settings':settings,
  '[data-player-status]':status,video}).evaluate(responsiveFailureFacts,'player-recovery');
 assert.equal(facts.state.settingsInNativeOptions,true);assert.equal(facts.state.settingsInStage,false);
 assert.equal(facts.state.optionsHasSettings,true);assert.equal(facts.state.stageHasSettings,false);
 assert.equal(facts.state.statusRecovery,true);assert.equal(facts.state.videoReadyState,4);
 const {rows,info}=recorder();await attachResponsiveFailure({evaluate:async()=>facts},info,'player-recovery');
 assert.equal(JSON.parse(rows[0].body).elements.settings.rect.width,358);
});
test('nonfinite geometry unknown display and missing media normalize into closed facts',async()=>{
 const facts=await page({'.player-settings':node({display:'PRIVATE',getBoundingClientRect:()=>({x:Infinity,y:NaN,width:358,height:200})})},
  {innerWidth:Infinity}).evaluate(responsiveFailureFacts,'player-settings');
 assert.equal(facts.viewport.width,null);assert.equal(facts.elements.settings.rect.x,null);
 assert.equal(facts.elements.settings.display,'other');assert.equal(facts.state.videoPaused,null);
 assert.equal(JSON.stringify(facts).includes('PRIVATE'),false);
});
test('unknown missing oversized kinds reject before evaluation or attachment',async()=>{
 let effects=0;
 for(const kind of [undefined,null,false,'unknown','x'.repeat(4097)])
  await assert.rejects(attachResponsiveFailure({evaluate:async()=>effects++},{attach:async()=>effects++},kind));
 assert.equal(effects,0);
});
test('malformed oversized and private snapshots retain only a fixed unavailable marker',async()=>{
 for(const data of [null,[],{private:'PRIVATE'},'x'.repeat(16385)]) {
  const {rows,info}=recorder();await attachResponsiveFailure({evaluate:async()=>data},info,'player-settings');
  const value=JSON.parse(rows[0].body);assert.equal(value.unavailable,true);
  assert.equal(value.reason,'invalid_snapshot');assert.equal(rows[0].body.includes('PRIVATE'),false);
 }
 const {rows,info}=recorder();await attachResponsiveFailure({evaluate:async()=>{throw new Error('PRIVATE URL token');}},info,'home-resume');
 assert.equal(JSON.parse(rows[0].body).reason,'snapshot_failed');assert.equal(rows[0].body.includes('PRIVATE'),false);
});
test('case budget is bounded and attachment rejection cannot replace the original assertion',async()=>{
 const {rows,info}=recorder();for(let i=0;i<300;i++)await attachResponsiveFailure(page(),info,'player-settings');
 assert.ok(rows.length>0&&rows.length<300);assert.ok(rows.every(row=>Buffer.byteLength(row.body)<=16384));
 assert.ok(rows.reduce((sum,row)=>sum+Buffer.byteLength(row.body),0)<=65536);
 assert.equal(await attachResponsiveFailure(page(),{attach:async()=>{throw new Error('PRIVATE');}},'home-resume'),false);
});

test('stalled observation fails boundedly without replacing the original assertion',async()=>{
 const {rows,info}=recorder();const start=Date.now();
 assert.equal(await attachResponsiveFailure({evaluate:()=>new Promise(()=>{})},info,'home-resume'),true);
 assert.ok(Date.now()-start<2000);assert.equal(JSON.parse(rows[0].body).reason,'snapshot_failed');
});

test('inconsistent availability rejects before private facts reach the attachment',async()=>{
 for(const change of [{hidden:null},{display:'unavailable'}]) {
  const facts=await page({'.player-settings':node()}).evaluate(responsiveFailureFacts,'player-settings');
  Object.assign(facts.elements.settings,change);
  const {rows,info}=recorder();await attachResponsiveFailure({evaluate:async()=>facts},info,'player-settings');
  assert.equal(JSON.parse(rows[0].body).reason,'invalid_snapshot');
 }
});
test('stalled attachment returns boundedly and late rejection stays handled',async()=>{
 let reject;const start=Date.now();
 assert.equal(await attachResponsiveFailure(page(),{attach:()=>new Promise((_,r)=>{reject=r;})},'home-resume'),false);
 assert.ok(Date.now()-start<2000);reject(new Error('PRIVATE'));await new Promise(resolve=>setImmediate(resolve));
});
