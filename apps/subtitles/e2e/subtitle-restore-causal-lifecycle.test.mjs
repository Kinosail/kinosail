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
page.context=()=>({newCDPSession:()=>new Promise(yes=>setTimeout(()=>yes(session),50))});
page.evaluate=async()=>{throw Error('witness not installed');};
const began=performance.now();const diagnostic=observe(page,'https://127.0.0.1:1','a'.repeat(16),began+200,began+10);
await assert.rejects(diagnostic.start());
const result=await diagnostic.finish(began+150);
assert.equal(session.detached,true,'late allocated session must join and detach inside shared cleanup budget');
assert.equal(result.valid,false);assert.equal(result.stopped,true);assert.equal(session.eventNames().length,0);assert.equal(page.eventNames().length,0);
assert.ok(performance.now()-began<180,'cleanup must be bounded');
console.log('actual observer late allocation, partial start, invalid inference and bounded detach controls pass');
