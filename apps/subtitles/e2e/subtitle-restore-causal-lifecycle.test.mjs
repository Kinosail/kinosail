// Isolated gap: deterministic late CDP allocation is unavailable in populated E2E.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { EventEmitter } from 'node:events';
import { installFetchWitness, equalDiagnosticHeaders } from './subtitle-restore-causal-witness.mjs';
let text=readFileSync(new URL('./subtitle-restore-causal-observer.ts',import.meta.url),'utf8').replace(/^import .*\n/gm,'').replace('export function observeCausal','function observeCausal');
const bounded=async(promise,ms)=>{let timer;try{return await Promise.race([promise,new Promise((_,no)=>{timer=setTimeout(()=>no(Error('bounded')),ms)})]);}finally{clearTimeout(timer);}};
const factory=new Function('installFetchWitness','equalDiagnosticHeaders','requestFailureCode','bounded','pause','validCausal','validNative','validServer',stripTypeScriptTypes(text)+'\nreturn observeCausal;');
const observe=factory(installFetchWitness,equalDiagnosticHeaders,()=> 'unclassified',bounded,ms=>new Promise(yes=>setTimeout(yes,ms)),()=>true,()=>false,()=>false);
class Session extends EventEmitter {detached=false;async send(){throw Error('controlled enable failure');}async detach(){this.detached=true;}}
const session=new Session(),page=new EventEmitter();
page.context=()=>({browser:()=>({browserType:()=>({name:()=> 'chromium'}),version:()=> '153.0.8010.12'}),newCDPSession:()=>new Promise(yes=>setTimeout(()=>yes(session),50))});
page.evaluate=async()=>{throw Error('witness not installed');};
const began=performance.now();const diagnostic=observe(page,'https://127.0.0.1:1','a'.repeat(16),began+200,began+10,'r06-restore-headers-desktop');
await assert.rejects(diagnostic.start());
const result=await diagnostic.finish(began+150);
assert.equal(session.detached,true,'late allocated session must join and detach inside shared cleanup budget');
assert.equal(result.valid,false);assert.equal(result.stopped,true);assert.equal(session.eventNames().length,0);assert.equal(page.eventNames().length,0);
assert.equal(result.completion.pinnedBrowserMatched,true);
assert.equal(await diagnostic.finish(began+150),result,'completion closure must be idempotent before navigation');
assert.ok(performance.now()-began<180,'cleanup must be bounded');
console.log('actual observer late allocation, partial start, invalid inference and bounded detach controls pass');

// Independent delivery scheduling cannot be forced reliably in populated E2E.
const schemaText=readFileSync(new URL('./subtitle-restore-causal-schema.ts',import.meta.url),'utf8').replace(/^import .*\n/gm,'').replace(/^export /gm,'');
const validators=new Function(stripTypeScriptTypes(schemaText)+'\nreturn {validCausal,validNative,validServer};')();
const joinedObserve=factory(installFetchWitness,equalDiagnosticHeaders,value=>value==='net::ERR_ABORTED'?'aborted':'unclassified',
 bounded,ms=>new Promise(yes=>setTimeout(yes,ms)),validators.validCausal,validators.validNative,validators.validServer);
class JoinedSession extends EventEmitter {detached=false;async send(){}async detach(){this.detached=true;}}
const joinedSession=new JoinedSession(),joinedPage=new EventEmitter();
const origin='https://127.0.0.1:1',item='b'.repeat(16),native204={outcome:'fulfilled',status:204,signalPresent:false,signalAborted:false,pagehide:false};
const nativeHeld={outcome:'rejected',status:0,signalPresent:true,signalAborted:true,pagehide:false};
const delivered={seen:true,held:false,delivered:true,cancelled:false,timedOut:false,settled:true};
const servers=[{...delivered},{...delivered},{...delivered,held:true,delivered:false,cancelled:true}];
let nativeReadyAt=Infinity,reads=0,wrapperClosed=false;
joinedPage.context=()=>({browser:()=>({browserType:()=>({name:()=> 'chromium'}),version:()=> '153.0.8010.12'}),newCDPSession:async()=>joinedSession});
joinedPage.request={get:async url=>({status:()=>200,body:async()=>Buffer.from(JSON.stringify(url.endsWith('/completion-witness')?{bodyless:true,matched:true}:servers))})};
joinedPage.evaluate=async(fn,argument)=>{
 if(typeof argument==='string'){
  const id=argument;
  joinedSession.emit('Network.requestWillBeSent',{requestId:id,type:'Fetch',request:{url:origin+'/__r06_restore/probe',method:'POST',postData:JSON.stringify({mode:id})}});
  if(id!=='held')joinedSession.emit('Network.responseReceived',{requestId:id,response:{status:204,headers:{'Cache-Control':'no-store','Content-Type':'application/json',Date:'Mon, 01 Jan 1990 00:00:00 GMT'}}});
  joinedSession.emit('Network.loadingFailed',{requestId:id,errorText:'net::ERR_ABORTED',canceled:true});return;
 }
 if(String(fn).includes('.read()')){
  reads++;
  return {valid:true,completion:{requestMatched:true,responseMatched:true},direct:{...native204},captured:{...native204},held:{...nativeHeld},
   restore:performance.now()>=nativeReadyAt?{...native204}:{...native204,outcome:'pending',status:0}};
 }
 if(String(fn).includes('.close()'))wrapperClosed=true;
};
const joinedBegan=performance.now(),joined=joinedObserve(joinedPage,origin,item,joinedBegan+400,joinedBegan+300,'r06-restore-headers-desktop');
await joined.start();await joined.probe();
joinedSession.emit('Network.requestWillBeSent',{requestId:'restore',type:'Fetch',request:{url:origin+'/api/v1/subtitle-library/'+item+'/restore',method:'POST'}});
joinedSession.emit('Network.loadingFailed',{requestId:'restore',errorText:'net::ERR_ABORTED',canceled:true});
nativeReadyAt=performance.now()+40;
const beforeJoinReads=reads,joinedResult=await joined.finish(joinedBegan+350);
assert.equal(joinedResult.restoreNative.outcome,'fulfilled','matched native completion must join after raw failure within same deadline');
assert.ok(reads>beforeJoinReads+1);assert.equal(joinedResult.restoreNetwork.terminal,'request-failed');
assert.equal(joinedResult.valid,true);assert.equal(joinedResult.stopped,true);assert.equal(wrapperClosed,true);assert.equal(joinedSession.detached,true);
assert.equal(await joined.finish(joinedBegan+350),joinedResult);assert.ok(performance.now()-joinedBegan<400);
console.log('actual observer joins delayed native fulfillment after raw failure within original deadline');
