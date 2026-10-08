import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';
import {EventEmitter} from 'node:events';
const source=stripTypeScriptTypes(readFileSync(new URL('../../apps/player/e2e/jellyfin-setup.spec.ts',import.meta.url),'utf8')).replace(/^import .*;\n/gm,'');
const pw=()=>{};for(const key of ['skip','use','afterEach'])pw[key]=()=>{};
const observe=runInNewContext(source+'\n(typeof observeTrustedHTTPSDisable === "function" ? observeTrustedHTTPSDisable : undefined)',{test:pw,configureProviderProfile(){},process:{env:{}},URL,Buffer,setTimeout,clearTimeout});
const origin='https://localhost:38127';
class Page extends EventEmitter {current=origin+'/settings#trusted-https';frame={url:()=>this.current};url(){return this.current;}mainFrame(){return this.frame;}}
test('later disable captures only actual POST/redirect/document facts and preserves original failure',async()=>{
 assert.equal(typeof observe,'function');
 const page=new Page(),bodies=[],original=new Error('original destination assertion');let actions=0;
 const request={url:()=>origin+'/settings/trusted-https/disable',method:()=> 'POST',isNavigationRequest:()=>true,frame:()=>page.frame};
 await assert.rejects(observe(page,{attach:async(name,{body})=>{assert.equal(name,'trusted-https-disable-stages');bodies.push(JSON.parse(body));}},async()=>{
  actions++;page.emit('request',request);page.emit('response',{request:()=>request,status:()=>303,headers:()=>({location:'/settings#trusted-https',secret:'private-secret'})});
  page.emit('request',{...request,method:()=> 'GET',url:()=>origin+'/settings'});page.emit('framenavigated',page.frame);throw original;
 }),error=>error===original);
 assert.equal(actions,1);assert.equal(page.eventNames().length,0);
 assert.equal(bodies[0].postRequests,1);assert.equal(bodies[0].postStatus,303);assert.equal(bodies[0].redirectSettings,true);assert.equal(bodies[0].documentCommitted,true);
 assert.equal(bodies[0].currentFragment,'trusted-https');assert.doesNotMatch(JSON.stringify(bodies),/localhost|private-secret|https:\/\//);
});
test('foreign/query/oversized events and arbitrary errors never leak; attachment failure preserves action',async()=>{
 assert.equal(typeof observe,'function');const page=new Page(),original=new Error('original action');
 await assert.rejects(observe(page,{attach:async()=>{throw new Error('private-secret');}},async()=>{
  for(const url of ['https://foreign.invalid/settings/trusted-https/disable',origin+'/settings/trusted-https/disable?secret','x'.repeat(2049)])
   page.emit('request',{url:()=>url,method:()=> 'POST'});
  throw original;
 }),error=>error===original);assert.equal(page.eventNames().length,0);
});
test('invalid initial authority rejects before action, listeners or attachment',async()=>{
 assert.equal(typeof observe,'function');for(const raw of ['https://foreign.invalid/settings','x'.repeat(2049),'http://user@localhost:38127/settings']) {
  const page=new Page();page.current=raw;let effects=0;
  await assert.rejects(observe(page,{attach:async()=>effects++},async()=>effects++));assert.equal(effects,0);assert.equal(page.eventNames().length,0);
 }
});
test('untrusted response status and redirect values stay bounded and closed',async()=>{
 for(const location of ['https://foreign.invalid/settings#trusted-https','x'.repeat(2049),{secret:'private-secret'}]) {
  const page=new Page(),values=[];const request={url:()=>origin+'/settings/trusted-https/disable',method:()=> 'POST'};
  await observe(page,{attach:async(_,value)=>values.push(JSON.parse(value.body))},async()=>{
   page.emit('response',{request:()=>request,status:()=> '303-private-secret',headers:()=>({location})});
  });
  assert.equal(values[0].postStatus,null);assert.notEqual(values[0].redirectSettings,true);assert.doesNotMatch(JSON.stringify(values),/private-secret|foreign/);
 }
});
test('actual recipe retains the later Disable action and exact URL oracle under a distinct observer',()=>{
 assert.match(source,/await observeTrustedHTTPSDisable\(page, testInfo, async \(\) => \{/);
 const block=source.slice(source.lastIndexOf('await observeTrustedHTTPSDisable'));
 assert.match(block,/Disable trusted HTTPS after restart/);assert.match(block,/await expect\(page\).toHaveURL\("\/settings#trusted-https"\)/);
 const workflow=readFileSync(new URL('../../.github/workflows/app.yml',import.meta.url),'utf8');
 assert.equal(workflow.split('\n').filter(row=>row.includes('run: node --test')&&row.includes('scripts/testing/trusted-https-disable-witness.test.mjs')).length,1);
});
