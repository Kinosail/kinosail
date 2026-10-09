import test from 'node:test';
import assert from 'node:assert/strict';
import * as tv from './tv.mjs';
import {validateTvActions} from './actions.mjs';
const selection={platform:'ios',target:'tv',udid:'12345678-1234-1234-1234-123456789ABC'};
for(const failed of ['capture','select','type','menu','focus'])test('actual address '+failed+' failure retains original calls and closed substage',async()=>{
 const calls=[],original=Object.freeze(new Error('PRIVATE-SENTINEL'));
 const client={capture:{snapshot:async()=>{calls.push('capture');if(failed==='capture')throw original;
  return {nodes:[{label:'Server address',type:'TextField',focused:failed!=='focus',inheritsLabel:true}]};}},
 command:{tvRemote:async({button})=>{const stage=button==='up'?'focus':button;calls.push(stage);if(stage===failed)throw original;}},
 interactions:{type:async()=>{calls.push('type');if(failed==='type')throw original;}}};
 await assert.rejects(tv.enterServerAddress(client,selection,'18769'),error=>error===original);
 const value=tv.tvAddressFailure?.(original);assert.ok(value);assert.equal(value.substage,failed);
 assert.equal(value.candidateCount,failed==='capture'?null:1);
 assert.equal(value.focusedPropertyPresent,failed==='capture'?null:true);assert.equal(value.inheritedLabel,failed==='capture'?null:true);
 assert.deepEqual(calls,failed==='capture'?['capture']:failed==='focus'?['capture','focus']:['capture','select','type','menu'].slice(0,['select','type','menu'].indexOf(failed)+2));
 assert.ok(!JSON.stringify(value).includes('PRIVATE'));assert.equal(Object.isFrozen(original),true);
});
test('invalid snapshot and exhausted focus loop report no invented focus/value facts',async()=>{
 for(const nodes of [null,[{label:'Server address',type:'TextField',inheritsLabel:true}]]) {
  let captures=0,remotes=0;const client={capture:{snapshot:async()=>{captures++;return {nodes};}},command:{tvRemote:async()=>remotes++}};let error;
  try{await tv.enterServerAddress(client,selection,'18769');}catch(value){error=value;}
  const facts=tv.tvAddressFailure?.(error);assert.ok(facts);assert.equal(facts.substage,nodes?'focus':'snapshot_validation');
  assert.equal(facts.focusedPropertyPresent,nodes?false:null);assert.equal(captures,nodes?32:1);assert.equal(remotes,nodes?32:0);
 }
});
test('address schema rejects unknown/missing/malformed/oversized/conflicting values',()=>{
 const address={substage:'type',candidateCount:1,focusedPropertyPresent:true,inheritedLabel:false};
 const record=value=>({version:1,events:[{stage:'server_address',status:'started'},{stage:'server_address',status:'failed',failure:{category:'unqualified',ownKeyCount:1,recognizedKeyMask:8,address:value}}]});
 assert.deepEqual(validateTvActions(record(address)),record(address));
 for(const value of [null,{}, {...address,extra:'PRIVATE'}, {...address,substage:'unknown'}, {...address,substage:'x'.repeat(2049)},
  {...address,candidateCount:-1},{...address,candidateCount:10001},{...address,candidateCount:1.5},
  {...address,focusedPropertyPresent:'true'},{...address,candidateCount:null},{...address,inheritedLabel:[]}])assert.throws(()=>validateTvActions(record(value)));
 for(const value of [{...address,substage:'capture'}, {...address,substage:'focus',candidateCount:2}, {...address,candidateCount:0}, {...address,focusedPropertyPresent:false}])assert.throws(()=>validateTvActions(record(value)));
 const foreign=record(address);foreign.events.forEach(event=>event.stage='open');assert.throws(()=>validateTvActions(foreign));
});

test('capture failure reports only own pinned SDK data codes and preserves frozen error',async()=>{
 for(const code of ['COMMAND_FAILED','UNSUPPORTED_OPERATION','DEVICE_NOT_FOUND','PRIVATE-SENTINEL',null,42,'x'.repeat(2049)]) {
  const original=Object.freeze(Object.assign(new Error('PRIVATE-SENTINEL'),{code}));let calls=0;
  await assert.rejects(tv.enterServerAddress({capture:{snapshot:async()=>{calls++;throw original;}}},selection,'18769'),error=>error===original);
  const facts=tv.tvAddressFailure(original);assert.equal(facts.sdkCode,['COMMAND_FAILED','UNSUPPORTED_OPERATION','DEVICE_NOT_FOUND'].includes(code)?code:'unqualified');
  assert.equal(calls,1);assert.ok(!JSON.stringify(facts).includes('PRIVATE-SENTINEL'));
 }
 for(const mode of ['accessor','inherited','proxy']) {
  let reads=0;let original=new Error('PRIVATE-SENTINEL');
  if(mode==='accessor')Object.defineProperty(original,'code',{get(){reads++;throw Error('PRIVATE-SENTINEL')}});
  else if(mode==='inherited')Object.setPrototypeOf(original,{code:'COMMAND_FAILED'});
  else original=new Proxy(original,{getOwnPropertyDescriptor(){throw Error('PRIVATE-SENTINEL')}});
  await assert.rejects(tv.enterServerAddress({capture:{snapshot:async()=>{throw original;}}},selection,'18769'),error=>error===original);
  assert.equal(tv.tvAddressFailure(original).sdkCode,'unqualified');assert.equal(reads,0);
 }
});
test('capture SDK code schema stays finite and phase-bound',()=>{
 const capture={substage:'capture',candidateCount:null,focusedPropertyPresent:null,inheritedLabel:null,sdkCode:'COMMAND_FAILED'};
 const record=address=>({version:1,events:[{stage:'server_address',status:'started'},{stage:'server_address',status:'failed',failure:{category:'unqualified',ownKeyCount:1,recognizedKeyMask:1,address}}]});
 assert.deepEqual(validateTvActions(record(capture)),record(capture));
 for(const code of ['PRIVATE-SENTINEL','x'.repeat(2049),null,42,{},'UNKNOWN'])assert.throws(()=>validateTvActions(record({...capture,sdkCode:code})));
 assert.throws(()=>validateTvActions(record({...capture,substage:'snapshot_validation'})));
});
