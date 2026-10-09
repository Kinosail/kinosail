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


test('actual settings witness selects visible utility actions instead of hidden progress notice',async()=>{
 const notice=node({hidden:true,display:'none',getBoundingClientRect:()=>({x:0,y:0,width:0,height:0})});
 const utility=node({display:'flex',getBoundingClientRect:()=>({x:36,y:850,width:1368,height:44})});
 const facts=await page({'.primary-player-actions':notice,
  '.primary-player-actions:not([data-progress-notice])':utility}).evaluate(responsiveFailureFacts,'player-settings');
 assert.equal(facts.elements.actions.hidden,false);assert.equal(facts.elements.actions.display,'flex');
 assert.equal(facts.elements.actions.rect.y,850);assert.equal(facts.elements.actions.rect.height,44);
 const absent=await page({'.primary-player-actions':notice}).evaluate(responsiveFailureFacts,'player-settings');
 assert.equal(absent.elements.actions.available,false);assert.equal(absent.elements.actions.rect,null);
});

test('actual callback classifies focus and native media state without retaining private control text',async()=>{
 const close=node({tagName:'BUTTON',matches:selector=>selector==='[data-player-settings-close]'});
 const video=node({tagName:'VIDEO',paused:true,readyState:4,currentTime:0,duration:30,networkState:1,
  controls:false,autoplay:false,ended:false,error:{code:0},webkitEnterFullscreen(){}});
 const body=node({classList:{contains:name=>name==='player-theater'}});
 const facts=await page({video},{navigator:{userAgent:'iPhone',platform:'iPhone',maxTouchPoints:5},
  document:{body,activeElement:close,querySelector:selector=>selector==='video'?video:null,querySelectorAll:()=>[]}
 }).evaluate(responsiveFailureFacts,'player-recovery');
 assert.equal(facts.state.activeControl,'close-settings');assert.equal(facts.state.playerTheater,true);
 assert.equal(facts.state.appleNativePlayback,true);assert.equal(facts.state.videoControls,false);
 assert.equal(facts.state.videoCurrentTime,0);assert.equal(facts.state.videoDuration,30);
 assert.equal(facts.state.videoNetworkState,1);assert.equal(facts.state.videoErrorCode,0);
 const {rows,info}=recorder();await attachResponsiveFailure({evaluate:async()=>facts},info,'player-recovery');
 assert.equal(JSON.parse(rows[0].body).state.activeControl,'close-settings');
});
test('unknown focus tags and nonfinite media values are unavailable, never private facts',async()=>{
 const video=node({tagName:'VIDEO',paused:false,readyState:2,currentTime:Infinity,duration:NaN,
  networkState:91,error:{code:99},controls:'PRIVATE',autoplay:'PRIVATE',ended:'PRIVATE'});
 const facts=await page({video},{navigator:{userAgent:'desktop',platform:'',maxTouchPoints:0},
  document:{activeElement:{tagName:'PRIVATE',matches:()=>false},querySelector:selector=>selector==='video'?video:null,querySelectorAll:()=>[]}
 }).evaluate(responsiveFailureFacts,'player-settings');
 assert.equal(facts.state.activeControl,'other');assert.equal(facts.state.appleNativePlayback,false);
 for(const key of ['videoCurrentTime','videoDuration','videoNetworkState','videoErrorCode','videoControls','videoAutoplay','videoEnded'])
  assert.equal(facts.state[key],null,key);
 assert.equal(JSON.stringify(facts).includes('PRIVATE'),false);
});
test('invalid focus or media states reject before private snapshots reach attachments',async()=>{
 for(const [key,value]of [['activeControl','PRIVATE'],['activeControl','x'.repeat(4097)],['playerTheater','PRIVATE'],
  ['appleNativePlayback',2],['videoCurrentTime',Infinity],['videoNetworkState',9],['videoErrorCode',5],['videoDuration',-1]]) {
  const facts=await page().evaluate(responsiveFailureFacts,'player-recovery');facts.state[key]=value;
  const {rows,info}=recorder();await attachResponsiveFailure({evaluate:async()=>facts},info,'player-recovery');
  assert.equal(JSON.parse(rows[0].body).reason,'invalid_snapshot');assert.equal(rows[0].body.includes('PRIVATE'),false);
 }
});

test('startup failure kind retains actual media/native state without private values',async()=>{
 const video=node({tagName:'VIDEO',paused:false,readyState:4,currentTime:1,duration:16,controls:true,autoplay:false,ended:false,networkState:1});
 const {rows,info}=recorder();
 assert.equal(await attachResponsiveFailure(page({video}),info,'player-startup'),true);
 const state=JSON.parse(rows[0].body).state;
 assert.equal(state.videoPaused,false);assert.equal(state.videoCurrentTime,1);assert.equal(state.videoAutoplay,false);
});

test('startup hold actual callbacks remain pending until release and settle during owned cleanup',async()=>{
 const {holdStartupMedia}=await import('../../apps/player/e2e/startup-media-hold.mjs');
 const routes=new Map(),removed=[];
 const page={route:async(pattern,fn)=>routes.set(pattern,fn),unroute:async(pattern,fn)=>{assert.equal(routes.get(pattern),fn);removed.push(pattern);}};
 const hold=await holdStartupMedia(page,'automatic');let delivered;
 const media=routes.get('**/media/**')({fulfill:async value=>{delivered=value;}});
 const hls=routes.get('**/hls/**')({continue:async()=>{}});
 await new Promise(resolve=>setImmediate(resolve));
 assert.deepEqual(hold.snapshot(),{mediaEntered:1,hlsEntered:1,released:false,finished:0,failed:0,overflow:false});
 assert.equal(delivered,undefined);hold.release();await Promise.all([media,hls]);
 assert.equal(delivered.status,206);assert.equal(delivered.body,'x');
 assert.equal(hold.snapshot().finished,2);await hold.close();assert.equal(removed.length,2);
});
test('startup hold rejects unknown input before route registration and records failure before cleanup',async()=>{
 const {holdStartupMedia}=await import('../../apps/player/e2e/startup-media-hold.mjs');let effects=0;
 for(const source of [undefined,null,'private','x'.repeat(4097)])await assert.rejects(holdStartupMedia({route:async()=>effects++},source));
 assert.equal(effects,0);const routes=new Map();const original=new Error('private cause');
 const hold=await holdStartupMedia({route:async(p,f)=>routes.set(p,f),unroute:async()=>{}},'direct');
 const task=routes.get('**/media/**')({continue:async()=>{throw original;}});task.catch(()=>{});
 const before=hold.snapshot();assert.equal(before.released,false);hold.release();await assert.rejects(task,error=>error===original);
 assert.equal(hold.snapshot().failed,1);await hold.close();assert.equal(before.released,false);
});
test('startup fixture overflow cannot enqueue or forward another held request',async()=>{
 const {holdStartupMedia}=await import('../../apps/player/e2e/startup-media-hold.mjs');const routes=new Map();let effects=0;
 const hold=await holdStartupMedia({route:async(p,f)=>routes.set(p,f),unroute:async()=>{}},'direct');
 const route={continue:async()=>effects++},tasks=Array.from({length:64},()=>routes.get('**/media/**')(route));
 const extra=Promise.resolve().then(()=>routes.get('**/media/**')(route));extra.catch(()=>{});
 await new Promise(resolve=>setImmediate(resolve));hold.release();await Promise.all(tasks);
 await assert.rejects(extra,/capacity/);assert.equal(effects,64);assert.equal(hold.snapshot().overflow,true);await hold.close();
});
test('negative dimensions reject snapshots while negative coordinates remain valid',async()=>{
 for(const [section,key] of [['viewport','width'],['viewport','height'],['settings','width'],['settings','height']]){
  const facts=await page({'.player-settings':node()}).evaluate(responsiveFailureFacts,'player-startup');
  if(section==='viewport')facts.viewport[key]=-1;else facts.elements.settings.rect[key]=-1;
  const {rows,info}=recorder();await attachResponsiveFailure({evaluate:async()=>facts},info,'player-startup');
  assert.equal(JSON.parse(rows[0].body).reason,'invalid_snapshot');
 }
 const facts=await page({'.player-settings':node({getBoundingClientRect:()=>({x:-10,y:-20,width:0,height:0})})}).evaluate(responsiveFailureFacts,'player-startup');
 const {rows,info}=recorder();await attachResponsiveFailure({evaluate:async()=>facts},info,'player-startup');
 assert.equal(JSON.parse(rows[0].body).elements.settings.rect.x,-10);
 assert.equal(JSON.parse(rows[0].body).elements.settings.rect.height,0);
});
test('actual callback normalizes impossible dimensions without discarding finite coordinates',async()=>{
 const facts=await page({'.player-settings':node({getBoundingClientRect:()=>({x:-10,y:-20,width:-1,height:-2})})},
  {innerWidth:-1,innerHeight:-2}).evaluate(responsiveFailureFacts,'player-startup');
 assert.equal(facts.viewport.width,null);assert.equal(facts.viewport.height,null);
 assert.equal(facts.elements.settings.rect.width,null);assert.equal(facts.elements.settings.rect.height,null);
 assert.equal(facts.elements.settings.rect.x,-10);assert.equal(facts.elements.settings.rect.y,-20);
});
test('playback intent witness retains actual template autoplay and finite start without private dataset',async()=>{
 const video=node({tagName:'VIDEO',dataset:{start:'1',private:'PRIVATE'},hasAttribute:key=>key==='data-autoplay',getAttribute:key=>key==='data-start'?'1':null});
 const facts=await page({'video':video},{navigator:{userAgent:'desktop',maxTouchPoints:0}}).evaluate(responsiveFailureFacts,'player-settings');
 assert.equal(facts.state.videoAutoplayIntent,true);assert.equal(facts.state.videoStart,1);assert.equal(facts.state.theaterAvailable,false);
 assert.equal(JSON.stringify(facts).includes('PRIVATE'),false);
});

test('method switch facts close document route capability and identity observations',async()=>{
 for(const kind of ['player-method-before-switch','player-method-after-switch']) {
  const {rows,info}=recorder();const video=node({paused:true,readyState:4,currentTime:0,duration:12,autoplay:false,controls:true,
   hasAttribute:()=>false,getAttribute:()=>null});
  await attachResponsiveFailure(page({video},{location:{protocol:'http:',hostname:'localhost',pathname:'/watch/item',search:'?compatible=1',hash:''},
   performance:{timeOrigin:1000000000000},document:{readyState:'complete',querySelector:s=>s==='video'?video:null,querySelectorAll:()=>[]}}),info,kind);
  const value=JSON.parse(rows[0].body);assert.equal(value.state.documentEpoch,1000000000000);assert.equal(value.state.routeMode,'watch-compatible');
  assert.equal(value.state.loopbackOwned,true);assert.equal(value.state.videoPaused,true);assert.equal(value.state.videoAutoplayIntent,false);
 }
});
test('loading width facts distinguish absent hidden disconnected and visible statuses',async()=>{
 for(const status of [null,node({hidden:true,isConnected:true}),node({isConnected:false}),node({isConnected:true})]) {
  const {rows,info}=recorder();await attachResponsiveFailure(page({'[data-player-status]':status}),info,'player-loading-width');
  const value=JSON.parse(rows[0].body);assert.equal(value.elements.status.available,status!==null);
  assert.equal(value.elements.status.connected,status?.isConnected??null);
  assert.equal(value.elements.status.hidden,status?.hidden??null);
 }
});
test('keyboard focus facts report only fixed skip/code-input semantics and no private text',async()=>{
 const skip=node({isConnected:true,matches:()=>false,textContent:'PRIVATE-SENTINEL'});
 const active=node({tagName:'INPUT',hasAttribute:key=>key==='autofocus',matches:selector=>selector==='[data-quick-connect-digit]'});
 const {rows,info}=recorder();await attachResponsiveFailure(page({}, {document:{readyState:'complete',activeElement:active,
  querySelector:s=>s==='a.skip[href="#main"]'?skip:null},performance:{now:()=>50}}),info,'keyboard-focus');
 const value=JSON.parse(rows[0].body);assert.equal(value.state.skipFocused,false);assert.equal(value.state.activeControl,'quick-code');
 assert.equal(value.state.activeAutofocus,true);assert.ok(!rows[0].body.includes('PRIVATE-SENTINEL'));
});
test('new witness kinds reject rogue fields and observation failures preserve original errors',async()=>{
 for(const kind of ['player-method-before-switch','player-method-after-switch','player-loading-width','keyboard-focus']) {
  const {rows,info}=recorder();await attachResponsiveFailure({evaluate:async()=>({kind,private:'PRIVATE-SENTINEL'})},info,kind);
  assert.equal(JSON.parse(rows[0].body).reason,'invalid_snapshot');assert.ok(!rows[0].body.includes('PRIVATE-SENTINEL'));
  await attachResponsiveFailure({evaluate:async()=>{throw Error('PRIVATE-SENTINEL')}},{attach:async()=>{throw Error('PRIVATE-SENTINEL')}},kind);
 }
});

test('new document and status fields reject malformed oversized unknown and conflicting values',async()=>{
 const good=await page().evaluate(responsiveFailureFacts,'player-method-after-switch');
 for(const change of [v=>delete v.state.documentEpoch,v=>v.state.documentEpoch=Infinity,v=>v.state.documentEpoch=-1,
  v=>v.state.documentReadyState='PRIVATE',v=>v.state.routeMode='x'.repeat(16385),v=>v.state.touchPoints=33,
  v=>v.state.appleTouch='true',v=>v.elements.status.connected=true,v=>v.elements.status.visibility='PRIVATE',
  v=>v.state.statusState='PRIVATE',v=>v.state.unknown='PRIVATE']) {
  const value=structuredClone(good);change(value);const {rows,info}=recorder();
  await attachResponsiveFailure({evaluate:async()=>value},info,'player-method-after-switch');
  assert.equal(JSON.parse(rows[0].body).reason,'invalid_snapshot');assert.ok(!rows[0].body.includes('PRIVATE'));
 }
});
