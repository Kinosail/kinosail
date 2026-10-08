import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync, lstatSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { readControl } from './control.mjs';
import { validateActiveOwned, validateOwned, validateReceipt } from './receipt.mjs';
const uuid='12345678-1234-1234-1234-123456789ABC';
test('address failure uses the existing two action events and owned private receipt',async()=>{
 const {createTvActions}=await import('./actions.mjs');const {enterServerAddress}=await import('./tv.mjs');const {readFileSync}=await import('node:fs');
 const f=fixture(),original=Object.freeze(Error('PRIVATE-SENTINEL'));
 try {
  const actions=createTvActions(f.project),client={capture:{snapshot:async()=>{throw original;}}};
  await assert.rejects(actions.step('server_address',()=>enterServerAddress(client,{platform:'ios',target:'tv',udid:uuid},'18769')),error=>error===original);
  const record=JSON.parse(readFileSync(join(f.root,'tv-actions.private.json'),'utf8'));assert.equal(record.events.length,2);
  assert.deepEqual(record.events[1].failure.address,{substage:'capture',candidateCount:null,focusedPropertyPresent:null,inheritedLabel:null});
  assert.ok(!JSON.stringify(record).includes('PRIVATE-SENTINEL'));
 }finally{f.dispose();}
});
function fixture(profile='tvos') {
 const project=mkdtempSync(join(tmpdir(),'kino-tv-admission-')); const root=join(project,'.e2e');mkdirSync(root,{mode:0o700}); const stat=lstatSync(root);
 const platform=profile==='tvos'?'ios':'android',device=platform==='ios'?uuid:'emulator-5554';
 const owned={run:'123-1',rootDev:stat.dev,rootIno:stat.ino,platform,profile,target:'tv',device,emulatorPID:platform==='ios'?null:123,emulatorStart:platform==='ios'?null:'12345',complete:false,...(platform==='ios'?{creationPending:{name:'Kinosail-TV-E2E-123-1',runtime:'com.apple.CoreSimulator.SimRuntime.tvOS-27-0',type:'com.apple.CoreSimulator.SimDeviceType.Apple-TV-4K-3rd-generation-4K',inventoryZero:true,inventorySHA256:'a'.repeat(64)}}:{})};
 const identity={platform,profile,target:'tv',device,port:'18769',revision:'b'.repeat(40),run:'123-1'};
 const appPath=resolve(project,'../..',platform==='ios'?'apps/player/apps/native/.build/tvos-simulator/Build/Products/Debug-appletvsimulator/KinosailPlayer.app':'apps/player/apps/android/app/build/outputs/apk/debug/app-debug.apk');
 const control={identity,appPath};
 function save(){writeFileSync(join(root,'owned-device.json'),JSON.stringify(owned),{mode:0o600});writeFileSync(join(root,'control.json'),JSON.stringify(control),{mode:0o600});}
 save();return{project,root,stat,owned,control,save,dispose:()=>rmSync(project,{recursive:true,force:true})};
}
test('both profiles admit exact private TV controls and owned virtual devices',()=>{for(const profile of ['tvos','androidtv']){const f=fixture(profile);try{assert.deepEqual(readControl(f.project),f.control);assert.equal(validateActiveOwned(f.owned,f.stat).target,'tv');}finally{f.dispose();}}});
test('unknown/missing/conflicting profile or target cannot admit controls',()=>{for(const mutate of [f=>delete f.control.identity.profile,f=>f.control.identity.profile='watchos',f=>f.control.identity.target='mobile',f=>f.control.identity.profile='androidtv',f=>f.control.identity.extra=true,f=>f.owned.profile='androidtv',f=>f.owned.target='mobile']){const f=fixture();try{mutate(f);f.save();assert.throws(()=>readControl(f.project));}finally{f.dispose();}}});
test('foreign phone binary, unowned device and duplicate JSON fields reject',()=>{for(const mode of ['phone','device','duplicate']){const f=fixture();try{if(mode==='phone')f.control.appPath=f.control.appPath.replace('tvos-simulator','ios-simulator').replace('appletvsimulator','iphonesimulator');if(mode==='device')f.owned.device='87654321-1234-1234-1234-123456789ABC';f.save();if(mode==='duplicate')writeFileSync(join(f.root,'control.json'),JSON.stringify(f.control).replace('"appPath":','"appPath":"foreign","appPath":'),{mode:0o600});assert.throws(()=>readControl(f.project));}finally{f.dispose();}}});
test('unresolved ownership cannot admit publication',()=>{const f=fixture();try{assert.throws(()=>validateOwned(f.owned,f.stat));f.owned.complete=true;assert.throws(()=>validateOwned(f.owned,f.stat));f.owned.creationPending=null;assert.equal(validateOwned(f.owned,f.stat).profile,'tvos');}finally{f.dispose();}});
test('TV receipt accepts only exact TV command templates and terminal journey facts',()=>{for(const profile of ['tvos','androidtv']){const f=fixture(profile);try{f.owned.complete=true;f.owned.creationPending=null;const receipt={revision:'b'.repeat(40),command:'python3 hosted.py '+profile,commands:[['go','build','-o',join(f.root,'bin/player'),'./apps/player/cmd/kinosail'],f.owned.platform==='ios'?['./scripts/build-apple.sh','tvos']:['avdmanager','create','avd','--name','Kinosail-TV-E2E-123-1','--package','system-images;android-36;android-tv;x86_64','--device','tv_1080p']],platform:f.owned.platform,profile,target:'tv',device:f.owned.device,environment:'disposable hosted simulator/emulator',data:'synthetic Example Movie (testsrc2/AAC); synthetic MFA Owner; actual dynamic pairing code',witness:{runtime:f.owned.platform==='ios'?'com.apple.CoreSimulator.SimRuntime.tvOS-27-0':'system-images;android-36;android-tv;x86_64'},result:1,cleanup:[],elapsedSeconds:3,packages:{e2e:'0.17.0','agent-device':'0.21.22'},boundaries:['no physical device','no phone/watchOS/Wear pairing','no casting/provider calls','no deployed server'],journey:{approval:'public Owner API and actual dynamic UI code',decodedFrames:true,partialProgressSeconds:2,watched:false,connectionRestored:true,progressPersisted:true,remoteFocus:true,menuReturned:true,tvForeground:true}};assert.equal(validateReceipt(receipt,f.owned,f.root).profile,profile);for(const change of [r=>r.target='mobile',r=>r.profile='wear',r=>r.commands.push(['./scripts/build-apple.sh','ios']),r=>r.journey.decodedFrames=false,r=>r.journey.menuReturned=false,r=>r.packages['@e2e-dev/mobile']='0.10.0']){const bad=structuredClone(receipt);change(bad);assert.throws(()=>validateReceipt(bad,f.owned,f.root));}}finally{f.dispose();}}});

test('closed TV actions persist under current active owner and retain only stage/schema facts',async()=>{
 const {createTvActions,validateTvActions}=await import('./actions.mjs');const f=fixture();
 try {
  const actions=createTvActions(f.project),cause=Object.freeze(Object.assign(Error('SYNTHETIC-PRIVATE'),{code:'ERROR',details:{argv:['secret']}}));
  await assert.rejects(()=>actions.step('inventory',async()=>{throw cause;}),error=>error===cause);
  const {readFileSync}=await import('node:fs');const record=JSON.parse(readFileSync(join(f.root,'tv-actions.private.json')));
  assert.equal(record.events.at(-1).stage,'inventory');assert.equal(record.events.at(-1).status,'failed');
  assert.equal(record.events.at(-1).failure.category,'unqualified');assert.equal(record.events.at(-1).failure.recognizedKeyMask,11);
  assert.equal(JSON.stringify(record).includes('SYNTHETIC-PRIVATE'),false);assert.equal(JSON.stringify(record).includes('secret'),false);
  assert.deepEqual(validateTvActions(record),record);
 }finally{f.dispose();}
});
test('unknown/malformed/oversized actions and foreign owner/file reject without action or foreign writes',async()=>{
 const {createTvActions,validateTvActions}=await import('./actions.mjs');
 for(const value of [null,[],{version:2,events:[]},{version:1,events:Array(97).fill({stage:'inventory',status:'started'})},
  {version:1,events:[{stage:'secret',status:'started'}]},{version:1,events:[{stage:'open',status:'passed',unknown:'secret'}]},
  {version:1,events:[{stage:'use',status:'started'},{stage:'use',status:'failed',failure:{category:'unqualified',ownKeyCount:129,recognizedKeyMask:16}}]},
  {version:1,events:[{stage:'use',status:'started'},{stage:'use',status:'failed',failure:{category:'unqualified',ownKeyCount:0,recognizedKeyMask:1}}]}])assert.throws(()=>validateTvActions(value));
 const f=fixture();try{
  const actions=createTvActions(f.project);let called=false;
  for(const stage of [undefined,'','unknown','x'.repeat(2049)])await assert.rejects(()=>actions.step(stage,async()=>{called=true;}));assert.equal(called,false);
  await assert.rejects(()=>actions.step('open',undefined));assert.equal(called,false);
  const {readFileSync,renameSync}=await import('node:fs');const path=join(f.root,'tv-actions.private.json');renameSync(path,path+'.retained');writeFileSync(path,'foreign',{mode:0o600});
  await assert.rejects(()=>actions.step('inventory',async()=>{called=true;}));assert.equal(called,false);assert.equal(readFileSync(path,'utf8'),'foreign');
 }finally{f.dispose();}
});
test('TV action observation never reads raw error getters or replaces frozen failures',async()=>{
 const {createTvActions}=await import('./actions.mjs');const f=fixture();try{
  const actions=createTvActions(f.project);let reads=0;const cause=Object.freeze(Object.defineProperty({},'details',{get(){reads++;throw Error('secret');}}));
  await assert.rejects(()=>actions.step('open',async()=>{throw cause;}),error=>error===cause);assert.equal(reads,0);
 }finally{f.dispose();}
});

test('closed body-stage diagnostics do not change actions and reject owner transitions before new effects',async()=>{
 const {createTvActions}=await import('./actions.mjs');
 for(const profile of ['tvos','androidtv']){const f=fixture(profile);try{
  const actions=createTvActions(f.project);let calls=0;
  for(const stage of ['owner','library','server_address','connect','approval','movies','movie_focus','play','decoded_frames','pause','menu','progress','relaunch','restored_connection','persisted_progress'])assert.equal(await actions.step(stage,async()=>++calls),calls);
  assert.equal(calls,15);f.control.identity.run='124-1';f.save();
  await assert.rejects(()=>actions.step('play',async()=>++calls));assert.equal(calls,15);
 }finally{f.dispose();}}
});

test('TV action creation requires active owner and preserves an existing foreign output',async()=>{
 const {createTvActions}=await import('./actions.mjs');
 for(const mode of ['complete','foreign-output']){const f=fixture();try{
  const {existsSync,readFileSync}=await import('node:fs');const path=join(f.root,'tv-actions.private.json');
  if(mode==='complete'){f.owned.complete=true;f.save();}else writeFileSync(path,'FOREIGN',{mode:0o600});
  assert.throws(()=>createTvActions(f.project));
  if(mode==='complete')assert.equal(existsSync(path),false);else assert.equal(readFileSync(path,'utf8'),'FOREIGN');
 }finally{f.dispose();}}
});
