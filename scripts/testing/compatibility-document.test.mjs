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
