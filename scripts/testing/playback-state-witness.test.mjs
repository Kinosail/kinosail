import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';
import {attachPlaybackState} from '../../apps/player/e2e/playback-state-witness.mjs';
const make=(raw,status=200)=>{const rows=[],calls=[];return {rows,calls,page:{request:{get:async(path,options)=>{calls.push([path,options]);return {status:()=>status,text:async()=>raw};}}},info:{attach:async(name,data)=>rows.push(JSON.parse(data.body))}};};
test('backend progress projection contains only bounded facts and exact stage',async()=>{
 const fixture=make(JSON.stringify({item:{title:'PRIVATE',progress:{seconds:12,watched:true}}}));
 await attachPlaybackState(fixture.page,fixture.info,'/watch/example-movie','before-method');
 assert.deepEqual(fixture.rows,[{schemaVersion:1,stage:'before-method',status:200,progress:{seconds:12,watched:true}}]);
 assert.equal(fixture.calls[0][0],'/api/v1/items/example-movie');
});
test('invalid watch or stage has no HTTP or attachment effects',async()=>{
 for(const watch of [null,'/watch/','/watch/'+ 'x'.repeat(129),'/watch/../secret','https://private/watch/item','/watch/id?secret=1']){
 const f=make('{}');await assert.rejects(attachPlaybackState(f.page,f.info,watch,'before-method'));assert.deepEqual(f.calls,[]);assert.deepEqual(f.rows,[]);
 }
 const f=make('{}');await assert.rejects(attachPlaybackState(f.page,f.info,'/watch/item','PRIVATE'));assert.deepEqual(f.calls,[]);
});
test('malformed missing oversized and private backend values publish only a closed unavailable result',async()=>{
 for(const raw of ['bad','x'.repeat(65537),'{}','['.repeat(33)+'0'+']'.repeat(33),'{"item":{"progress":{"seconds":0,"watched":false,"watched":true}}}',JSON.stringify({item:{progress:{seconds:-1,watched:true}}}),JSON.stringify({item:{progress:{seconds:'PRIVATE',watched:'PRIVATE'}}}),JSON.stringify({item:{progress:{seconds:Infinity,watched:false}}})]){
 const f=make(raw);await attachPlaybackState(f.page,f.info,'/watch/item','before-loading');
 assert.deepEqual(f.rows,[{schemaVersion:1,stage:'before-loading',status:200,unavailable:true}]);
 }
});
test('observation and attachment failures do not replace recipe errors',async()=>{
 const f=make('{}');f.page.request.get=async()=>{throw Error('PRIVATE');};
 await attachPlaybackState(f.page,f.info,'/watch/item','before-method');assert.equal(JSON.stringify(f.rows).includes('PRIVATE'),false);
 f.info.attach=async()=>{throw Error('PRIVATE');};await attachPlaybackState(f.page,f.info,'/watch/item','before-method');
});

test('loading returns validated facts and confirmed Watched sends exactly one trusted public gesture',async()=>{
 const {startWatchedPlayback}=await import('../../apps/player/e2e/playback-state-witness.mjs');
 for(const watched of [true,false]){
  const f=make(JSON.stringify({item:{progress:{seconds:0,watched}}})),calls=[];
  f.page.locator=selector=>({focus:async()=>calls.push(['focus',selector])});
  f.page.keyboard={press:async key=>calls.push(['press',key])};
  const facts=await attachPlaybackState(f.page,f.info,'/watch/item','before-loading');
  assert.deepEqual(facts,f.rows[0]);
  await startWatchedPlayback(f.page,facts);
  assert.deepEqual(calls,watched?[['focus','.media-stage'],['press','Space']]:[]);
 }
});
test('unavailable malformed or failed observations cannot cause a playback gesture',async()=>{
 const {startWatchedPlayback}=await import('../../apps/player/e2e/playback-state-witness.mjs');
 const page={locator:()=>{throw Error('unexpected gesture');}};
 for(const facts of [undefined,null,{}, {schemaVersion:1,stage:'before-loading',status:200,unavailable:true},
 {schemaVersion:1,stage:'before-method',status:200,progress:{seconds:0,watched:true}},
 {schemaVersion:1,stage:'before-loading',status:200,progress:{seconds:-1,watched:true}},
 {schemaVersion:1,stage:'before-loading',status:200,progress:{seconds:0,watched:'true'}},
 {schemaVersion:1,stage:'before-loading',status:200,progress:{seconds:0,watched:true},secret:'PRIVATE'}])await startWatchedPlayback(page,facts);
 for(const failure of ['request','attach']){
  const f=make(JSON.stringify({item:{progress:{seconds:0,watched:true}}}));
  if(failure==='request')f.page.request.get=async()=>{throw Error('PRIVATE');};
  else f.info.attach=async()=>{throw Error('PRIVATE');};
  const facts=await attachPlaybackState(f.page,f.info,'/watch/item','before-loading');
  const original=Error('original clock assertion');
  await assert.rejects(async()=>{await startWatchedPlayback(page,facts);throw original;},error=>error===original);
 }
});

test('attachment deadline cannot authorize intent and public gesture failures retain identity',async()=>{
 const {startWatchedPlayback}=await import('../../apps/player/e2e/playback-state-witness.mjs');
 const f=make(JSON.stringify({item:{progress:{seconds:0,watched:true}}}));f.info.attach=()=>new Promise(()=>{});
 const facts=await attachPlaybackState(f.page,f.info,'/watch/item','before-loading');
 assert.equal(facts.unavailable,true);
 await startWatchedPlayback({locator:()=>{throw Error('unexpected gesture');}},facts);
 const original=Error('public focus failed');
 await assert.rejects(startWatchedPlayback({locator:()=>({focus:async()=>{throw original;}})},
 {schemaVersion:1,stage:'before-loading',status:200,progress:{seconds:0,watched:true}}),error=>error===original);
});

test('fresh method-switch stages retain closed current progress without public playback intent',async()=>{
 for(const stage of ['at-method-switch','after-method-switch']) {
  const f=make(JSON.stringify({item:{progress:{seconds:12,watched:true}}}));
  const value=await attachPlaybackState(f.page,f.info,'/watch/item',stage);
  assert.deepEqual(value,{schemaVersion:1,stage,status:200,progress:{seconds:12,watched:true}});
  assert.equal(f.calls.length,1);assert.equal(f.rows.length,1);
 }
});

test('registered responsive method scenarios hold short media during geometry and switch only during actual playback',async()=>{
 const source=stripTypeScriptTypes(readFileSync(new URL('../../apps/player/e2e/layout-audit-player.spec.ts',import.meta.url),'utf8')).replace(/^import .*;\n/gm,'');
 for(const scenario of ['progressing','watched','seeking','error','nonfinite','zero duration']){
  let run,focused=false,method='Direct Play',switches=0,plays=0,pauses=0;
  const media={paused:scenario==='watched',ended:false,seeking:false,currentTime:scenario==='watched'?0:3,duration:12,readyState:scenario==='watched'?0:4,error:null};
  const rejected=Error('active playback required');
  const advance=amount=>{if(!media.paused){media.currentTime=Math.min(12,media.currentTime+amount);if(media.currentTime===12){media.ended=true;media.paused=true;}}};
  const box={left:0,right:400,top:500,bottom:544,height:44};
  const node={getBoundingClientRect:()=>box,parentElement:{getBoundingClientRect:()=>box}};
  const evaluate=(fn,arg)=>runInNewContext('('+fn.toString()+')(element,arg)',{element:node,arg,document:{querySelector:()=>node},getComputedStyle:()=>({backgroundColor:'rgb(1, 1, 1)'})});
  const panel={evaluate:async fn=>evaluate(fn)};node.getBoundingClientRect=()=>({...box,bottom:300,height:250});
  const stage={focus:async()=>{focused=true;}};
  const video={getAttribute:async()=> 'Compatibility',evaluate:async(fn,arg)=>{advance(.25);return fn(media,arg);}};
  const compatible={click:async()=>{if(media.paused||media.ended||media.seeking||!Number.isFinite(media.currentTime)||!Number.isFinite(media.duration)||media.duration<=0||media.error)throw rejected;
    switches++;method='Compatibility';media.currentTime=.25;}};
  const actions={getByRole:(role)=>role==='link'?compatible:{},getByText:()=>({click:async()=>{}})};
  const badge={evaluate:async fn=>evaluate(fn),click:async()=>{}};
  const page={goto:async path=>{if(path.startsWith('/watch')){method='Direct Play';if(media.ended){media.currentTime=0;media.readyState=0;}}},
   setViewportSize:async()=>{},evaluate:async()=>{},locator:selector=>selector==='video'?video:selector==='.media-stage'?stage:selector==='[data-playback-mode-status]'?badge:selector==='.player-settings'?panel:actions,
   keyboard:{press:async key=>{if(key==='Escape')return;assert.equal(key,'Space');assert(focused);if(media.paused){plays++;media.paused=false;media.ended=false;media.readyState=4;}else{pauses++;media.paused=true;}
    if(scenario==='seeking')media.seeking=true;if(scenario==='error')media.error={code:3};if(scenario==='nonfinite')media.duration=Infinity;if(scenario==='zero duration')media.duration=0;}},
   waitForTimeout:async()=>{},screenshot:async()=>advance(4)};
  const expect=value=>({toHaveText:async text=>assert.equal(method,text),toBeVisible:async()=>{},toBeHidden:async()=>{},toBeFocused:async()=>assert(focused),
   toHaveJSProperty:async(key,wanted)=>assert.equal(media[key],wanted),toBeGreaterThanOrEqual:minimum=>assert(value>=minimum),toBeTruthy:()=>assert(value),
   toBe: wanted=>assert.equal(value,wanted),not:{toBe:wanted=>assert.notEqual(value,wanted)},toEqual:wanted=>assert.equal(JSON.stringify(value),JSON.stringify(wanted))});
  expect.poll=callback=>({toBe:async wanted=>{if(await callback()!==wanted)throw rejected;},toBeTruthy:async()=>{if(!await callback())throw rejected;},toBeGreaterThan:async minimum=>{if(!(await callback()>minimum))throw rejected;}});
  const register=(title,callback)=>{if(title.startsWith('player shows and switches'))run=callback;};register.skip=()=>{};
  class AxeBuilder{async analyze(){advance(3);return {violations:[]};}}
  runInNewContext(source,{test:register,expect,AxeBuilder,configureLayoutAudit:()=>{},login:async()=>{},firstPlayable:async()=>'/watch/item',
   attachPlaybackState:async()=>advance(1),attachResponsiveFailure:async()=>{},layoutProblems:async()=>({documentOverflow:0,outside:[],tinyControls:[],distortedChecks:[],clippedControls:[],overlappingStatuses:[]}),
   viewports:[{width:1440,height:900},{width:390,height:844}],process:{env:{KINOSAIL_TEST_INSTANCE:'1'}}});
  if(['progressing','watched'].includes(scenario)){await run({page},{outputPath:name=>name});assert.equal(switches,2,scenario);assert.equal(pauses,2,scenario);assert(plays>=2,scenario);}
  else {await assert.rejects(run({page},{outputPath:name=>name}),error=>error===rejected,scenario);assert.equal(switches,0,scenario);}
 }
});
