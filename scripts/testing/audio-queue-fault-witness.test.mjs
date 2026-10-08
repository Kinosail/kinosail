import {test} from 'node:test';
import assert from 'node:assert/strict';
import {faultWitness, fetchQueueResponse, holdQueueResponse, fulfillFailure, observeFaultResponse} from '../../apps/player/e2e/test-instance-audio-queue-witness.mjs';
const deferred=()=>{let resolve;const promise=new Promise(done=>resolve=done);return {promise,resolve};};
test('actual fetch seam distinguishes unentered, pending upstream and returned non200 without hiding status',async()=>{
 const facts=faultWitness(), upstream=deferred();
 assert.equal(facts.handlerEntered,false);
 const route={fetch:()=>upstream.promise}, pending=fetchQueueResponse(route,facts);
 assert.equal(facts.handlerEntered,true);assert.equal(facts.upstreamStarted,true);assert.equal(facts.upstreamReturned,false);
 const response={status:()=>503};upstream.resolve(response);assert.equal(await pending,response);
 assert.equal(facts.upstreamReturned,true);assert.equal(facts.upstreamStatus,503);assert.equal(facts.holdRegistered,false);
});
test('fetch rejection retains original error and only closed failure facts',async()=>{
 const facts=faultWitness(), error=new Error('PRIVATE URL header body token');
 await assert.rejects(fetchQueueResponse({fetch:()=>Promise.reject(error)},facts),value=>value===error);
 assert.equal(facts.upstreamFailed,true);assert.equal(facts.upstreamReturned,false);
 assert.equal(JSON.stringify(facts).includes('PRIVATE'),false);
});
test('unknown malformed and out-of-range response statuses stay out of snapshots',async()=>{
 for(const status of [null, true, 'PRIVATE', -1, 99, 600, Infinity, 200.5]) {
  const facts=faultWitness();await fetchQueueResponse({fetch:async()=>({status:()=>status})},facts);
  assert.equal(facts.upstreamStatus,null);assert.equal(facts.invalidStatus,true);
  assert.equal(JSON.stringify(facts).includes('PRIVATE'),false);
 }
});
test('held-response observation is captured before cleanup release and fulfillment joins',async()=>{
 const facts=faultWitness(), calls=[], delivery=deferred();let release;
 const route={fulfill:async options=>{calls.push(options);await delivery.promise;}};
 const response={status:()=>200};const pending=holdQueueResponse(route,response,facts,value=>release=value);
 assert.equal(typeof release,'function');assert.equal(facts.holdRegistered,true);
 const before=structuredClone(facts);release();await Promise.resolve();
 assert.equal(before.holdReleased,false);assert.equal(before.fulfilled,false);
 assert.equal(facts.holdReleased,true);assert.equal(facts.fulfillStarted,true);assert.equal(facts.fulfilled,false);
 delivery.resolve();await pending;assert.equal(facts.fulfilled,true);assert.deepEqual(calls,[{response}]);
});
test('503 fulfilled and response observed are separate network facts, never app consumption',async()=>{
 const facts=faultWitness(), delivery=deferred();let options;
 const pending=fulfillFailure({fulfill:async value=>{options=value;await delivery.promise;}},facts,{'X-Request-ID':'qa-queue-read-failure'});
 assert.equal(facts.handlerEntered,true);assert.equal(facts.fulfillStarted,true);assert.equal(facts.fulfilled,false);
 assert.equal(facts.responseObserved,false);delivery.resolve();await pending;
 observeFaultResponse(facts,{status:()=>503});assert.equal(facts.fulfilled,true);assert.equal(facts.responseObserved,true);
 assert.equal(facts.responseStatus,503);assert.equal('appConsumed' in facts,false);
 assert.deepEqual(options,{status:503,headers:{'X-Request-ID':'qa-queue-read-failure'}});
});
test('fulfillment rejection remains an original failure with bounded facts',async()=>{
 const facts=faultWitness(), error=new Error('PRIVATE transport');
 await assert.rejects(fulfillFailure({fulfill:async()=>{throw error;}},facts,{}),value=>value===error);
 assert.equal(facts.fulfillFailed,true);assert.equal(facts.fulfilled,false);
 assert.equal(JSON.stringify(facts).includes('PRIVATE'),false);
});

// Resolve actual pinned fixture pools without running hooks, browser or Server.
test('only the two routed queue faults block workers in every pinned engine', async t => {
 const {createRequire}=await import('node:module');
 const {dirname,join}=await import('node:path');
 const {fileURLToPath}=await import('node:url');
 const {mkdtempSync,readFileSync,rmSync}=await import('node:fs');
 const {tmpdir}=await import('node:os');
 const directory=fileURLToPath(new URL('../../apps/player/e2e/',import.meta.url));
 const require=createRequire(join(directory,'package.json'));
 const packagePath=createRequire(require.resolve('@playwright/test')).resolve('playwright/package.json');
 assert.equal(JSON.parse(readFileSync(packagePath,'utf8')).version,'1.63.0');
 const cache=mkdtempSync(join(tmpdir(),'kino-queue-fixture-'));
 const keys=['KINOSAIL_BROWSER_MATRIX','KINOSAIL_BROWSER_PROJECT','PWTEST_CACHE_DIR'];
 const before=Object.fromEntries(keys.map(key=>[key,process.env[key]]));
 try {
  process.env.KINOSAIL_BROWSER_MATRIX='full';delete process.env.KINOSAIL_BROWSER_PROJECT;process.env.PWTEST_CACHE_DIR=cache;
  const {configLoader,testLoader,poolBuilder}=require(join(dirname(packagePath),'lib/common/index.js'));
  const config=await configLoader.loadConfig({configDir:directory,resolvedConfigFile:join(directory,'playwright.config.ts')});
  const errors=[],suite=await testLoader.loadTestFile(join(directory,'test-instance-audio-queue.spec.ts'),config,errors);
  assert.deepEqual(errors,[]);assert.equal(suite.allTests().length,4);
  assert.equal(suite.allTests().filter(row=>row.tags.includes('@routed-fault')).length,2);
  for(const project of config.projects) await t.test(project.project.name,async()=>{
   poolBuilder.PoolBuilder.createForWorker(project).buildPools(suite,errors);assert.deepEqual(errors,[]);
   for(const registered of suite.allTests()) {
    assert.deepEqual(registered.titlePath().filter(Boolean),['test-instance-audio-queue.spec.ts',registered.title]);
    const option=registered._pool._registrations.get('serviceWorkers').fn;
    let resolved=option;
    if(typeof option==='function') await option({contextOptions:project.project.use.contextOptions??{}},value=>{resolved=value;});
    assert.equal(resolved,registered.tags.includes('@routed-fault')?'block':'allow');
   }
  });
 } finally {
  for(const key of keys) {if(before[key]===undefined)delete process.env[key];else process.env[key]=before[key];}
  rmSync(cache,{recursive:true,force:true});
 }
});
