import test from 'node:test';
import assert from 'node:assert/strict';
import {validateReceipt as phone} from '../e2e-mobile/receipt.mjs';
import {validateReceipt as tv} from '../e2e-tv/receipt.mjs';
const valid={tool:'Xcode',outcome:'valid',exitCode:0,stdoutBytes:32,stderrBytes:0,stdoutTokensRecognized:true,combinedTokensRecognized:true,outputTruncated:false};
function fixture(target,probe){
 const owned={platform:'ios',device:null,run:'123-1',...(target==='tv'?{profile:'tvos',target:'tv'}:{})};
 const receipt={revision:'a'.repeat(40),command:`python3 hosted.py ${target==='tv'?'tvos':'ios'}`,commands:[],platform:'ios',device:null,...(target==='tv'?{profile:'tvos',target:'tv'}:{}),environment:'disposable hosted simulator/emulator',data:'synthetic Example Movie (testsrc2/AAC); synthetic MFA Owner; actual dynamic pairing code',witness:{nativeToolProbe:probe},result:1,cleanup:[],elapsedSeconds:1,packages:target==='tv'?{e2e:'0.17.0','agent-device':'0.21.22'}:{e2e:'0.17.0','@e2e-dev/mobile':'0.10.0','agent-device':'0.21.22'},boundaries:target==='tv'?['no physical device','no phone/watchOS/Wear pairing','no casting/provider calls','no deployed server']:['no physical device','no tvOS/watchOS','no casting/provider calls','no deployed server']};
 return {owned,receipt};
}
for(const [target,validate] of [['phone',phone],['tv',tv]]){
 test(`${target} receipt admits only closed native probe stages`,()=>{
  for(const outcome of ['started','unavailable','timeout','overflow','nonzero','rejected','valid']){
   const probe={...valid,outcome,exitCode:outcome==='valid'?0:outcome==='nonzero'?7:null,combinedTokensRecognized:outcome==='valid',outputTruncated:outcome==='overflow'};
   const {owned,receipt}=fixture(target,probe);assert.deepEqual(validate(receipt,owned,'/owned/.e2e').witness.nativeToolProbe,probe);
  }
 });
 test(`${target} receipt rejects ambiguous or unbounded probe without mutation`,()=>{
  for(const mutate of [p=>delete p.outcome,p=>p.outcome='PRIVATE-SENTINEL',p=>p.extra='PRIVATE-SENTINEL',p=>p.tool='Java',p=>p.stdoutBytes=8194,p=>p.stderrBytes=-1,p=>p.exitCode=999,p=>p.stdoutTokensRecognized='true',p=>p.outputTruncated=1,p=>p.combinedTokensRecognized=false,p=>p.exitCode=7,p=>p.outputTruncated=true]){
   const probe={...valid};mutate(probe);const {owned,receipt}=fixture(target,probe),before=JSON.stringify(receipt);
   assert.throws(()=>validate(receipt,owned,'/owned/.e2e'));assert.equal(JSON.stringify(receipt),before);
  }
 });
}
