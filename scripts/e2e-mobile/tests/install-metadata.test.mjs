import test from 'node:test';
import assert from 'node:assert/strict';
import {installPhone} from '../install.mjs';
// agent-device v0.21.22 packages/kernel/src/errors.ts:220-235 rehydrates
// these seven own keys even when their daemon values are undefined.
const names=['hint','diagnosticId','logPath','logPathUnavailable','diagnosticsRecord','retriable','supportedOn'];
function failure(metadata={}) {
 const details={stdout:'',stderr:'PRIVATE-SENTINEL uninstall refused',exitCode:1};
 for(const name of names)details[name]=metadata[name];
 Object.assign(details,metadata);
 const cause=Object.assign(new Error('PRIVATE-SENTINEL'),{code:'COMMAND_FAILED',details});
 return Object.assign(new Error('engine failed',{cause}),{code:'ENGINE_FAILURE'});
}
async function observed(error) {
 let calls=0;await assert.rejects(installPhone({installApp:async(...args)=>{calls++;assert.deepEqual(args,[undefined,{reinstall:true}]);throw error;}}),value=>value===error);
 assert.equal(calls,1);const text=error.details.observed;assert.ok(Buffer.byteLength(text)<=256);assert.ok(!text.includes('PRIVATE-SENTINEL'));return JSON.parse(text);
}
test('actual pinned rehydration metadata retains closed process evidence without copying metadata',async()=>{
 for(const metadata of [{},{hint:'PRIVATE-SENTINEL',diagnosticId:'PRIVATE-SENTINEL',logPath:'/PRIVATE-SENTINEL',logPathUnavailable:'PRIVATE-SENTINEL',
  diagnosticsRecord:{session:'PRIVATE-SENTINEL',requestId:'PRIVATE-SENTINEL'},retriable:false,supportedOn:'PRIVATE-SENTINEL'}, {processExitError:true}]) {
  const error=failure(metadata);assert.equal(Object.keys(failure().cause.details).length,10);
  assert.deepEqual(await observed(error),{stage:'install_app',category:'command_failed',exitCode:1,stdoutBytes:0,stderrBytes:34,packageAbsentText:false});
 }
});
test('unknown, malformed, oversized and conflicting metadata fails closed',async()=>{
 for(const metadata of [{unknown:'PRIVATE-SENTINEL'},{hint:null},{hint:[]},{hint:'x'.repeat(2049)},{hint:'\ud800'},
  {diagnosticId:1},{retriable:'true'},{supportedOn:{}},{processExitError:'true'},
  {diagnosticsRecord:null},{diagnosticsRecord:{session:'PRIVATE-SENTINEL'}},
  {diagnosticsRecord:{session:'PRIVATE-SENTINEL',requestId:'ok',unknown:true}},
  {diagnosticsRecord:{session:'x'.repeat(2049),requestId:'ok'}},{diagnosticsRecord:{session:'',requestId:'ok'}}])
  assert.equal((await observed(failure(metadata))).category,'unqualified');
 const error=failure();Object.defineProperty(error.cause.details,'hint',{get(){throw new Error('PRIVATE-SENTINEL');}});
 assert.equal((await observed(error)).category,'unqualified');
});

test('supported optional output and nullable exits qualify explicit unavailable facts',async()=>{
 for(const details of [{exitCode:null},{exitCode:1},{stderr:'PRIVATE-SENTINEL',exitCode:null},
  {stdout:undefined,stderr:undefined,exitCode:null},{stdout:'',stderr:'',exitCode:undefined}]) {
  const error=failure();error.cause.details=details;
  assert.deepEqual(await observed(error),{stage:'install_app',category:'command_failed',exitCode:details.exitCode??null,
   stdoutBytes:details.stdout===undefined?null:Buffer.byteLength(details.stdout),stderrBytes:details.stderr===undefined?null:Buffer.byteLength(details.stderr),packageAbsentText:false});
 }
});
test('optional does not accept malformed present outputs or emit arbitrary rejection data',async()=>{
 for(const details of [{exitCode:null,stdout:null},{exitCode:null,stderr:42},{exitCode:null,stdout:'x'.repeat(65537)},
  {exitCode:null,stderr:'x'.repeat(8193)},{exitCode:null,stdout:'\ud800'},{exitCode:'PRIVATE-SENTINEL'},
  {exitCode:null,unknown:'PRIVATE-SENTINEL'},{exitCode:0},{exitCode:null,...Object.fromEntries(Array.from({length:129},(_,i)=>['unknown'+i,'PRIVATE-SENTINEL']))},
  {exitCode:null,[Symbol('PRIVATE-SENTINEL')]:true},{stdout:undefined,stderr:undefined,exitCode:undefined},{},null]) {
  const error=failure();error.cause.details=details;const value=await observed(error);
  assert.equal(value.category,'unqualified');assert.ok(['invalid_fields','unknown_fields','missing_fields'].includes(value.reason));
 }
 const error=failure();Object.defineProperty(error.cause.details,'stdout',{get(){throw new Error('PRIVATE-SENTINEL')}});
 assert.equal((await observed(error)).reason,'opaque');
});

test('accessors cannot change values between validation and projection',async()=>{
 const error=failure();let reads=0;
 Object.defineProperty(error.cause.details,'stdout',{get(){reads++;return reads===1?'':'PRIVATE-SENTINEL'.repeat(5000);}});
 assert.equal((await observed(error)).reason,'opaque');assert.equal(reads,0);
});
