import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';
const owner=()=>import('../../apps/player/e2e/compatibility-document.mjs');
test('old moving video cannot satisfy compatibility checks before exact new document commits',async()=>{
 const {selectCompatibilityDocument}=await owner();const calls=[];let commit;
 const gate=new Promise(resolve=>{commit=resolve;});
 const page={url:()=> 'http://127.0.0.1:18083/watch/item',evaluate:async()=>42,
 waitForURL:async(url,options)=>{calls.push(['url',url]);assert(options.timeout<=10000);await gate;},
 waitForFunction:async(fn,input,options)=>{calls.push(['document',input]);assert(options.timeout<=10000);
  const saved={performance:globalThis.performance,location:globalThis.location,document:globalThis.document,window:globalThis.window};
  try {globalThis.performance={timeOrigin:42};globalThis.location={href:input.destination};globalThis.document={readyState:'complete'};globalThis.window={mediaEvents:[]};assert.equal(fn(input),false);
   globalThis.performance={timeOrigin:43};assert.equal(fn(input),true);
   globalThis.window={};assert.equal(fn(input),false);
  } finally {for(const [key,value] of Object.entries(saved))globalThis[key]=value;}
 }};
 const link={getAttribute:async()=>'?compatible=1',click:async()=>calls.push(['click'])};
 let done=false;const pending=selectCompatibilityDocument(page,link,Date.now()).then(()=>{done=true;});
 await new Promise(resolve=>setImmediate(resolve));assert.equal(done,false);assert.deepEqual(calls.map(x=>x[0]),['click','url']);
 commit();await pending;assert.equal(done,true);assert.deepEqual(calls.map(x=>x[0]),['click','url','document']);
});
test('invalid destinations and document identity reject before mode click',async()=>{
 const {selectCompatibilityDocument}=await owner();let effects=0;
 for(const href of [null,'','x'.repeat(2049),'https://private.invalid/watch/item?compatible=1','?unknown=1','?compatible=1&secret=PRIVATE','?compatible=1#private']){
  await assert.rejects(selectCompatibilityDocument({url:()=> 'http://127.0.0.1/watch/item',evaluate:async()=>42},{getAttribute:async()=>href,click:async()=>{effects++;}},Date.now()));
 }
 for(const prior of ['http://foreign.invalid/watch/item','http://127.0.0.1/login','http://127.0.0.1/watch/'+ 'x'.repeat(129)])await assert.rejects(selectCompatibilityDocument({url:()=>prior},{getAttribute:async()=>'?compatible=1',click:async()=>{effects++;}},Date.now()));
 for(const identity of [null,-1,Infinity,'PRIVATE'])await assert.rejects(selectCompatibilityDocument({url:()=> 'http://127.0.0.1/watch/item',evaluate:async()=>identity},{getAttribute:async()=>'?compatible=1',click:async()=>{effects++;}},Date.now()));
 for(const started of [null,NaN,Infinity,-1,Date.now()+100000,Date.now()-10001])await assert.rejects(selectCompatibilityDocument({url:()=> 'http://127.0.0.1/watch/item',evaluate:async()=>42},{getAttribute:async()=>'?compatible=1',click:async()=>{effects++;}},started));
 assert.equal(effects,0);
});
test('destination or initialization failure preserves original identity without retry',async()=>{
 const {selectCompatibilityDocument}=await owner();
 for(const stage of ['url','document']){let clicks=0;const original=Error('private original');
 const page={url:()=> 'https://localhost/watch/item',evaluate:async()=>42,waitForURL:async()=>{if(stage==='url')throw original;},waitForFunction:async()=>{throw original;}};
 await assert.rejects(selectCompatibilityDocument(page,{getAttribute:async()=>'?compatible=1',click:async()=>{clicks++;}},Date.now()),e=>e===original);assert.equal(clicks,1);}
});

test('rendered Direct Play mode href remains its exact public destination',async()=>{
 const {selectCompatibilityDocument}=await owner();const destinations=[];
 await selectCompatibilityDocument({url:()=> 'https://localhost/watch/item?compatible=1',evaluate:async()=>42,
 waitForURL:async value=>destinations.push(value),waitForFunction:async()=>{}},
 {getAttribute:async()=>'?direct=1',click:async()=>{}},Date.now());
 assert.deepEqual(destinations,['https://localhost/watch/item?direct=1']);
});

test('closed compatibility telemetry records document continuity without media-event fallback',async()=>{
 const {attachCompatibilityDocumentState}=await owner();const rows=[],calls=[];
 let current={timeOrigin:42,observerPresent:true,documentReady:true};
 const page={evaluate:async()=>{calls.push('evaluate');return current;}}, info={attach:async(name,data)=>{assert.equal(name,'compatibility-document-state');assert(Buffer.byteLength(data.body)<=1024);rows.push(JSON.parse(data.body));}};
 const baseline=await attachCompatibilityDocumentState(page,info,'before-clock');
 assert.equal(baseline.timeOrigin,42);
 current={timeOrigin:43,observerPresent:false,documentReady:true};
 await attachCompatibilityDocumentState(page,info,'failure',baseline.timeOrigin);
 assert.deepEqual(rows,[{schemaVersion:1,phase:'before-clock',timeOrigin:42,observerPresent:true,documentReady:true,sameDocument:null},{schemaVersion:1,phase:'failure',timeOrigin:43,observerPresent:false,documentReady:true,sameDocument:false}]);
 assert.equal(calls.length,2);
});
test('compatibility telemetry rejects invalid identity and phase before any observation or attachment',async()=>{
 const {attachCompatibilityDocumentState}=await owner();let effects=0;
 const page={evaluate:async()=>{effects++;}},info={attach:async()=>{effects++;}};
 for(const phase of [null,'','unknown','failure'.repeat(2048)])await assert.rejects(attachCompatibilityDocumentState(page,info,phase));
 for(const identity of ['',-1,0,NaN,Infinity,1e15,'PRIVATE',{}])await assert.rejects(attachCompatibilityDocumentState(page,info,'failure',identity));
 assert.equal(effects,0);
});
test('untrusted compatibility telemetry fields publish only unavailable',async()=>{
 const {attachCompatibilityDocumentState}=await owner(),rows=[];
 for(const value of [null,{}, {timeOrigin:-1,observerPresent:true,documentReady:true},{timeOrigin:42,observerPresent:'PRIVATE',documentReady:true},{timeOrigin:42,observerPresent:true,documentReady:true,private:'PRIVATE'},'PRIVATE'.repeat(2048)])
  await attachCompatibilityDocumentState({evaluate:async()=>value},{attach:async(_name,data)=>rows.push(JSON.parse(data.body))},'failure',42);
 assert.equal(rows.length,6);assert.ok(rows.every(row=>JSON.stringify(row)==='{"schemaVersion":1,"phase":"failure","unavailable":true}'));
});
test('compatibility observation failures never replace the original telemetry assertion',async()=>{
 const {attachCompatibilityDocumentState}=await owner();
 for(const failed of ['evaluate','attach','deadline']){
  const original=Error('original mediaEvents assertion');
  const page={evaluate:async()=>{if(failed==='evaluate')throw Error('PRIVATE');return {timeOrigin:42,observerPresent:false,documentReady:true};}};
  const info={attach:async()=>{if(failed==='attach')throw Error('PRIVATE');if(failed==='deadline')await new Promise(()=>{});}};
  await assert.rejects(async()=>{await attachCompatibilityDocumentState(page,info,'failure',42);throw original;},error=>error===original);
 }
});
test('registered compatibility journey retains real clock and original missing-events error with phase witnesses',async()=>{
 const {attachCompatibilityDocumentState}=await owner(), original=TypeError('original mediaEvents missing'), rows=[];let run,moving=0;
 const source=stripTypeScriptTypes(readFileSync(new URL('../../apps/player/e2e/playback-startup.spec.ts',import.meta.url),'utf8')).replace(/^import .*;\n/gm,'');
 const register=(name,callback)=>{if(name==='selecting compatibility playback starts without a second play click')run=callback;};
 register.skip=()=>{};register.use=()=>{};register.beforeEach=()=>{};
 const video={evaluate:async(callback,arg)=>callback({currentTime:1,paused:false},arg),getAttribute:async()=> 'Compatibility'};
 const dummy={click:async()=>{},filter(){return this;}};
 const page={addInitScript:async()=>{},getByRole:()=>dummy,getByText:()=>dummy,locator:selector=>selector==='video'?video:dummy,evaluate:async(callback,arg)=>{
  if(String(callback).includes('mediaEvents.find'))throw original;
  if(String(callback).includes('observerPresent'))return {timeOrigin:42,observerPresent:false,documentReady:true};
  return callback(arg);
 }};
 const expect=()=>({toBe:()=>{},toHaveText:async()=>{},toBeLessThan:()=>{}});
 expect.poll=callback=>({toBeGreaterThan:async value=>{moving++;assert(await callback()>value);}});
 runInNewContext(source,{test:register,expect,selectCompatibilityDocument:async()=>{},attachCompatibilityDocumentState,login:async()=>{},process:{env:{KINOSAIL_TEST_INSTANCE:'1'}}});
 await assert.rejects(run({page},{attach:async(_name,data)=>rows.push(JSON.parse(data.body))}),error=>error===original);
 assert.equal(moving,1);assert.deepEqual(rows.map(row=>row.phase),['before-clock','failure']);
 assert.equal(rows.every(row=>row.observerPresent===false),true);
});
